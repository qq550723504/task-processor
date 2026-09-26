package imageagentworker

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"task-processor/internal/aicapability"
	aistore "task-processor/internal/aicapability/store"
	"task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	imagestore "task-processor/internal/imageagent/store"
	imagetemporal "task-processor/internal/imageagent/temporal"
	openai "task-processor/internal/integration/openai"
	"task-processor/internal/listingsubscription"
	platformtemporal "task-processor/internal/platform/temporal"
)

func TestOrganizationWorkerRealTemporalUnpricedGenerationDoesNotReserveOrDispatch(t *testing.T) {
	address := os.Getenv("ISSUE487_TEMPORAL_ADDRESS")
	if address == "" {
		t.Skip("requires isolated Temporal ISSUE487_TEMPORAL_ADDRESS")
	}
	db := admissionPostgres(t, "org-1", "member-1", 1000)
	require.NoError(t, imagestore.AutoMigrateOrganizationScope(db))
	require.NoError(t, db.AutoMigrate(&openai.AIClientCredential{}))
	require.NoError(t, aistore.AutoMigrateInvocationLedger(db))
	var providerCalls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { providerCalls.Add(1) }))
	defer provider.Close()
	credentials := openai.NewGormCredentialResolver(db)
	for _, name := range []string{"default", "image_gpt_image_2"} {
		require.NoError(t, credentials.SaveCredential(context.Background(), openai.AIClientCredential{TenantID: "org-1", UserID: "actor-1", ClientName: name, APIKey: "controlled-local-only", BaseURL: provider.URL + "/v1", Model: "review-test", Enabled: true, TimeoutSecond: 1}))
	}
	manager, err := openai.NewManager(&openai.ManagerConfig{Clients: map[string]*openai.ClientConfig{
		"default":           openai.NewClientConfig("unused", "review-test", provider.URL+"/v1", 1),
		"image_gpt_image_2": openai.NewClientConfig("unused", "image-test", provider.URL+"/v1", 1),
	}, DefaultClient: "default"})
	require.NoError(t, err)
	recorder := aistore.NewGormInvocationRecorder(db)
	recorder.SetUsageSettler(listingsubscription.AIInvocationUsageAdapter{Repository: listingsubscription.NewGormRepository(db)})
	tracked := &trackedCommercialReservation{GormInvocationRecorder: recorder}
	cfg := &config.Config{Database: &config.DatabaseConfig{}, CommercialDatabase: &config.DatabaseConfig{Host: "isolated-fixture", Port: 5432, Database: "commercial", User: "commercial_runtime"}, CommercialOwnerDatabase: &config.DatabaseConfig{Host: "isolated-fixture", Port: 5432, Database: "commercial", User: "commercial_owner_runtime"}}
	cfg.ImageAgent.ArtifactStore = durableArtifactStoreConfig("aws", true)
	resolver := imageAgentWorkerDependencyResolver{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		// This unpriced-generation fixture injects pools; runtime-role SQL is
		// covered separately and is not claimed by this Temporal test.
		VerifyResourceRuntime: func(context.Context, *gorm.DB) error { return nil },
		OpenDB:                func(*config.DatabaseConfig) (*gorm.DB, error) { return db, nil },
		CloseDB:               func(*config.DatabaseConfig, *gorm.DB) error { return nil },
		BuildAI: func(*config.Config, *gorm.DB, *gorm.DB, *logrus.Logger) (*openai.Manager, openai.ClientConfigResolver, aicapability.InvocationRecorder, error) {
			return manager, nil, tracked, nil
		},
		BuildOrganizationCapabilities: buildOrganizationWorkerCapabilities,
		BuildOrganizationAuthorizer: func(*config.Config) (imageagent.ExecutionAuthorizer, error) {
			return admissionExactAuthorizer{}, nil
		},
		BuildArtifactStore: func(*config.Config, imageAgentArtifactTiming, *logrus.Logger) (imagetemporal.DurableArtifactStore, error) {
			return stubWorkerArtifactStore{}, nil
		},
	}
	dependencies, closeDependencies, err := resolveImageAgentTemporalDependenciesForMode("controlled", logrus.New(), imagetemporal.WorkerWireModeOrganization, resolver)
	require.NoError(t, err)
	defer closeDependencies()
	require.IsType(t, organizationMainSlotExecutor{}, dependencies.StagedSlotExecutor)
	repository := dependencies.Repository
	runID := "run487-" + uuid.NewString()[:8]
	policyContext := imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}
	budget := imageagent.Budget{MaxImages: 1, EnabledLimits: imageagent.BudgetLimitImages}
	run := imageagent.Run{ID: runID, ScopeProtocol: imageagent.OrganizationScopeProtocol, BusinessTaskID: "catalog-receipt-1", TenantID: "org-1", UserID: "actor-1", MemberID: "member-1", Mode: imageagent.RunModeManual, TargetPlatform: "product", ImagePolicyContext: policyContext, IdempotencyKey: "start-" + runID, Status: imageagent.RunStatusExecuting, ActivePlanRevision: 1, Version: 1, Budget: budget, MaxConcurrentSlots: 1, StartedAt: time.Now().UTC()}
	plan := imageagent.Plan{Revision: 1, IdempotencyKey: "plan-" + runID, SourceAssetIDs: []string{"catalog-image-1"}, CreatedBy: run.UserID, Slots: []imageagent.Slot{{ID: "main-1", Role: imageagent.SlotRoleMain, Status: imageagent.SlotStatusPending, SourceAssetIDs: []string{"catalog-image-1"}, IdempotencyKey: "slot-" + runID}}}
	catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: "product-1", Title: "Controlled product", SourceSnapshotVersion: 1}, Assets: []imageagent.AuthorizedAsset{{ID: "catalog-image-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/product.png", Width: 1200, Height: 1200}}})
	require.NoError(t, err)
	scope := imageagent.ScopeForRun(run)
	_, err = repository.InitializeRun(context.Background(), imageagent.ProjectionInitialization{Scope: scope, Run: run, Plan: plan, Catalog: catalog, Snapshot: imageagent.RunProjection{Run: run, Plan: plan}, CommitID: "start:" + run.IdempotencyKey, EventType: "run.initialized", EventPayload: []byte(`{}`)})
	require.NoError(t, err)
	activities, err := imagetemporal.NewActivities(imagetemporal.ActivityDependencies{Repository: repository, SlotExecutor: dependencies.SlotExecutor, Publisher: dependencies.Publisher, PublisherV3: dependencies.PublisherV3, StagedSlotExecutor: dependencies.StagedSlotExecutor, ArtifactStore: dependencies.ArtifactStore, ExecutionAuthorizer: dependencies.ExecutionAuthorizer})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, closeClient, err := platformtemporal.Dial(ctx, platformtemporal.Config{Address: address, Namespace: "default"})
	require.NoError(t, err)
	defer closeClient()
	worker, err := imagetemporal.NewWorker(imagetemporal.WorkerConfig{Client: client, Activities: activities, WireMode: imagetemporal.WorkerWireModeOrganization})
	require.NoError(t, err)
	require.NoError(t, worker.Start())
	defer worker.Stop()
	identity := imageagent.ExecutionIdentity{ScopeProtocol: run.ScopeProtocol, RunID: run.ID, TenantID: run.TenantID, UserID: run.UserID, MemberID: run.MemberID, BusinessTaskID: run.BusinessTaskID}
	workflowID := "organization-v1:" + base64.RawURLEncoding.EncodeToString([]byte(run.TenantID)) + ":" + base64.RawURLEncoding.EncodeToString([]byte(run.UserID)) + ":" + base64.RawURLEncoding.EncodeToString([]byte(run.ID))
	err = imagetemporal.NewOrganizationClient(client).StartManual(ctx, imageagent.WorkflowStart{Run: run, Identity: identity, Plan: plan, AssetCatalog: catalog})
	require.NoError(t, err)
	defer func() {
		_ = client.TerminateWorkflow(context.Background(), workflowID, "", "controlled isolated test cleanup")
	}()
	effectIdentity := imageagent.SlotExternalEffectIdentity{RunScope: scope, PlanRevision: 1, SlotID: "main-1", Attempt: 1}
	for {
		effect, effectErr := repository.(imageagent.SlotExternalEffectV3Repository).GetSlotExternalEffectV3(ctx, effectIdentity)
		if effectErr == nil && effect.Phase == imageagent.SlotEffectV3Phase("provider_not_dispatched") {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("organization worker did not persist missing-admission refusal: effect=%v err=%v", effect.Phase, effectErr)
		case <-time.After(100 * time.Millisecond):
		}
	}
	require.Zero(t, providerCalls.Load(), "no generation or retired Review dispatch without generation admission")
	tracked.mu.Lock()
	require.Zero(t, tracked.reserveCalls, "retired future-Review reservation must not be called")
	tracked.mu.Unlock()
	var reservations int64
	require.NoError(t, db.Table("saas_usage_events").Where("source_type = ?", "ai_invocation_reservation").Count(&reservations).Error)
	require.Zero(t, reservations)
}

type trackedCommercialReservation struct {
	*aistore.GormInvocationRecorder
	mu           sync.Mutex
	reserveCalls int
}

func (r *trackedCommercialReservation) ReserveAIInvocationUsage(ctx context.Context, tenantID, memberID, invocationID string, maximumTokens int64, occurredAt time.Time) error {
	err := r.GormInvocationRecorder.ReserveAIInvocationUsage(ctx, tenantID, memberID, invocationID, maximumTokens, occurredAt)
	r.mu.Lock()
	r.reserveCalls++
	r.mu.Unlock()
	return err
}

type admissionExactAuthorizer struct{}

func (admissionExactAuthorizer) AuthorizeExecution(_ context.Context, identity imageagent.ExecutionIdentity) error {
	if identity.TenantID != "org-1" || identity.UserID != "actor-1" || identity.MemberID != "member-1" {
		return fmt.Errorf("controlled exact member mismatch")
	}
	return nil
}
