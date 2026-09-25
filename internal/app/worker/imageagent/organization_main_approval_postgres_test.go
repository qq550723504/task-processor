package imageagentworker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	sdkclient "go.temporal.io/sdk/client"
	"gorm.io/gorm"
	"task-processor/internal/accountallocation"
	"task-processor/internal/aicapability"
	"task-processor/internal/aicapability/store"
	"task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/objectstore"
	imagestore "task-processor/internal/imageagent/store"
	imagetemporal "task-processor/internal/imageagent/temporal"
	openai "task-processor/internal/integration/openai"
	accountallocationstore "task-processor/internal/integration/persistence/accountallocation"
	assetpersistence "task-processor/internal/integration/persistence/product/asset"
	"task-processor/internal/listingsubscription"
	platformtemporal "task-processor/internal/platform/temporal"
)

func TestOrganizationWorkerRealTemporalControlledMainRequiresHumanApproval(t *testing.T) {
	address := os.Getenv("ISSUE487_TEMPORAL_ADDRESS")
	if address == "" {
		t.Skip("requires isolated Temporal ISSUE487_TEMPORAL_ADDRESS")
	}
	db := admissionPostgres(t, "org-1", "member-1", 10000)
	require.NoError(t, db.Exec(`UPDATE saas_tenant_entitlements SET limits = ? WHERE tenant_id = ?`, `{"ai_tokens":1000000}`, "org-1").Error)
	require.NoError(t, imagestore.AutoMigrateOrganizationScope(db))
	require.NoError(t, assetpersistence.AutoMigrate(db))
	require.NoError(t, db.AutoMigrate(&openai.AIClientCredential{}))
	require.NoError(t, store.AutoMigrateInvocationLedger(db))
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	var pngBody bytes.Buffer
	require.NoError(t, png.Encode(&pngBody, img))
	imageBytes := pngBody.Bytes()
	var imageCalls, reviewCalls, unexpectedCalls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/images/edits":
			imageCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(imageBytes)}}})
		case "/v1/chat/completions":
			reviewCalls.Add(1)
			_, _ = w.Write([]byte(`{"id":"controlled-review","choices":[{"message":{"role":"assistant","content":"{\"score\":0.9,\"needs_human_review\":false,\"reasons\":[]}"}}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`))
		default:
			unexpectedCalls.Add(1)
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()
	credentials := openai.NewGormCredentialResolver(db)
	for _, name := range []string{"default", "image_gpt_image_2"} {
		require.NoError(t, credentials.SaveCredential(context.Background(), openai.AIClientCredential{TenantID: "org-1", UserID: "actor-1", ClientName: name, APIKey: "controlled-local-only", BaseURL: provider.URL + "/v1", Model: "review-test", Enabled: true, TimeoutSecond: 2}))
	}
	reference := &http.Client{Transport: controlledImageReference{body: imageBytes}}
	defaultConfig := openai.NewClientConfig("unused", "review-test", provider.URL+"/v1", 2)
	imageConfig := openai.NewClientConfig("unused", "image-test", provider.URL+"/v1", 2)
	defaultConfig.ImageReferenceHTTPClient, imageConfig.ImageReferenceHTTPClient = reference, reference
	manager, err := openai.NewManager(&openai.ManagerConfig{Clients: map[string]*openai.ClientConfig{"default": defaultConfig, "image_gpt_image_2": imageConfig}, DefaultClient: "default"})
	require.NoError(t, err)
	recorder := store.NewGormInvocationRecorder(db)
	recorder.SetUsageSettler(listingsubscription.AIInvocationUsageAdapter{Repository: listingsubscription.NewGormRepository(db)})
	cfg := &config.Config{Database: &config.DatabaseConfig{}, CommercialDatabase: &config.DatabaseConfig{}}
	cfg.ImageAgent.ArtifactStore = durableArtifactStoreConfig("aws", true)
	resolver := imageAgentWorkerDependencyResolver{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		OpenDB:     func(*config.DatabaseConfig) (*gorm.DB, error) { return db, nil },
		CloseDB:    func(*config.DatabaseConfig, *gorm.DB) error { return nil },
		BuildAI: func(*config.Config, *gorm.DB, *gorm.DB, *logrus.Logger) (*openai.Manager, openai.ClientConfigResolver, aicapability.InvocationRecorder, error) {
			return manager, nil, recorder, nil
		},
		BuildOrganizationCapabilities: buildOrganizationWorkerCapabilities,
		BuildOrganizationAuthorizer:   func(*config.Config) (imageagent.ExecutionAuthorizer, error) { return admissionExactAuthorizer{}, nil },
		BuildArtifactStore: func(*config.Config, imageAgentArtifactTiming, *logrus.Logger) (imagetemporal.DurableArtifactStore, error) {
			return controlledApprovalArtifactStore{}, nil
		},
	}
	dependencies, closeDependencies, err := resolveImageAgentTemporalDependenciesForMode("controlled", logrus.New(), imagetemporal.WorkerWireModeOrganization, resolver)
	require.NoError(t, err)
	defer closeDependencies()
	runID := "run487-" + uuid.NewString()[:8]
	budget := imageagent.Budget{MaxImages: 3, EnabledLimits: imageagent.BudgetLimitImages}
	policy, err := budget.Policy()
	require.NoError(t, err)
	policyContext := imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}
	run := imageagent.Run{ID: runID, ScopeProtocol: imageagent.OrganizationScopeProtocol, BusinessTaskID: "catalog-receipt-1", TenantID: "org-1", UserID: "actor-1", MemberID: "member-1", Mode: imageagent.RunModeManual, TargetPlatform: "product", ImagePolicyContext: policyContext, IdempotencyKey: "start-" + runID, Status: imageagent.RunStatusExecuting, ActivePlanRevision: 1, Version: 1, Budget: budget, MaxConcurrentSlots: 1, StartedAt: time.Now().UTC()}
	plan := imageagent.Plan{Revision: 1, IdempotencyKey: "plan-" + runID, SourceAssetIDs: []string{"catalog-image-1"}, CreatedBy: run.UserID, Slots: []imageagent.Slot{{ID: "main", Role: imageagent.SlotRoleMain, Status: imageagent.SlotStatusPending, SourceAssetIDs: []string{"catalog-image-1"}, IdempotencyKey: "slot-" + runID}}}
	catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: "product-1", Title: "Controlled product", SourceSnapshotVersion: 1}, Assets: []imageagent.AuthorizedAsset{{ID: "catalog-image-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/product.png", Width: 1200, Height: 1200}}})
	require.NoError(t, err)
	scope := imageagent.ScopeForRun(run)
	repository := dependencies.Repository
	_, err = repository.InitializeRun(context.Background(), imageagent.ProjectionInitialization{Scope: scope, Run: run, Plan: plan, Catalog: catalog, Snapshot: imageagent.RunProjection{Run: run, Plan: plan}, CommitID: "start:" + run.IdempotencyKey, EventType: "run.initialized", EventPayload: []byte(`{}`)})
	require.NoError(t, err)
	activities, err := imagetemporal.NewActivities(imagetemporal.ActivityDependencies{Repository: repository, SlotExecutor: dependencies.SlotExecutor, Publisher: dependencies.Publisher, PublisherV3: dependencies.PublisherV3, StagedSlotExecutor: dependencies.StagedSlotExecutor, ArtifactStore: dependencies.ArtifactStore, ExecutionAuthorizer: dependencies.ExecutionAuthorizer})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	_, err = client.ExecuteWorkflow(ctx, sdkclient.StartWorkflowOptions{ID: workflowID, TaskQueue: imagetemporal.OrganizationTaskQueue}, "ImageAgentOrganizationWorkflowV1", imagetemporal.WorkflowInput{RunID: run.ID, Identity: identity, Mode: run.Mode, TargetPlatform: run.TargetPlatform, ImagePolicyContext: &policyContext, Plan: plan, MaxConcurrentSlots: 1, WaitForCommands: true, AssetCatalog: catalog, BudgetPolicy: policy, StartedAt: run.StartedAt})
	require.NoError(t, err)
	defer func() {
		_ = client.TerminateWorkflow(context.Background(), workflowID, "", "controlled isolated test cleanup")
	}()
	var projection imageagent.RunProjection
	for {
		projection, err = repository.GetProjection(ctx, scope)
		require.NoError(t, err)
		if projection.Run.Status == imageagent.RunStatusAwaitingFinalApproval || projection.Run.Status == imageagent.RunStatusBlocked || projection.Run.Status == imageagent.RunStatusFailed {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("controlled main workflow did not reach approval or terminal block")
		case <-time.After(100 * time.Millisecond):
		}
	}
	require.Equal(t, imageagent.RunStatusAwaitingFinalApproval, projection.Run.Status, "block=%+v", projection.Run.Block)
	require.NotEmpty(t, projection.ResultDigest)
	require.EqualValues(t, 2, imageCalls.Load())
	require.EqualValues(t, 1, reviewCalls.Load())
	require.Zero(t, unexpectedCalls.Load())
	var approved int64
	require.NoError(t, db.Table("product_approved_assets").Count(&approved).Error)
	require.Zero(t, approved, "generation and QA never auto-approve Product Asset")
	var usage []struct {
		Quantity int64
		MemberID string
		Status   string
	}
	require.NoError(t, db.Table("saas_usage_events").Where("source_type = ?", "ai_invocation").Find(&usage).Error)
	require.Len(t, usage, 1)
	require.EqualValues(t, 7, usage[0].Quantity)
	require.Equal(t, "member-1", usage[0].MemberID)
	require.Equal(t, "committed", usage[0].Status)
	allocationReader, err := accountallocationstore.New(db)
	require.NoError(t, err)
	var window struct{ WindowStart, WindowEnd time.Time }
	require.NoError(t, db.Table("account_member_token_allocations").Where("organization_id = ? AND member_id = ?", run.TenantID, run.MemberID).Select("window_start, window_end").Take(&window).Error)
	accountView, err := allocationReader.Snapshot(ctx, accountallocation.Quota{OrganizationID: run.TenantID, Metric: accountallocation.MetricToken, Total: 1000000, WindowStart: window.WindowStart, WindowEnd: window.WindowEnd})
	require.NoError(t, err)
	require.EqualValues(t, 7, accountView.Enterprise.Consumed)
	require.Len(t, accountView.Allocations, 1)
	require.Equal(t, run.MemberID, accountView.Allocations[0].MemberID)
	require.EqualValues(t, 7, accountView.Allocations[0].Consumed)
	require.EqualValues(t, 9993, accountView.Allocations[0].Remaining)
	approval := imagetemporal.NewOrganizationClient(client)
	require.NoError(t, approval.ApproveResults(ctx, imageagent.ApproveResultsCommand{RunID: run.ID, PlanRevision: 1, ResultDigest: projection.ResultDigest, ActorID: run.UserID, ActionID: "human-approval-1", Identity: identity}))
	for {
		projection, err = repository.GetProjection(ctx, scope)
		require.NoError(t, err)
		if projection.Run.Status == imageagent.RunStatusCompleted {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("human approval did not persist completion")
		case <-time.After(100 * time.Millisecond):
		}
	}
	require.NoError(t, db.Table("product_approved_assets").Where("tenant_id = ? AND run_id = ?", run.TenantID, run.ID).Count(&approved).Error)
	require.EqualValues(t, 1, approved)
	var receipts int64
	require.NoError(t, db.Table("product_approval_receipts").Where("tenant_id = ?", run.TenantID).Count(&receipts).Error)
	require.EqualValues(t, 1, receipts)
	approvalReader, err := assetpersistence.NewBoundedApprovalCommitReader(db, 2<<20)
	require.NoError(t, err)
	commit, err := approvalReader.ReadApprovalCommit(ctx, run.TenantID, imagetemporal.ApprovalActionPublicationKey("human-approval-1", run.ID, 1))
	require.NoError(t, err)
	require.Equal(t, run.TenantID, commit.TenantID)
	require.Equal(t, run.ID, commit.Assets[0].RunID)
	require.EqualValues(t, 1, commit.Assets[0].PlanRevision)
	require.Equal(t, "product-1", commit.ProductKey)
	require.Equal(t, "product", commit.TargetPlatform)
	require.EqualValues(t, 1, commit.SourceSnapshotVersion)
	require.Len(t, projection.Slots, 1)
	require.Len(t, projection.Slots[0].Candidates, 1)
	candidate := projection.Slots[0].Candidates[0]
	require.Equal(t, candidate.AssetID, commit.Assets[0].ID)
	require.Equal(t, controlledApprovalArtifactStore{}.PublicURL(candidate.DurableAsset.ObjectKey), commit.Assets[0].URL)
	require.Equal(t, candidate.SourceAssetID, commit.Assets[0].SourceAssetID)
	require.Equal(t, candidate.Width, commit.Assets[0].Width)
	require.Equal(t, candidate.Height, commit.Assets[0].Height)
}

