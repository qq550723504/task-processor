package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	imagestore "task-processor/internal/imageagent/store"
	imagetemporal "task-processor/internal/imageagent/temporal"
	"task-processor/internal/imageagent/temporal/temporaltest"
	assetpersistence "task-processor/internal/integration/persistence/product/asset"
	"task-processor/internal/platform/temporal"
	productasset "task-processor/internal/product/asset"
)

// This isolates the completed replay seam. The separate worker integration
// proves real generation and first human approval; here a real closed Temporal
// execution must not be updated after the immutable Product Asset fact exists.
func TestAcquisitionCompletedApprovalHTTPReplaysFromExactOwnerFact(t *testing.T) {
	dsn, address := os.Getenv("ISSUE487_TEST_DSN"), os.Getenv("ISSUE487_TEMPORAL_ADDRESS")
	if dsn == "" || address == "" {
		t.Skip("requires isolated ISSUE487_TEST_DSN and ISSUE487_TEMPORAL_ADDRESS")
	}
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := "replay487_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE SCHEMA "+schema).Error)
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		pool, _ := db.DB()
		_ = pool.Close()
		_ = root.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		rootPool, _ := root.DB()
		_ = rootPool.Close()
	})
	require.NoError(t, imagestore.AutoMigrateOrganizationScope(db))
	require.NoError(t, assetpersistence.AutoMigrate(db))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, closeClient, err := temporal.Dial(ctx, temporal.Config{Address: address, Namespace: "default"})
	require.NoError(t, err)
	defer closeClient()
	operationID, runID, actionID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	actorID, tenantID, memberID := "actor-1", "org-1", "member-1"
	workflowID := "organization-v1:" + base64.RawURLEncoding.EncodeToString([]byte(tenantID)) + ":" + base64.RawURLEncoding.EncodeToString([]byte(actorID)) + ":" + base64.RawURLEncoding.EncodeToString([]byte(runID))
	temporaltest.CompleteEmptyExecution(t, ctx, client, workflowID)
	identity := imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: tenantID, UserID: actorID, MemberID: memberID, BusinessTaskID: operationID, RunID: runID}
	workflowClient := imagetemporal.NewOrganizationClient(client)
	const digest = "completed-digest"
	require.Error(t, workflowClient.ApproveResults(ctx, imageagent.ApproveResultsCommand{RunID: runID, PlanRevision: 1, ResultDigest: digest, ActorID: actorID, ActionID: actionID, Identity: identity}), "the real Temporal execution is closed")
	repository := imagestore.NewOrganizationRepository(db)
	catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: "product-1", Title: "Controlled", SourceSnapshotVersion: 1}, Assets: []imageagent.AuthorizedAsset{{ID: "catalog-image-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example.test/source.png", Width: 2, Height: 2}}})
	require.NoError(t, err)
	plan := imageagent.Plan{Revision: 1, IdempotencyKey: "plan-1", SourceAssetIDs: []string{"catalog-image-1"}, CreatedBy: actorID, Slots: []imageagent.Slot{{ID: "main", Role: imageagent.SlotRoleMain, Status: imageagent.SlotStatusPending, SourceAssetIDs: []string{"catalog-image-1"}, IdempotencyKey: "slot-1"}}}
	ownerKey, err := imageagent.ArtifactOwnerKey(actorID)
	require.NoError(t, err)
	hash := strings.Repeat("a", 64)
	candidate := imageagent.AssetCandidate{AssetID: "approved-main-1", SourceAssetID: "catalog-image-1", Width: 2, Height: 2, DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: fmt.Sprintf("image-agent/public/%s/%s/%s/1/main/1/0-%s.png", tenantID, ownerKey, runID, hash), SHA256: hash}}
	acceptedSlot := plan.Slots[0]
	acceptedSlot.Status = imageagent.SlotStatusAccepted
	slots := []imageagent.SlotProjection{{Slot: acceptedSlot, Attempt: 1, Candidates: []imageagent.AssetCandidate{candidate}}}
	run := imageagent.Run{ID: runID, ScopeProtocol: imageagent.OrganizationScopeProtocol, BusinessTaskID: operationID, TenantID: tenantID, UserID: actorID, MemberID: memberID, Mode: imageagent.RunModeManual, TargetPlatform: "product", ImagePolicyContext: imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}, IdempotencyKey: "start-1", Status: imageagent.RunStatusCompleted, ActivePlanRevision: 1, Version: 1, StartedAt: time.Now().UTC()}
	_, err = repository.InitializeRun(ctx, imageagent.ProjectionInitialization{Scope: imageagent.ScopeForRun(run), Run: run, Plan: plan, Catalog: catalog, Snapshot: imageagent.RunProjection{Run: run, Plan: plan, Slots: slots, ResultDigest: digest, AssetCatalog: catalog}, CommitID: "start:" + run.IdempotencyKey, EventType: "run.initialized", EventPayload: json.RawMessage(`{}`)})
	require.NoError(t, err)
	assetWriter, err := assetpersistence.NewRepository(db)
	require.NoError(t, err)
	key := imagetemporal.ApprovalActionPublicationKey(actionID, runID, 1)
	_, err = assetWriter.CommitApproval(ctx, productasset.ApprovalCommit{TenantID: tenantID, ProductKey: "product-1", TargetPlatform: "product", SourceSnapshotVersion: 1, ActionID: key, Assets: []productasset.ApprovedAsset{{ID: candidate.AssetID, RunID: runID, PlanRevision: 1, SlotID: "main", Attempt: 1, Role: productasset.RoleMain, URL: acquisitionImagePublicURLs{}.PublicURL(candidate.DurableAsset.ObjectKey), SourceAssetID: candidate.SourceAssetID, Width: 2, Height: 2}}})
	require.NoError(t, err)
	reader, err := assetpersistence.NewBoundedApprovalCommitReader(db, 2<<20)
	require.NoError(t, err)
	service, err := imageagent.NewService(repository, workflowClient, unavailableReplayCatalog{}, imageagent.WithOrganizationScope())
	require.NoError(t, err)
	tracked := &trackedReplayService{Service: service}
	router := gin.New()
	for _, route := range acquisitionImageRoutes(tracked, &acquisitionImageCandidatesSpy{}, reader, func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }, acquisitionImagePublicURLs{}) {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	request := func(member, requestedAction string) int {
		t.Helper()
		body := `{"planRevision":1,"resultDigest":"` + digest + `","actionId":"` + requestedAction + `"}`
		r := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/sourcing/1688/acquisitions/"+operationID+"/main-image/runs/"+runID+"/approve", strings.NewReader(body))
		r = r.WithContext(authidentity.WithAuthenticatedIdentity(r.Context(), authidentity.AuthenticatedIdentity{TenantID: tenantID, EffectiveOrganizationID: tenantID, UserID: actorID, EffectiveMemberID: member}))
		r.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, r)
		return response.Code
	}
	var assetsBefore, receiptsBefore int64
	require.NoError(t, db.Table("product_approved_assets").Count(&assetsBefore).Error)
	require.NoError(t, db.Table("product_approval_receipts").Count(&receiptsBefore).Error)
	require.Equal(t, http.StatusAccepted, request(memberID, actionID))
	require.Equal(t, http.StatusConflict, request(memberID, uuid.NewString()))
	require.Equal(t, http.StatusForbidden, request("new-member", actionID), "Service rejects a replacement canonical member before receipt read")
	require.Zero(t, tracked.approvals.Load(), "completed replay must not update closed Temporal or dispatch providers")
	var assetsAfter, receiptsAfter int64
	require.NoError(t, db.Table("product_approved_assets").Count(&assetsAfter).Error)
	require.NoError(t, db.Table("product_approval_receipts").Count(&receiptsAfter).Error)
	require.Equal(t, assetsBefore, assetsAfter)
	require.Equal(t, receiptsBefore, receiptsAfter)
	require.NoError(t, db.Exec("UPDATE product_approval_receipts SET payload_hash = ? WHERE tenant_id = ? AND action_id = ?", strings.Repeat("0", 64), tenantID, key).Error)
	require.Equal(t, http.StatusServiceUnavailable, request(memberID, actionID), "corrupt owner fact must fail closed")
	require.Zero(t, tracked.approvals.Load())
}

type unavailableReplayCatalog struct{}

func (unavailableReplayCatalog) Resolve(context.Context, imageagent.AssetCatalogScope) (imageagent.AssetCatalog, error) {
	return imageagent.AssetCatalog{}, imageagent.ErrCommandBlocked
}

type trackedReplayService struct {
	*imageagent.Service
	approvals atomic.Int32
}

func (service *trackedReplayService) ApproveResults(ctx context.Context, runID string, revision int64, digest, actionID string) error {
	service.approvals.Add(1)
	return service.Service.ApproveResults(ctx, runID, revision, digest, actionID)
}
