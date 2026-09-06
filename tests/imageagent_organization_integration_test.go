package tests

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/api/serviceerror"
	sdkclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	apphttpapi "task-processor/internal/app/httpapi"
	imageworker "task-processor/internal/app/worker/imageagent"
	"task-processor/internal/authidentity"
	zitadel "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/objectstore"
	imagestore "task-processor/internal/imageagent/store"
	imagetemporal "task-processor/internal/imageagent/temporal"
	imagetools "task-processor/internal/imageagent/tools"
	openai "task-processor/internal/integration/openai"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/product/catalog"
	"task-processor/internal/workbenchcontext"
)

// Only identity issuance, the remote IAM response, Temporal's server boundary
// and object/provider I/O are controlled external substitutes. HTTP admission,
// grant parsing/resolution/authorization, Catalog/run/effect persistence,
// Temporal Client/converter/Activities, Manager and Review adapter are real.
type scope339TemporalServer struct {
	sdkclient.Client
	mu         sync.Mutex
	payload    *commonpb.Payloads
	options    sdkclient.StartWorkflowOptions
	starts     int
	failNext   bool
	executions map[string]bool
}

func (s *scope339TemporalServer) ExecuteWorkflow(_ context.Context, options sdkclient.StartWorkflowOptions, _ interface{}, args ...interface{}) (sdkclient.WorkflowRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.starts++
	if s.executions == nil {
		s.executions = map[string]bool{}
	}
	s.executions[options.ID] = true
	payload, err := converter.GetDefaultDataConverter().ToPayloads(args...)
	if err != nil {
		return nil, err
	}
	s.payload = payload
	s.options = options
	if s.failNext {
		s.failNext = false
		return nil, fmt.Errorf("controlled accepted start response lost")
	}
	return nil, nil
}
func (s *scope339TemporalServer) QueryWorkflow(context.Context, string, string, string, ...interface{}) (converter.EncodedValue, error) {
	return nil, serviceerror.NewNotFound("controlled server has no running workflow")
}
func (s *scope339TemporalServer) input(t *testing.T) imagetemporal.WorkflowInput {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	var input imagetemporal.WorkflowInput
	require.NoError(t, converter.GetDefaultDataConverter().FromPayloads(s.payload, &input))
	return input
}

type scope339IAM struct {
	revoked atomic.Bool
	failed  atomic.Bool
	calls   atomic.Int32
}