type controlledApprovalArtifactStore struct{ stubWorkerArtifactStore }

func (controlledApprovalArtifactStore) PrepareSlotArtifacts(input objectstore.PrepareSlotArtifactsInput) (objectstore.PreparedSlotArtifacts, error) {
	ownerKey, err := imageagent.ArtifactOwnerKey(input.Identity.OwnerUserID)
	if err != nil {
		return objectstore.PreparedSlotArtifacts{}, err
	}
	assets := make([]imageagent.StagedAssetRef, len(input.Assets))
	for index, asset := range input.Assets {
		operations, err := imageagent.NormalizeArtifactOperations(asset.Operations)
		if err != nil {
			return objectstore.PreparedSlotArtifacts{}, err
		}
		hash := sha256.Sum256(asset.Bytes)
		encoded := hex.EncodeToString(hash[:])
		assets[index] = imageagent.StagedAssetRef{ObjectKey: fmt.Sprintf("image-agent/staging/%s/%s/%s/%d/%s/%d/%d-%s.png", input.Identity.TenantID, ownerKey, input.Identity.RunID, input.Identity.PlanRevision, input.Identity.SlotID, input.Identity.Attempt, index, encoded), SHA256: encoded, SizeBytes: int64(len(asset.Bytes)), ContentType: asset.ContentType, Width: asset.Width, Height: asset.Height, SourceAssetID: asset.SourceAssetID, Operations: operations, ProviderReceiptID: asset.ProviderReceiptID}
	}
	manifest, err := imageagent.NormalizeStagingManifest(imageagent.StagingManifest{Assets: assets})
	return objectstore.PreparedSlotArtifacts{Manifest: manifest}, err
}