func (s *scope339IAM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.calls.Add(1)
	if s.failed.Load() {
		http.Error(w, "controlled authorization unavailable", 503)
		return
	}
	var req struct {
		Filters []map[string]json.RawMessage `json:"filters"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || len(req.Filters) < 2 {
		http.Error(w, "invalid query", 400)
		return
	}
	var users struct {
		IDs []string `json:"ids"`
	}
	_ = json.Unmarshal(req.Filters[0]["inUserIds"], &users)
	if len(users.IDs) != 1 {
		http.Error(w, "missing actor", 400)
		return
	}
	orgs := []string{"B", "C"}
	if len(req.Filters) == 3 {
		if r.Header.Get("Authorization") != "Bearer service339" {
			http.Error(w, "wrong service credential", 401)
			return
		}
		var org struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(req.Filters[2]["organizationId"], &org)
		orgs = []string{org.ID}
	}
	entries := []any{}
	if !s.revoked.Load() {
		for _, org := range orgs {
			role := "listingkit_operator"
			if users.IDs[0] == "admin" {
				role = "listingkit_admin"
			}
			entries = append(entries, map[string]any{"id": "grant-" + org, "project": map[string]string{"id": "project"}, "organization": map[string]string{"id": org}, "user": map[string]string{"id": users.IDs[0]}, "state": "STATE_ACTIVE", "roles": []any{map[string]string{"key": role}}})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"pagination": map[string]string{"totalResult": fmt.Sprint(len(entries)), "appliedLimit": "100"}, "authorizations": entries})
}

type scope339Verifier struct{}

func (scope339Verifier) Verify(_ context.Context, token string) (authidentity.AuthenticatedIdentity, error) {
	if token != "actor" && token != "admin" {
		return authidentity.AuthenticatedIdentity{}, fmt.Errorf("controlled invalid identity")
	}
	return authidentity.AuthenticatedIdentity{HomeOrganizationID: "A", TenantID: "A", UserID: token, TokenExpiresAt: time.Now().Add(time.Hour)}, nil
}

type scope339Fixture struct {
	db            *gorm.DB
	iam           *scope339IAM
	authClient    *zitadel.AuthorizationClient
	auth          *authz.ListingKitAuthorizer
	temporal      *scope339TemporalServer
	bindings      []apphttpapi.OrganizationImageBinding
	providerCalls atomic.Int32
}

func newScope339Fixture(t *testing.T) *scope339Fixture {
	t.Helper()
	dsn := os.Getenv("ISSUE339_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL ISSUE339_TEST_DSN")
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	root, err := gorm.Open(postgres.Open(dsn), cfg)
	require.NoError(t, err)
	schema := "scope339_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE SCHEMA "+schema).Error)
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		raw, _ := db.DB()
		_ = raw.Close()
		_ = root.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		raw, _ = root.DB()
		_ = raw.Close()
	})
	require.NoError(t, imagestore.AutoMigrateOrganizationScope(db))
	require.NoError(t, catalogstore.AutoMigrate(db))
	require.NoError(t, db.AutoMigrate(&openai.AIClientCredential{}))
	f := &scope339Fixture{db: db, iam: &scope339IAM{}, temporal: &scope339TemporalServer{}}
	upstream := httptest.NewServer(f.iam)
	t.Cleanup(upstream.Close)
	f.authClient = zitadel.NewAuthorizationClient(upstream.URL, upstream.Client())
	f.auth, err = authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	repo, err := catalogstore.NewRepository(db)
	require.NoError(t, err)
	for _, org := range []string{"B", "C"} {
		published, err := repo.PublishSnapshot(context.Background(), catalog.PublishRequest{Identity: catalog.SnapshotIdentity{TenantID: org, ProductKey: "product"}, PublicationID: "scope339-publication", Snapshot: catalog.ProductSnapshot{Title: "Controlled shoes", Images: []catalog.Image{{URL: "https://source.example/" + org + ".png", Role: "source", Width: 1200, Height: 1200}}}})
		require.NoError(t, err)
		f.bindings = append(f.bindings, apphttpapi.OrganizationImageBinding{ContextID: "catalog-input", OwnerUserID: "actor", Identity: published.Identity, Version: published.Version, PublicationID: published.PublicationID})
	}
	return f
}
func (f *scope339Fixture) server(t *testing.T) *httptest.Server {
	t.Helper()
	resolver := workbenchcontext.NewResolver(workbenchcontext.NewGrantResolver(f.authClient, workbenchcontext.NewGrantCache(time.Now)), "project", "v1", nil)
	app, err := apphttpapi.NewImageAgentOrganizationApplication(f.db, scope339Verifier{}, resolver, f.auth, imagetemporal.NewOrganizationClient(f.temporal), imageagent.TenantAllowlistStartGate{Enabled: true, AllowedTenantIDs: []string{"B", "C"}}, f.bindings)
	require.NoError(t, err)
	server := httptest.NewServer(app.Handler)
	t.Cleanup(server.Close)
	return server
}

const scope339Body = `{"run_id":"run339","business_task_id":"catalog-input","target_platform":"shein","image_policy_context":{"country":"us","family":"default","scene_category":"shoes"},"mode":"manual","idempotency_key":"run-key339","max_concurrent_slots":1,"budget":{"max_images":12,"max_agent_steps":20,"max_model_calls":30,"max_repair_attempts_per_slot":2,"max_elapsed":500000000000},"plan":{"revision":1,"idempotency_key":"plan-key339","source_asset_ids":["catalog-image-1"],"slots":[{"id":"slot1","role":"main","status":"pending","source_asset_ids":["catalog-image-1"],"idempotency_key":"slot-key339"}]}}`
const scope339Path = "/api/organization/image-agent/runs"

func scope339Request(t *testing.T, server *httptest.Server, method, path, actor, org, body string) int {
	t.Helper()
	req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+actor)
	req.Header.Set("X-Requested-Organization-ID", org)
	req.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(req)
	require.NoError(t, err)
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	t.Logf("%s %s actor=%s selected=%s status=%d body=%s", method, path, actor, org, response.StatusCode, data)
	return response.StatusCode
}
func TestOrganizationScopeHTTPPersistenceAndActivity(t *testing.T) {
	f := newScope339Fixture(t)
	server := f.server(t)
	require.Equal(t, 202, scope339Request(t, server, "POST", scope339Path, "actor", "B", scope339Body))
	input := f.temporal.input(t)
	require.Equal(t, "B", input.Identity.TenantID)
	require.Equal(t, "actor", input.Identity.UserID)
	require.Equal(t, "run339", input.Identity.RunID)
	require.Equal(t, imageagent.OrganizationScopeProtocol, input.Identity.ScopeProtocol)
	require.Equal(t, imagetemporal.OrganizationTaskQueue, f.temporal.options.TaskQueue)
	require.True(t, strings.HasPrefix(f.temporal.options.ID, "organization-v1:"))
	repo := imagestore.NewOrganizationRepository(f.db)
	scope := imageagent.RunScope{TenantID: "B", OwnerUserID: "actor", RunID: "run339"}
	stored, err := repo.GetProjection(context.Background(), scope)
	require.NoError(t, err)
	require.Equal(t, input.AssetCatalog, stored.AssetCatalog)
	require.Contains(t, stored.AssetCatalog.Assets[0].URL, "/B.png")
	// Rebuild HTTP resolver/service/repository, retry the actual command, then
	// decode Temporal's serialized payload with a fresh converter call.
	require.Equal(t, 202, scope339Request(t, f.server(t), "POST", scope339Path, "actor", "B", scope339Body))
	restored := f.temporal.input(t)
	require.Equal(t, input.Identity, restored.Identity)
	require.Equal(t, 200, scope339Request(t, f.server(t), "GET", scope339Path+"/run339", "actor", "B", ""))
	starts := f.temporal.starts
	for _, tc := range []struct{ name, actor, org, body string }{
		{"missing organization", "actor", "", scope339Body},
		{"cross organization identity", "actor", "C", scope339Body},
		{"cross organization key", "actor", "C", strings.Replace(scope339Body, "run339", "other339", 1)},
		{"changed payload", "actor", "B", strings.Replace(scope339Body, "slot-key339", "different", 1)},
		{"body owner tamper", "actor", "B", strings.Replace(scope339Body, `"mode":"manual"`, `"mode":"manual","tenant_id":"C"`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.GreaterOrEqual(t, scope339Request(t, f.server(t), "POST", scope339Path, tc.actor, tc.org, tc.body), 400)
			require.Equal(t, starts, f.temporal.starts)
		})
	}
	for _, actor := range []string{"actor", "admin"} {
		org := "C"
		if actor == "admin" {
			org = "B"
		}
		require.Equal(t, 404, scope339Request(t, f.server(t), "GET", scope339Path+"/run339", actor, org, ""))
		require.Equal(t, 404, scope339Request(t, f.server(t), "POST", scope339Path+"/run339/restart", actor, org, ""))
	}
	f.iam.revoked.Store(true)
	require.GreaterOrEqual(t, scope339Request(t, f.server(t), "POST", scope339Path, "actor", "B", scope339Body), 400)
	f.iam.revoked.Store(false)
	f.iam.failed.Store(true)
	require.GreaterOrEqual(t, scope339Request(t, f.server(t), "POST", scope339Path, "actor", "B", scope339Body), 400)
	f.iam.failed.Store(false)
	require.Equal(t, starts, f.temporal.starts)
	require.Equal(t, 404, scope339Request(t, server, "POST", "/api/v1/image-agent/task-runs", "actor", "B", `{}`))
	legacy := imagestore.NewGormRepository(f.db)
	_, err = legacy.GetProjection(context.Background(), scope)
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	_, err = legacy.GetAssetCatalog(context.Background(), scope)
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	_, err = legacy.ListEvents(context.Background(), scope, 0, 10)
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	_, err = legacy.CommitProjection(context.Background(), imageagent.ProjectionCommit{Scope: scope, Snapshot: stored})
	require.Error(t, err)
	oldRun := stored.Run
	oldRun.ID = "old339"
	oldRun.ScopeProtocol = ""
	oldRun.IdempotencyKey = "old-key339"
	oldScope := imageagent.ScopeForRun(oldRun)
	_, err = legacy.InitializeRun(context.Background(), imageagent.ProjectionInitialization{Scope: oldScope, Run: oldRun, Plan: stored.Plan, Catalog: stored.AssetCatalog, Snapshot: imageagent.RunProjection{Run: oldRun, Plan: stored.Plan}, CommitID: "old-initialize", EventType: "run.initialized", EventPayload: json.RawMessage(`{}`)})
	require.NoError(t, err)
	_, err = repo.GetProjection(context.Background(), oldScope)
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	_, err = repo.GetAssetCatalog(context.Background(), oldScope)
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	_, err = repo.ListEvents(context.Background(), oldScope, 0, 10)
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	oldBody := strings.ReplaceAll(scope339Body, "run339", "old339")
	require.GreaterOrEqual(t, scope339Request(t, f.server(t), "POST", scope339Path, "actor", "B", oldBody), 400)
	oldProjection, err := legacy.GetProjection(context.Background(), oldScope)
	require.NoError(t, err)
	require.Empty(t, oldProjection.Run.ScopeProtocol)
	for _, tc := range []struct {
		name       string
		repository imageagent.Repository
		projection imageagent.RunProjection
	}{
		{"organization cannot mutate historical", repo, oldProjection},
		{"historical cannot mutate organization", legacy, stored},
	} {
		t.Run(tc.name, func(t *testing.T) {
			direct := tc.repository.(interface {
				UpdateRun(context.Context, imageagent.RunScope, int64, imageagent.RunMutation) error
				AppendPlan(context.Context, imageagent.RunScope, int64, imageagent.Plan) error
			})
			scope := imageagent.ScopeForRun(tc.projection.Run)
			err := direct.UpdateRun(context.Background(), scope, tc.projection.Run.Version, imageagent.RunMutation{Status: imageagent.RunStatusFailed, CurrentNode: "must-not-write", ActivePlanRevision: 1})
			require.ErrorIs(t, err, imageagent.ErrRunNotFound)
			// Also reject a replay of an existing plan before consulting its receipt.
			require.ErrorIs(t, direct.AppendPlan(context.Background(), scope, 1, tc.projection.Plan), imageagent.ErrRunNotFound)
			plan := tc.projection.Plan
			plan.Revision = 2
			plan.ParentRevision = 1
			plan.IdempotencyKey = "must-not-append"
			require.ErrorIs(t, direct.AppendPlan(context.Background(), scope, 1, plan), imageagent.ErrRunNotFound)
		})
	}
	current, err := repo.GetProjection(context.Background(), scope)
	require.NoError(t, err)
	require.Equal(t, stored.Run, current.Run)
	f.verifyActivity(t, restored)
	f.verifyRecoveryCommand(t)
}

func TestOrganizationScopeInitializationFailureAndStartResponseLoss(t *testing.T) {
	f := newScope339Fixture(t)
	require.NoError(t, f.db.Callback().Create().Before("gorm:create").Register("scope339:fail-event", func(tx *gorm.DB) {
		if tx.Statement.Table == "image_agent_v2_events" {
			tx.AddError(fmt.Errorf("controlled event write failure"))
		}
	}))
	require.GreaterOrEqual(t, scope339Request(t, f.server(t), "POST", scope339Path, "actor", "B", scope339Body), 400)
	for _, table := range []string{"image_agent_v2_runs", "image_agent_v2_plans", "image_agent_v2_asset_catalog_manifests", "image_agent_v2_projection_snapshots", "image_agent_v2_events"} {
		var count int64
		require.NoError(t, f.db.Table(table).Count(&count).Error)
		require.Zero(t, count)
	}
	require.Zero(t, f.temporal.starts)
	require.NoError(t, f.db.Callback().Create().Remove("scope339:fail-event"))
	f.temporal.failNext = true
	require.GreaterOrEqual(t, scope339Request(t, f.server(t), "POST", scope339Path, "actor", "B", scope339Body), 400)
	first := f.temporal.input(t)
	require.Equal(t, 202, scope339Request(t, f.server(t), "POST", scope339Path, "actor", "B", scope339Body))
	require.Equal(t, first.Identity, f.temporal.input(t).Identity)
	require.Equal(t, 2, f.temporal.starts)
	require.Len(t, f.temporal.executions, 1)
	var count int64
	require.NoError(t, f.db.Table("image_agent_v2_runs").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func (f *scope339Fixture) verifyRecoveryCommand(t *testing.T) {
	repo := imagestore.NewOrganizationRepository(f.db)
	scope := imageagent.RunScope{TenantID: "B", OwnerUserID: "actor", RunID: "run339"}
	current, err := repo.GetProjection(context.Background(), scope)
	require.NoError(t, err)
	next := current
	next.Run.Status = imageagent.RunStatusBlocked
	next.Run.CurrentNode = "blocked"
	next.Run.Version++
	next.Run.Block = &imageagent.Block{Code: imageagent.SlotReviewRequiredCode, SlotID: "slot1"}
	next.Slots = append([]imageagent.SlotProjection(nil), current.Slots...)
	next.Slots[0].Slot.Status = imageagent.SlotStatusBlocked
	next.Slots[0].Attempt = 1
	next.Slots[0].ErrorCode = imageagent.SlotReviewRequiredCode
	_, err = repo.CommitProjection(context.Background(), imageagent.ProjectionCommit{Scope: scope, CommitID: "scope339-blocked", ExpectedProjectionVersion: current.ProjectionVersion, ExpectedRunVersion: current.Run.Version, Snapshot: next, EventType: "run.blocked", EventPayload: json.RawMessage(`{}`), RunMutation: &imageagent.RunMutation{Status: next.Run.Status, CurrentNode: next.Run.CurrentNode, ActivePlanRevision: next.Run.ActivePlanRevision, Block: next.Run.Block}, SlotMutation: &imageagent.SlotProjectionMutation{PlanRevision: 1, Projection: next.Slots[0], Result: imageagent.SlotResult{SlotID: "slot1", Attempt: 1, Status: imageagent.SlotStatusBlocked, ErrorCode: imageagent.SlotReviewRequiredCode}, Attempt: imageagent.StepAttempt{TenantID: "B", OwnerUserID: "actor", RunID: "run339", PlanRevision: 1, SlotID: "slot1", Attempt: 1, Node: "execute_slot", IdempotencyKey: "scope339-blocked-attempt", Outcome: "blocked", ErrorCategory: imageagent.SlotReviewRequiredCode}}})
	require.NoError(t, err)
	require.Equal(t, 202, scope339Request(t, f.server(t), "POST", scope339Path+"/run339/slots/slot1/attempts/1/recover", "actor", "B", `{"plan_revision":1,"action_id":"recover339"}`))
	var recovery imagetemporal.EffectRecoveryWorkflowInput
	require.NoError(t, converter.GetDefaultDataConverter().FromPayloads(f.temporal.payload, &recovery))
	require.Equal(t, "run339", recovery.Identity.RunID)
	require.Equal(t, "B", recovery.Identity.TenantID)
	require.Equal(t, "actor", recovery.Identity.UserID)
	require.Equal(t, "catalog-input", recovery.Identity.BusinessTaskID)
	require.Equal(t, imagetemporal.OrganizationTaskQueue, f.temporal.options.TaskQueue)
	require.True(t, strings.HasPrefix(f.temporal.options.ID, "organization-v1:"))
}

// The object store substitute can read an existing synthetic staging manifest;
// all generation/publication methods panic if accidentally invoked.
type scope339Artifacts struct {
	imagetemporal.DurableArtifactStore
	reads atomic.Int32
}

func (s *scope339Artifacts) PublicURL(key string) string { return "https://assets.example/" + key }
func (s *scope339Artifacts) RecoverSlotArtifacts(_ context.Context, _ imageagent.SlotExternalEffectIdentity, _ imageagent.StagingManifest) (objectstore.PreparedSlotArtifacts, error) {
	s.reads.Add(1)
	return objectstore.PreparedSlotArtifacts{}, nil
}
func (s *scope339Artifacts) EnsureStaged(context.Context, objectstore.PreparedSlotArtifacts) error {
	return nil
}

type scope339NoPublication struct {
	imageagent.ApprovedAssetPublisher
	imageagent.ApprovedAssetPublisherV3
}

func (f *scope339Fixture) verifyActivity(t *testing.T, wf imagetemporal.WorkflowInput) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.providerCalls.Add(1)
		require.Equal(t, "Bearer synthetic-B", r.Header.Get("Authorization"))
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		require.Contains(t, string(body), "/B.png")
		require.NotContains(t, string(body), "/A.png")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"scope339-review","model":"review-test","choices":[{"index":0,"message":{"role":"assistant","content":"{\"score\":0.2,\"needs_human_review\":true,\"reasons\":[\"controlled review\"]}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`)
	}))
	defer provider.Close()
	credentials := openai.NewGormCredentialResolver(f.db)
	for _, org := range []string{"A"} {
		for _, name := range []string{"default", "image_gpt_image_2"} {
			require.NoError(t, credentials.SaveCredential(context.Background(), openai.AIClientCredential{TenantID: org, UserID: "actor", ClientName: name, APIKey: "synthetic-" + org, BaseURL: provider.URL + "/v1", Model: "review-test", Enabled: true, TimeoutSecond: 1}))
		}
	}
	cfg := openai.NewClientConfig("static-must-not-be-used", "review-test", provider.URL+"/v1", 1)
	manager, err := openai.NewManager(&openai.ManagerConfig{Clients: map[string]*openai.ClientConfig{"default": cfg, "image_gpt_image_2": cfg}, DefaultClient: "default"})
	require.NoError(t, err)
	capabilities, err := imageworker.BuildOrganizationImageCapabilities(manager, f.db)
	require.NoError(t, err)
	executor := imagetools.NewProductImageSlotExecutor(imagetools.Dependencies{SubjectExtractor: capabilities.SubjectExtractor, WhiteBackgroundRenderer: capabilities.WhiteBackgroundRenderer, SceneRenderer: capabilities.SceneRenderer, Reviewer: capabilities.Reviewer, UsageQuoter: capabilities.UsageQuoter, ProfileResolver: capabilities.ProfileResolver})
	repo := imagestore.NewOrganizationRepository(f.db)
	effects := repo.(imageagent.SlotExternalEffectV3Repository)
	artifacts := &scope339Artifacts{}
	authorizer := imageworker.OrganizationExecutionAuthorizer{Client: f.authClient, ServiceToken: func(context.Context) (string, error) { return "service339", nil }, ProjectID: "project", Authorizer: f.auth}
	activities, err := imagetemporal.NewActivities(imagetemporal.ActivityDependencies{Repository: repo, SlotExecutor: executor, Publisher: scope339NoPublication{}, PublisherV3: scope339NoPublication{}, StagedSlotExecutor: executor, ArtifactStore: artifacts, ExecutionAuthorizer: authorizer})
	require.NoError(t, err)
	input := imagetemporal.ExecuteSlotV3ActivityInput{RunID: wf.RunID, Identity: wf.Identity, PlanRevision: wf.Plan.Revision, Slot: wf.Plan.Slots[0], Attempt: 1, IdempotencyKey: "review339", BudgetAuthorization: true, BudgetPolicy: wf.BudgetPolicy, ReviewActionID: "review-action339", TargetPlatform: wf.TargetPlatform, ImagePolicyContext: wf.ImagePolicyContext, AssetCatalog: wf.AssetCatalog}
	// Capture and decode the actual Activity envelope too; no authidentity or
	// aiidentity context is inserted by this test at the consumer end.
	data, err := converter.GetDefaultDataConverter().ToPayloads(input)
	require.NoError(t, err)
	input = imagetemporal.ExecuteSlotV3ActivityInput{}
	require.NoError(t, converter.GetDefaultDataConverter().FromPayloads(data, &input))
	execution := imageagent.SlotExecutionInput{RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID, PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt, IdempotencyKey: input.IdempotencyKey, TargetPlatform: input.TargetPlatform, ImagePolicyContext: input.ImagePolicyContext, AssetCatalog: input.AssetCatalog, ProductContext: input.AssetCatalog.ProductContext}
	reservation := imageagent.SlotEffectV3Reservation{Identity: imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}, PlanRevision: input.PlanRevision, SlotID: input.Slot.ID, Attempt: input.Attempt}, IdempotencyKey: input.IdempotencyKey, InputFingerprint: imageagent.SlotExecutionFingerprint(execution)}
	// This is a controlled existing generation receipt, not a substitute for
	// Review quoting. The actual Activity obtains and validates its own quote.
	reservation.Policy = wf.BudgetPolicy
	maximum := imageagent.UsageVector{Images: 1, AgentSteps: 1}
	reservation.Quote = imageagent.SlotUsageQuote{Fingerprint: "existing-staging-quote339", Maximum: maximum, Operations: []imageagent.SlotUsageOperation{{Name: "render_scene", Fingerprint: "existing-staging-operation339", MaximumOutputs: 1, Maximum: maximum}}}
	_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
	require.NoError(t, err)
	require.True(t, claimed)
	sum := sha256.Sum256([]byte("existing synthetic staging evidence"))
	hash := hex.EncodeToString(sum[:])
	owner, err := imageagent.ArtifactOwnerKey(input.Identity.UserID)
	require.NoError(t, err)
	manifest := imageagent.StagingManifest{Assets: []imageagent.StagedAssetRef{{ObjectKey: fmt.Sprintf("image-agent/staging/%s/%s/%s/%d/%s/1/0-%s.png", input.Identity.TenantID, owner, input.RunID, input.PlanRevision, input.Slot.ID, hash), SHA256: hash, SizeBytes: 35, ContentType: "image/png", Width: 1200, Height: 1200, SourceAssetID: input.Slot.SourceAssetIDs[0], Operations: []string{"render_scene_model"}}}}
	_, err = effects.PrepareSlotStagingV3(context.Background(), reservation, manifest)
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		mutate func(*imagetemporal.ExecuteSlotV3ActivityInput)
	}{
		{"missing protocol", func(i *imagetemporal.ExecuteSlotV3ActivityInput) { i.Identity.ScopeProtocol = "" }},
		{"unknown protocol", func(i *imagetemporal.ExecuteSlotV3ActivityInput) { i.Identity.ScopeProtocol = "other" }},
		{"organization tamper", func(i *imagetemporal.ExecuteSlotV3ActivityInput) { i.Identity.TenantID = "A" }},
		{"actor tamper", func(i *imagetemporal.ExecuteSlotV3ActivityInput) { i.Identity.UserID = "admin" }},
		{"run tamper", func(i *imagetemporal.ExecuteSlotV3ActivityInput) { i.RunID = "other" }},
		{"business context tamper", func(i *imagetemporal.ExecuteSlotV3ActivityInput) { i.Identity.BusinessTaskID = "other" }},
		{"catalog tamper", func(i *imagetemporal.ExecuteSlotV3ActivityInput) { i.AssetCatalog.ProductContext.Title = "other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := input
			tc.mutate(&bad)
			_, err := activities.ReviewStagedSlotV3(context.Background(), bad)
			require.Error(t, err)
			require.Zero(t, f.providerCalls.Load())
			require.Zero(t, artifacts.reads.Load())
		})
	}
	f.iam.revoked.Store(true)
	_, err = activities.ReviewStagedSlotV3(context.Background(), input)
	require.Error(t, err)
	f.iam.revoked.Store(false)
	f.iam.failed.Store(true)
	_, err = activities.ReviewStagedSlotV3(context.Background(), input)
	require.Error(t, err)
	f.iam.failed.Store(false)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = activities.ReviewStagedSlotV3(canceled, input)
	require.Error(t, err)
	expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	_, err = activities.ReviewStagedSlotV3(expired, input)
	require.Error(t, err)
	require.Zero(t, f.providerCalls.Load())
	require.Zero(t, artifacts.reads.Load())
	// B is absent: A and static credentials must not substitute for its route.
	_, err = activities.ReviewStagedSlotV3(context.Background(), input)
	require.Error(t, err)
	require.Zero(t, f.providerCalls.Load())
	for _, enabled := range []bool{false, true} {
		for _, name := range []string{"default", "image_gpt_image_2"} {
			require.NoError(t, credentials.SaveCredential(context.Background(), openai.AIClientCredential{TenantID: "B", UserID: "actor", ClientName: name, APIKey: "synthetic-B", BaseURL: provider.URL + "/v1", Model: "review-test", Enabled: enabled, TimeoutSecond: 1}))
		}
		if !enabled {
			require.NoError(t, f.db.Model(&openai.AIClientCredential{}).Where("tenant_id = ? AND user_id = ?", "B", "actor").Update("enabled", false).Error)
			_, err = activities.ReviewStagedSlotV3(context.Background(), input)
			require.Error(t, err)
			require.Zero(t, f.providerCalls.Load())
		}
	}
	readsBefore := artifacts.reads.Load()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(activities.ReviewStagedSlotV3)
	_, err = env.ExecuteActivity(activities.ReviewStagedSlotV3, input)
	require.Error(t, err)
	require.Contains(t, err.Error(), imageagent.SlotReviewRequiredCode)
	require.EqualValues(t, 1, f.providerCalls.Load())
	require.EqualValues(t, readsBefore+1, artifacts.reads.Load())
	t.Log("actual HTTP Home A / selected B -> persisted scope -> converter -> SDK Activity -> service authorization -> B credentials -> Manager / adapter: provider HTTP=1; no generation or approval")
}