func (controlledApprovalArtifactStore) Finalize(ctx context.Context, manifest imageagent.StagingManifest) (imageagent.FinalManifest, error) {
	return controlledApprovalArtifactStore{}.FinalizeWithProgress(ctx, manifest, nil)
}

func (controlledApprovalArtifactStore) FinalizeWithProgress(ctx context.Context, manifest imageagent.StagingManifest, progress func(context.Context, int) error) (imageagent.FinalManifest, error) {
	manifest, err := imageagent.NormalizeStagingManifest(manifest)
	if err != nil {
		return imageagent.FinalManifest{}, err
	}
	assets := make([]imageagent.PublishedAssetRef, len(manifest.Assets))
	for index, asset := range manifest.Assets {
		if progress != nil {
			if err := progress(ctx, index); err != nil {
				return imageagent.FinalManifest{}, err
			}
		}
		assets[index] = imageagent.PublishedAssetRef{ObjectKey: "image-agent/public/" + strings.TrimPrefix(asset.ObjectKey, "image-agent/staging/"), SHA256: asset.SHA256, SizeBytes: asset.SizeBytes, ContentType: asset.ContentType, Width: asset.Width, Height: asset.Height, SourceAssetID: asset.SourceAssetID, Operations: asset.Operations, ProviderReceiptID: asset.ProviderReceiptID}
	}
	return imageagent.NormalizeFinalManifest(imageagent.FinalManifest{Assets: assets})
}
