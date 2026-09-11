package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/app/productsourcing"
	imageagentworker "task-processor/internal/app/worker/imageagent"
	"task-processor/internal/authidentity"
	zitadel "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/assetpublication"
	"task-processor/internal/imageagent/objectstore"
	imagestore "task-processor/internal/imageagent/store"
	imageagenttemporal "task-processor/internal/imageagent/temporal"
	chain1temporal "task-processor/internal/imageagent/temporal/acceptancetest"
	imageagenttools "task-processor/internal/imageagent/tools"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/listing/record"
	listingtask "task-processor/internal/listing/task"
	"task-processor/internal/listingsubscription"
	imagepolicy "task-processor/internal/marketplace/imagepolicy"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/enrichment"
	productimage "task-processor/internal/product/image"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/storecenter"
	"task-processor/internal/workbenchcontext"
)

const (
	chain1Organization = "org-chain1-388"
	chain1Actor        = "actor-chain1-388"
	chain1Project      = "project-chain1-388"
	chain1UserToken    = "chain1-user-token"
	chain1ServiceToken = "chain1-worker-token"
	chain1ProductKey   = "chain1-product"
)

func TestChain1ControlledImagePortsExerciseCurrentProductSlotExecutor(t *testing.T) {
	imagePorts := &chain1ImagePorts{}
	executor := imageagenttools.NewProductImageSlotExecutor(imageagenttools.Dependencies{
		SubjectExtractor: imagePorts, WhiteBackgroundRenderer: imagePorts, SceneRenderer: imagePorts,
		Reviewer: imagePorts, UsageQuoter: imagePorts, ProfileResolver: imagePorts,
	})
	catalogInput, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{
		Assets:         []imageagent.AuthorizedAsset{{ID: "catalog-image-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example.test/chain1.png", SourceURL: "https://source.example.test/chain1.png", DisplayURL: "https://source.example.test/chain1.png", Width: 1, Height: 1}},
		ProductContext: imageagent.ProductContextRef{ProductID: chain1ProductKey, Title: "Reviewed chain product", ProductType: "Home / Drinkware", SourceSnapshotVersion: 2},
	})
	require.NoError(t, err)
	result, err := executor.GenerateSlot(context.Background(), imageagent.SlotExecutionInput{
		RunID: "chain1-run-388", TenantID: chain1Organization, UserID: chain1Actor, TargetPlatform: "shein",
		ImagePolicyContext: &imageagent.ImagePolicyContext{Country: "us", Family: "default", SceneCategory: "studio"},
		PlanRevision:       1, Slot: imageagent.Slot{ID: "main", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"catalog-image-1"}, IdempotencyKey: "chain1-main-key-388", Status: imageagent.SlotStatusPending},
		Attempt: 1, IdempotencyKey: "chain1-main-key-388:plan:1:attempt:1", AssetCatalog: catalogInput, ProductContext: catalogInput.ProductContext,
	})
	require.NoError(t, err)
	require.Len(t, result.Assets, 1)
	require.Equal(t, chain1PNG, result.Assets[0].Bytes)
	require.Positive(t, imagePorts.calls.Load())
}

// TestChain1LocalProductAcceptance is opt-in because its assertions require a
// task-owned empty PostgreSQL database and a real Temporal server. The only
// substitutes below implement declared external ports; all internal owners,
// authorization decisions, repositories, activities and workflows are real.
func TestChain1LocalProductAcceptance(t *testing.T) {
	dsn, temporalAddress := os.Getenv("CHAIN1_ACCEPTANCE_DSN"), os.Getenv("CHAIN1_TEMPORAL_ADDRESS")
	if dsn == "" || temporalAddress == "" {
		t.Skip("requires CHAIN1_ACCEPTANCE_DSN and CHAIN1_TEMPORAL_ADDRESS")
	}

	db := chain1Database(t, dsn)
	chain1InstallSchemas(t, db)
	tableCount := chain1TableCount(t, db)

	grants := &chain1GrantFixture{}
	grants.userAllowed.Store(true)
	grants.workerAllowed.Store(true)
	grantServer := httptest.NewServer(grants)
	t.Cleanup(grantServer.Close)
	authorizationClient := zitadel.NewAuthorizationClient(grantServer.URL, grantServer.Client())
	resolver := workbenchcontext.NewResolver(workbenchcontext.NewGrantResolver(authorizationClient, nil), chain1Project, "chain1-v1", nil)
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	verifier := chain1TokenVerifier{}

	// SRC-1: build its private request capability from the same verified token,
	// current Organization resolver and ListingKit authorizer used by HTTP.
	sourceContext := chain1LiveWriteContext(t, resolver, verifier)
	sourceContext, err = (productReviewCapabilityBinder{now: time.Now}).Bind(sourceContext, "Bearer "+chain1UserToken)
	require.NoError(t, err)
	sourceProducer, err := productsourcing.NewInternalProducer(db, &productReviewLiveOrganizationAccess{resolver: resolver, now: time.Now}, authorizer)
	require.NoError(t, err)
	envelope := chain1SourceEnvelope("source-v1", "Chain product")
	sourceReceipt, err := sourceProducer.Publish(sourceContext, sourcing.PublicationCommand{
		PublicationID: "chain1-source-publication-v1",
		Producer:      sourcing.ProducerDescriptor{Kind: sourcing.ControlledSnapshotProducerKind, Version: sourcing.ControlledSnapshotProducerVersion},
		ProductKey:    chain1ProductKey, Envelope: envelope,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, sourceReceipt.CatalogVersion)
	sourceReplay, err := sourceProducer.Publish(sourceContext, sourcing.PublicationCommand{
		PublicationID: "chain1-source-publication-v1",
		Producer:      sourcing.ProducerDescriptor{Kind: sourcing.ControlledSnapshotProducerKind, Version: sourcing.ControlledSnapshotProducerVersion},
		ProductKey:    chain1ProductKey, Envelope: envelope,
	})
	require.NoError(t, err)
	require.Equal(t, sourceReceipt, sourceReplay)
	conflictEnvelope := envelope
	conflictEnvelope.ProductCandidate.Title = "different payload"
	_, err = sourceProducer.Publish(sourceContext, sourcing.PublicationCommand{
		PublicationID: sourceReceipt.PublicationID,
		Producer:      sourcing.ProducerDescriptor{Kind: sourcing.ControlledSnapshotProducerKind, Version: sourcing.ControlledSnapshotProducerVersion},
		ProductKey:    chain1ProductKey, Envelope: conflictEnvelope,
	})
	require.ErrorIs(t, err, sourcing.ErrSourcePublicationConflict)

	// REV-1: actual HTTP create/accept/apply. The first committed Apply response
	// is deliberately discarded, then the application is rebuilt and replayed.
	reviewGenerator := &chain1TitleGenerator{}
	reviewApp, err := NewProductReviewApplication(db, verifier, resolver, authorizer, reviewGenerator)
	require.NoError(t, err)
	reviewServer := chain1HTTPServer(t, reviewApp)
	created := chain1ReviewRequest(t, reviewServer, http.MethodPost, "/api/product/text-proposals", "chain1-review-create", fmt.Sprintf(`{"product_key":%q,"base_version":%d}`, chain1ProductKey, sourceReceipt.CatalogVersion), http.StatusOK)
	require.Equal(t, "pending", created.State)
	revision := chain1Uint(t, created.Revision)
	accepted := chain1ReviewRequest(t, reviewServer, http.MethodPost, "/api/product/text-proposals/"+created.ProposalID+"/decisions", "chain1-review-accept", fmt.Sprintf(`{"action":"accept","expected_revision":%d}`, revision), http.StatusOK)
	require.Equal(t, "accepted", accepted.State)

	lostReview := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := httptest.NewRecorder()
		reviewApp.Handler.ServeHTTP(recorder, r)
		if recorder.Code == http.StatusOK {
			if connection, _, hijackErr := w.(http.Hijacker).Hijack(); hijackErr == nil {
				_ = connection.Close()
				return
			}
		}
		w.WriteHeader(recorder.Code)
		_, _ = w.Write(recorder.Body.Bytes())
	}))
	applyBody := fmt.Sprintf(`{"expected_revision":%d}`, chain1Uint(t, accepted.Revision))
	_, _, err = chain1RawRequest(lostReview.Client(), http.MethodPost, lostReview.URL+"/api/product/text-proposals/"+created.ProposalID+"/apply", "chain1-review-apply", applyBody, chain1Organization)
	require.Error(t, err, "the committed Apply response must be lost")
	lostReview.Close()
	reviewServer.Close()
	rebuiltReviewApp, err := NewProductReviewApplication(db, verifier, resolver, authorizer, reviewGenerator)
	require.NoError(t, err)
	rebuiltReview := chain1HTTPServer(t, rebuiltReviewApp)
	applied := chain1ReviewRequest(t, rebuiltReview, http.MethodPost, "/api/product/text-proposals/"+created.ProposalID+"/apply", "chain1-review-apply", applyBody, http.StatusOK)
	require.NotNil(t, applied.ApplyReceipt)
	appliedVersion := chain1Uint(t, applied.ApplyReceipt.ProductVersion)
	require.Greater(t, appliedVersion, sourceReceipt.CatalogVersion)

	exactCatalogReader, err := catalogstore.NewBoundedSnapshotReader(db, sourcing.MaxEncodedSnapshotBytes)
	require.NoError(t, err)
	exactProduct, err := exactCatalogReader.GetSnapshot(context.Background(), catalog.SnapshotIdentity{TenantID: chain1Organization, ProductKey: chain1ProductKey}, appliedVersion)
	require.NoError(t, err)
	require.Equal(t, applied.ApplyReceipt.PublicationID, exactProduct.PublicationID)
	require.Equal(t, "Reviewed chain product", exactProduct.Snapshot.Title)

	// Store Center: use current Subscription, quota, audit and Store creation
	// state machine. No final Store or entitlement row is inserted by the test.
	subscriptionRepository := listingsubscription.NewGormRepository(db)
	subscriptionService, err := listingsubscription.NewService(subscriptionRepository)
	require.NoError(t, err)
	_, err = subscriptionService.ApplyPlan(context.Background(), chain1Organization, listingsubscription.PlanApplyInput{PlanCode: listingsubscription.PlanProfessional, Status: listingsubscription.StatusActive}, chain1Actor)
	require.NoError(t, err)
	storeRepository, err := storecenter.NewGormStoreRepository(db)
	require.NoError(t, err)
	auditRepository, err := storecenter.NewGormAuditRepository(db)
	require.NoError(t, err)
	storeService, err := storecenter.NewService(storeRepository, listingsubscription.NewGormStoreQuotaLedger(subscriptionRepository), auditRepository, chain1ConnectionStatus{}, time.Now)
	require.NoError(t, err)
	storeCreate := storecenter.CreateStoreRequest{
		OrganizationID: chain1Organization, ActorSubject: chain1Actor, IdempotencyKey: uuid.NewString(),
		Name: "CHAIN-1 controlled store", Platform: "shein", Region: "US", ExternalStoreID: "chain1-external-store",
	}
	storeResult, err := storeService.Create(context.Background(), storeCreate)
	require.NoError(t, err)
	require.False(t, storeResult.Replayed)
	storeID := storeResult.Store.ID()
	storeReplay, err := storeService.Create(context.Background(), storeCreate)
	require.NoError(t, err)
	require.True(t, storeReplay.Replayed)
	require.Equal(t, storeID, storeReplay.Store.ID())

	// DRAFT-S1 is not ready for the exact applied version before approval.
	draftApp, _, err := NewSheinRecordApplication(db, verifier, resolver, authorizer)
	require.NoError(t, err)
	draftServer := chain1HTTPServer(t, draftApp)
	draftBody := chain1DraftBody(chain1ProductKey, appliedVersion, storeID)
	status, beforeApproval, err := chain1RawRequest(draftServer.Client(), http.MethodPost, draftServer.URL+"/api/listing/shein-records", "chain1-draft-before", draftBody, chain1Organization)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnprocessableEntity, status, string(beforeApproval))
	require.JSONEq(t, `{"error":"not_ready"}`, string(beforeApproval))

	// Organization ImageAgent: real PG repository, real Temporal server/client,
	// organization worker mode and current worker authorization. Only declared
	// provider and immutable object-storage ports are controlled fixtures.
	temporalDriver := chain1TemporalDriver(t, temporalAddress)
	defer func() { _ = temporalDriver.Close() }()
	imageRepository := imagestore.NewOrganizationRepository(db)
	assetRepository, err := assetstore.NewRepository(db)
	require.NoError(t, err)
	objects := newChain1ObjectStore()
	durableArtifacts, err := objectstore.NewDurableArtifactStore(objects, objectstore.DurableArtifactStoreConfig{MaxArtifactBytes: 1 << 20, OperationTimeout: 5 * time.Second})
	require.NoError(t, err)
	imagePorts := &chain1ImagePorts{}
	executor := imageagenttools.NewProductImageSlotExecutor(imageagenttools.Dependencies{
		SubjectExtractor: imagePorts, WhiteBackgroundRenderer: imagePorts, SceneRenderer: imagePorts,
		Reviewer: imagePorts, UsageQuoter: imagePorts, ProfileResolver: imagePorts,
	})
	publisherV2, err := assetpublication.NewV2Publisher(imageRepository, assetRepository)
	require.NoError(t, err)
	publisherV3, err := assetpublication.NewPublisher(imageRepository, assetRepository, durableArtifacts)
	require.NoError(t, err)
	executionAuthorizer := imageagentworker.OrganizationExecutionAuthorizer{
		Client: authorizationClient, ServiceToken: func(context.Context) (string, error) { return chain1ServiceToken, nil },
		ProjectID: chain1Project, Authorizer: authorizer,
	}
	activityDependencies := imageagenttemporal.ActivityDependencies{
		ExecutionAuthorizer: executionAuthorizer, Repository: imageRepository, SlotExecutor: executor,
		Publisher: publisherV2, PublisherV3: publisherV3, StagedSlotExecutor: executor, ArtifactStore: durableArtifacts,
	}
	activities, err := imageagenttemporal.NewActivities(activityDependencies)
	require.NoError(t, err)
	chain1StartWorker(t, temporalDriver, activities)
	workflowClient := temporalDriver.OrganizationClient()
	imageApp := chain1ImageApplication(t, db, verifier, resolver, authorizer, workflowClient, exactProduct)
	imageServer := chain1HTTPServer(t, imageApp)
	runID, actionID := "chain1-run-388", "chain1-approve-388"
	createRun := fmt.Sprintf(`{"run_id":%q,"business_task_id":"chain1-context-388","target_platform":"shein","image_policy_context":{"country":"us","family":"default","scene_category":"studio"},"mode":"manual","idempotency_key":"chain1-run-key-388","plan":{"revision":1,"idempotency_key":"chain1-plan-key-388","source_asset_ids":["catalog-image-1"],"slots":[{"id":"main","role":"main","source_asset_ids":["catalog-image-1"],"idempotency_key":"chain1-main-key-388","status":"pending"}]},"budget":{},"max_concurrent_slots":1}`, runID)
	status, raw, err := chain1RawRequest(imageServer.Client(), http.MethodPost, imageServer.URL+"/api/organization/image-agent/runs", "", createRun, chain1Organization)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, status, string(raw))
	status, raw, err = chain1RawRequest(imageServer.Client(), http.MethodPost, imageServer.URL+"/api/organization/image-agent/runs", "", createRun, chain1Organization)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, status, string(raw))
	awaiting := chain1WaitRun(t, imageServer, runID, imageagent.RunStatusAwaitingFinalApproval, 45*time.Second)
	require.NotEmpty(t, awaiting.ResultDigest)
	require.EqualValues(t, 1, awaiting.Plan.Revision)

	// Persisted workflow history survives both worker and Temporal server restart.
	temporalDriver.StopWorker()
	imageServer.Close()
	_ = temporalDriver.Close()
	chain1RestartTemporal(t)
	temporalDriver = chain1TemporalDriver(t, temporalAddress)
	activities, err = imageagenttemporal.NewActivities(activityDependencies)
	require.NoError(t, err)
	chain1StartWorker(t, temporalDriver, activities)
	workflowClient = temporalDriver.OrganizationClient()
	imageApp = chain1ImageApplication(t, db, verifier, resolver, authorizer, workflowClient, exactProduct)
	imageServer = chain1HTTPServer(t, imageApp)

	wrongApproval := fmt.Sprintf(`{"plan_revision":1,"result_digest":"sha256:%s","action_id":"chain1-wrong-approve-388"}`, strings.Repeat("0", 64))
	status, _, err = chain1RawRequest(imageServer.Client(), http.MethodPost, imageServer.URL+"/api/organization/image-agent/runs/"+runID+"/results/approve", "", wrongApproval, chain1Organization)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, status)
	grants.userAllowed.Store(false)
	approvalBody := fmt.Sprintf(`{"plan_revision":1,"result_digest":%q,"action_id":%q}`, awaiting.ResultDigest, actionID)
	status, _, err = chain1RawRequest(imageServer.Client(), http.MethodPost, imageServer.URL+"/api/organization/image-agent/runs/"+runID+"/results/approve", "", approvalBody, chain1Organization)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, status)
	grants.userAllowed.Store(true)

	// The HTTP update is accepted under current user auth, but its publication
	// activity cannot cross the separately revoked worker grant. Lose that HTTP
	// response, restore the grant, then reconstruct from durable owners.
	grants.workerAllowed.Store(false)
	workerCallsBeforeRevoke := grants.workerCalls.Load()
	lostApproval := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := httptest.NewRecorder()
		imageApp.Handler.ServeHTTP(recorder, r)
		if connection, _, hijackErr := w.(http.Hijacker).Hijack(); hijackErr == nil {
			_ = connection.Close()
			return
		}
		w.WriteHeader(recorder.Code)
		_, _ = w.Write(recorder.Body.Bytes())
	}))
	lostDone := make(chan error, 1)
	go func() {
		_, _, requestErr := chain1RawRequest(lostApproval.Client(), http.MethodPost, lostApproval.URL+"/api/organization/image-agent/runs/"+runID+"/results/approve", "", approvalBody, chain1Organization)
		lostDone <- requestErr
	}()
	chain1WaitCounter(t, &grants.workerCalls, workerCallsBeforeRevoke+1, 10*time.Second)
	_, err = assetRepository.GetApprovedInventory(context.Background(), productasset.InventoryScope{TenantID: chain1Organization, ProductKey: chain1ProductKey, TargetPlatform: "shein", SourceSnapshotVersion: appliedVersion})
	require.ErrorIs(t, err, productasset.ErrApprovedAssetsNotReady)
	grants.workerAllowed.Store(true)
	select {
	case lostErr := <-lostDone:
		require.Error(t, lostErr, "approval response must be lost after the durable command completes")
	case <-time.After(45 * time.Second):
		t.Fatal("approval command did not complete after worker authorization was restored")
	}
	lostApproval.Close()
	inventory := chain1WaitInventory(t, assetRepository, appliedVersion, 45*time.Second)
	require.Len(t, inventory.Assets, 1)
	require.Equal(t, runID, inventory.Assets[0].RunID)
	require.EqualValues(t, 1, inventory.Assets[0].PlanRevision)
	require.Equal(t, productasset.RoleMain, inventory.Assets[0].Role)
	completed := chain1WaitRun(t, imageServer, runID, imageagent.RunStatusCompleted, 45*time.Second)
	require.Equal(t, awaiting.ResultDigest, completed.ResultDigest)
	var approvalAction string
	require.NoError(t, db.Raw("SELECT action_id FROM product_approval_receipts WHERE tenant_id = ?", chain1Organization).Row().Scan(&approvalAction))
	require.Equal(t, chain1ApprovalPublicationKey(actionID, runID, 1), approvalAction)
	require.Equal(t, 1, objects.countPrefix("image-agent/public/"))

	// Rebuild DRAFT-S1, persist exact-version output, replay it, and prove a
	// newer Catalog version does not inherit the old approval.
	draftServer.Close()
	draftApp, reader, err := NewSheinRecordApplication(db, verifier, resolver, authorizer)
	require.NoError(t, err)
	draftServer = chain1HTTPServer(t, draftApp)
	status, draftRaw, err := chain1RawRequest(draftServer.Client(), http.MethodPost, draftServer.URL+"/api/listing/shein-records", "chain1-draft-save", draftBody, chain1Organization)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status, string(draftRaw))
	var draftReceipt record.Receipt
	require.NoError(t, json.Unmarshal(draftRaw, &draftReceipt))
	require.NotEmpty(t, draftReceipt.RecordID)
	require.True(t, draftReceipt.DiagnosticOnly)
	status, replayRaw, err := chain1RawRequest(draftServer.Client(), http.MethodPost, draftServer.URL+"/api/listing/shein-records", "chain1-draft-save", draftBody, chain1Organization)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)
	require.JSONEq(t, string(draftRaw), string(replayRaw))
	stored, err := reader.ReadOfflinePackage(context.Background(), listingtask.Actor{TenantID: chain1Organization, UserID: chain1Actor, Roles: []string{"listingkit_admin"}}, draftReceipt.RecordID)
	require.NoError(t, err)
	require.Equal(t, appliedVersion, stored.Input.SnapshotVersion)
	require.Equal(t, storeID, stored.Input.StoreID)
	require.Equal(t, inventory.Assets[0].URL, func() string {
		var payload map[string]any
		require.NoError(t, json.Unmarshal(stored.Payload, &payload))
		images := payload["images"].(map[string]any)
		return images["main_image"].(string)
	}())

	expectedBase := appliedVersion
	nextEnvelope := chain1SourceEnvelope("source-v2", "Chain product v2")
	nextReceipt, err := sourceProducer.Publish(sourceContext, sourcing.PublicationCommand{
		PublicationID: "chain1-source-publication-v2",
		Producer:      sourcing.ProducerDescriptor{Kind: sourcing.ControlledSnapshotProducerKind, Version: sourcing.ControlledSnapshotProducerVersion},
		ProductKey:    chain1ProductKey, ExpectedBaseVersion: &expectedBase, Envelope: nextEnvelope,
	})
	require.NoError(t, err)
	require.Equal(t, appliedVersion+1, nextReceipt.CatalogVersion)
	status, _, err = chain1RawRequest(draftServer.Client(), http.MethodPost, draftServer.URL+"/api/listing/shein-records", "chain1-draft-vnext", chain1DraftBody(chain1ProductKey, nextReceipt.CatalogVersion, storeID), chain1Organization)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnprocessableEntity, status)
	status, _, err = chain1RawRequest(draftServer.Client(), http.MethodPost, draftServer.URL+"/api/listing/shein-records", "chain1-draft-save", chain1DraftBody(chain1ProductKey, nextReceipt.CatalogVersion, storeID), chain1Organization)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, status)
	status, _, err = chain1RawRequest(draftServer.Client(), http.MethodPost, draftServer.URL+"/api/listing/shein-records", "chain1-cross-org", draftBody, "org-chain1-foreign")
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, status)

	for table, want := range map[string]int64{
		"product_source_publications":         2,
		"product_source_publication_receipts": 2,
		"product_snapshot_versions":           3,
		"product_approval_receipts":           1,
		"product_approved_assets":             1,
		"workbench_stores":                    1,
		"saas_store_quota_allocations":        1,
		"image_agent_v2_runs":                 1,
		"listing_shein_records":               1,
		"listing_shein_record_operations":     1,
	} {
		var count int64
		require.NoError(t, db.Table(table).Count(&count).Error)
		require.Equal(t, want, count, table)
	}
	require.Equal(t, tableCount, chain1TableCount(t, db), "requests must not perform DDL")
	require.Positive(t, reviewGenerator.calls.Load())
	require.Positive(t, grants.userCalls.Load())
	require.Positive(t, grants.workerCalls.Load())
	require.Positive(t, imagePorts.calls.Load())
	fmt.Printf("CHAIN1_RECEIPT org=%s actor=%s product=%s version=%d publication=%s run=%s plan=1 digest=%s action=%s asset=%s store=%s record=%s diagnostic=%s\n",
		chain1Organization, chain1Actor, chain1ProductKey, appliedVersion, exactProduct.PublicationID, runID, awaiting.ResultDigest, actionID, inventory.Assets[0].ID, storeID, draftReceipt.RecordID, draftReceipt.DiagnosticStatus)
}

func chain1Database(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	var publicTables int64
	require.NoError(t, root.Raw("SELECT count(*) FROM pg_tables WHERE schemaname = 'public'").Row().Scan(&publicTables))
	require.Zero(t, publicTables, "CHAIN-1 PostgreSQL must start empty")
	schema := "chain1_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE SCHEMA "+schema).Error)
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		if raw, e := db.DB(); e == nil {
			_ = raw.Close()
		}
		_ = root.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		if raw, e := root.DB(); e == nil {
			_ = raw.Close()
		}
	})
	return db
}

func chain1InstallSchemas(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, InstallProductReviewSchema(db))
	require.NoError(t, assetstore.AutoMigrate(db))
	require.NoError(t, imagestore.AutoMigrateOrganizationScope(db))
	require.NoError(t, listingsubscription.AutoMigrateRepository(db))
	require.NoError(t, storecenter.AutoMigrateStoreRepository(db))
	require.NoError(t, storecenter.AutoMigrateAuditRepository(db))
	schema, err := os.ReadFile("../listingrecordstore/schema.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(schema)).Error)
}

func chain1TableCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Raw("SELECT count(*) FROM pg_tables WHERE schemaname = current_schema()").Row().Scan(&count))
	return count
}

func chain1LiveWriteContext(t *testing.T, resolver *workbenchcontext.Resolver, verifier chain1TokenVerifier) context.Context {
	t.Helper()
	identity, err := verifier.Verify(context.Background(), chain1UserToken)
	require.NoError(t, err)
	identity, err = resolver.Resolve(context.Background(), httproute.OrganizationAccessPolicyLiveWrite, workbenchcontext.ResolveInput{Identity: identity, BearerToken: chain1UserToken, RequestedOrganizationID: chain1Organization})
	require.NoError(t, err)
	return authidentity.WithAuthenticatedIdentity(context.Background(), identity)
}

func chain1SourceEnvelope(version, title string) sourcing.SourceEnvelope {
	return sourcing.SourceEnvelope{
		Identity:         sourcing.SourceIdentity{SourceType: sourcing.SourceTypeManualImport, SourcePlatform: "controlled-chain1", SourceID: "chain1-source", SourceVersion: version},
		RawReference:     sourcing.RawSourceReference{ReferenceType: "captured", ReferenceID: "chain1-evidence-" + version, SnapshotID: "chain1-snapshot-" + version, Checksum: sourcing.RawSnapshotChecksum(title), CapturedAt: time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)},
		ProductCandidate: sourcing.ProductCandidate{Title: title, Description: "Controlled CHAIN-1 product", Brand: "Chain", CategoryPath: []string{"Home", "Drinkware"}, Variants: []sourcing.ProductVariantCandidate{{SourceID: "variant-1", SKU: "CHAIN-1", Price: 12, Currency: "USD", Stock: 3}}},
		AssetCandidates:  []sourcing.AssetCandidate{{SourceID: "source-image-1", URL: "https://source.example.test/chain1.png", MediaType: "image", Role: "main", Width: 1, Height: 1}},
		Trace:            sourcing.SourceTrace{SourceRunID: "chain1-" + version, RequestID: "chain1-request-" + version, Notes: []string{"controlled external source fixture"}},
	}
}

type chain1TokenVerifier struct{}

func (chain1TokenVerifier) Verify(_ context.Context, token string) (authidentity.AuthenticatedIdentity, error) {
	if token != chain1UserToken {
		return authidentity.AuthenticatedIdentity{}, errors.New("controlled token rejected")
	}
	return authidentity.AuthenticatedIdentity{UserID: chain1Actor, HomeOrganizationID: chain1Organization, TokenExpiresAt: time.Now().Add(time.Hour)}, nil
}

type chain1GrantFixture struct {
	userAllowed, workerAllowed atomic.Bool
	userCalls, workerCalls     atomic.Int32
}

func (fixture *chain1GrantFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/zitadel.authorization.v2.AuthorizationService/ListAuthorizations" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	allowed := false
	switch token {
	case chain1UserToken:
		fixture.userCalls.Add(1)
		allowed = fixture.userAllowed.Load()
	case chain1ServiceToken:
		fixture.workerCalls.Add(1)
		allowed = fixture.workerAllowed.Load()
	}
	w.Header().Set("Content-Type", "application/json")
	if !allowed {
		_, _ = io.WriteString(w, `{"pagination":{},"authorizations":[]}`)
		return
	}
	_, _ = fmt.Fprintf(w, `{"pagination":{"totalResult":"1"},"authorizations":[{"id":"chain1-authorization","project":{"id":%q},"organization":{"id":%q,"name":"CHAIN-1"},"user":{"id":%q},"state":"STATE_ACTIVE","roles":[{"key":"listingkit_admin"}]}]}`, chain1Project, chain1Organization, chain1Actor)
}

type chain1TitleGenerator struct{ calls atomic.Int32 }

func (g *chain1TitleGenerator) Generate(_ context.Context, request enrichment.GenerationRequest) (enrichment.Candidate, error) {
	g.calls.Add(1)
	evidenceID, err := enrichment.CanonicalEvidenceID(request.Source)
	return enrichment.Candidate{Changes: []enrichment.FieldChange{{Field: "title", Value: "Reviewed chain product", EvidenceIDs: []string{evidenceID}}}}, err
}

func chain1HTTPServer(t *testing.T, application *http.Server) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(application.Handler)
	server.Config.ReadTimeout, server.Config.WriteTimeout = application.ReadTimeout, application.WriteTimeout
	server.Start()
	t.Cleanup(server.Close)
	return server
}

func chain1Headers(request *http.Request, idempotencyKey, organizationID string) {
	request.Header.Set("Authorization", "Bearer "+chain1UserToken)
	request.Header.Set("X-Requested-Organization-ID", organizationID)
	request.Header.Set("X-Tenant-ID", "forged")
	request.Header.Set("X-User-ID", "forged")
	request.Header.Set("X-User-Roles", "platform_admin")
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	request.Header.Set("Content-Type", "application/json")
}

func chain1RawRequest(client *http.Client, method, target, key, body, organizationID string) (int, []byte, error) {
	request, err := http.NewRequest(method, target, strings.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	chain1Headers(request, key, organizationID)
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	return response.StatusCode, raw, err
}

func chain1ReviewRequest(t *testing.T, server *httptest.Server, method, path, key, body string, expected int) productReviewViewDTO {
	t.Helper()
	status, raw, err := chain1RawRequest(server.Client(), method, server.URL+path, key, body, chain1Organization)
	require.NoError(t, err)
	require.Equal(t, expected, status, string(raw))
	var view productReviewViewDTO
	if expected == http.StatusOK {
		require.NoError(t, json.Unmarshal(raw, &view))
	}
	return view
}

func chain1Uint(t *testing.T, value string) uint64 {
	t.Helper()
	parsed, err := strconv.ParseUint(value, 10, 64)
	require.NoError(t, err)
	return parsed
}

type chain1ConnectionStatus struct{}

func (chain1ConnectionStatus) Status(context.Context, storecenter.ConnectionStatusInput) (storecenter.ConnectionStatus, error) {
	return storecenter.ConnectionStatusDisconnected, nil
}

func chain1DraftBody(productKey string, version uint64, storeID string) string {
	return fmt.Sprintf(`{"product_key":%q,"snapshot_version":%d,"store_id":%q,"country":"US","language":"en","action":"save_draft"}`, productKey, version, storeID)
}

func chain1ApprovalPublicationKey(actionID, runID string, revision int64) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d", actionID, runID, revision)))
	return "image-agent:approval:" + hex.EncodeToString(digest[:])
}

func chain1TemporalDriver(t *testing.T, address string) *chain1temporal.Driver {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	driver, err := chain1temporal.Connect(ctx, address)
	require.NoError(t, err)
	return driver
}

func chain1StartWorker(t *testing.T, driver *chain1temporal.Driver, activities *imageagenttemporal.Activities) {
	t.Helper()
	require.NoError(t, driver.StartOrganizationWorker(activities))
}

func chain1ImageApplication(t *testing.T, db *gorm.DB, verifier chain1TokenVerifier, resolver *workbenchcontext.Resolver, authorizer *authz.ListingKitAuthorizer, workflows imageagent.WorkflowClient, product catalog.PublishedSnapshot) *http.Server {
	t.Helper()
	application, err := NewImageAgentOrganizationApplication(db, verifier, resolver, authorizer, workflows,
		imageagent.TenantAllowlistStartGate{Enabled: true, AllowedTenantIDs: []string{chain1Organization}},
		[]OrganizationImageBinding{{ContextID: "chain1-context-388", OwnerUserID: chain1Actor, Identity: product.Identity, Version: product.Version, PublicationID: product.PublicationID}},
	)
	require.NoError(t, err)
	return application
}

var chain1PNG, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")

type chain1ImagePorts struct{ calls atomic.Int32 }

func (ports *chain1ImagePorts) Extract(_ context.Context, request productimage.ExtractRequest) (productimage.Candidate, error) {
	ports.calls.Add(1)
	return chain1ImageCandidate(request.Source, productimage.RoleSubject, "extract_subject"), nil
}

func (ports *chain1ImagePorts) RenderWhiteBackground(_ context.Context, request productimage.RenderRequest) (productimage.Candidate, error) {
	ports.calls.Add(1)
	return chain1ImageCandidate(request.Source, productimage.RoleWhiteBackground, "render_white_background"), nil
}

func (ports *chain1ImagePorts) RenderScene(_ context.Context, request productimage.SceneRequest) ([]productimage.Candidate, error) {
	ports.calls.Add(1)
	return []productimage.Candidate{chain1ImageCandidate(request.Source, productimage.RoleScene, "render_scene")}, nil
}
func chain1ImageCandidate(source productimage.Asset, role productimage.Role, operation string) productimage.Candidate {
	return productimage.Candidate{Asset: productimage.Asset{Bytes: append([]byte(nil), chain1PNG...), MediaType: "image/png", SourceURL: source.URL, SourceAssetID: source.SourceAssetID, Role: role, Width: 1, Height: 1, Operations: []string{operation}}, Metadata: productimage.GenerationMetadata{Capability: operation, ModelFamily: "controlled", InvocationID: "chain1-" + operation, PromptReference: "chain1", PromptVersion: "v1"}}
}

func (ports *chain1ImagePorts) Review(context.Context, productimage.ReviewRequest) (productimage.Review, error) {
	ports.calls.Add(1)
	return productimage.Review{Score: 1}, nil
}

func (ports *chain1ImagePorts) QuoteUsage(_ context.Context, request productimage.UsageQuoteRequest) (productimage.UsageQuote, error) {
	ports.calls.Add(1)
	return productimage.UsageQuote{Operation: request.Operation, Provider: "controlled", RouteReference: "chain1-route", Model: "chain1-model", CredentialReference: "chain1-credential", ConfigurationVersion: "v1", PricingVersion: "chain1-pricing-v1", Fingerprint: request.Operation + "-chain1", MaximumOutputs: request.MaximumOutputs, MaximumModelCalls: 1, CostUpperBoundKnown: true}, nil
}

func (ports *chain1ImagePorts) Resolve(input imagepolicy.ProfileInput) (imagepolicy.ProductImageProfile, error) {
	ports.calls.Add(1)
	return imagepolicy.ProductImageProfile{Key: imagepolicy.PolicyKey(input), PolicyVersion: "chain1-policy-v1", SceneDefaults: productimage.SceneOptions{SceneCategory: input.SceneCategory, SceneStyle: "studio"}}, nil
}

type chain1StoredObject struct {
	data                  []byte
	contentType, checksum string
	metadata              map[string]string
}
type chain1ObjectStore struct {
	mu      sync.Mutex
	objects map[string]chain1StoredObject
}

func newChain1ObjectStore() *chain1ObjectStore {
	return &chain1ObjectStore{objects: map[string]chain1StoredObject{}}
}
func (*chain1ObjectStore) PublicURL(key string) string {
	return "https://cdn.example.test/" + strings.TrimLeft(key, "/")
}
func (store *chain1ObjectStore) PutImmutable(_ context.Context, input objectstore.ImmutableObjectPut) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if existing, ok := store.objects[input.Key]; ok {
		if existing.checksum == input.SHA256 && bytes.Equal(existing.data, input.Data) {
			return nil
		}
		return objectstore.ErrObjectConflict
	}
	store.objects[input.Key] = chain1StoredObject{data: append([]byte(nil), input.Data...), contentType: input.ContentType, checksum: input.SHA256, metadata: map[string]string{"sha256": input.SHA256, "size-bytes": strconv.FormatInt(input.SizeBytes, 10)}}
	return nil
}
func (store *chain1ObjectStore) InspectObject(_ context.Context, key string) (objectstore.ObjectInspection, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	object, ok := store.objects[key]
	if !ok {
		return objectstore.ObjectInspection{}, nil
	}
	return objectstore.ObjectInspection{Exists: true, ContentLength: int64(len(object.data)), ContentType: object.contentType, Metadata: chain1CloneMetadata(object.metadata)}, nil
}
func (store *chain1ObjectStore) ReadObject(_ context.Context, key string, maximum int64) ([]byte, objectstore.ObjectInspection, error) {
	inspection, _ := store.InspectObject(context.Background(), key)
	store.mu.Lock()
	defer store.mu.Unlock()
	object, ok := store.objects[key]
	if !ok {
		return nil, inspection, nil
	}
	if int64(len(object.data)) > maximum {
		return nil, inspection, objectstore.ErrArtifactUnavailable
	}
	return append([]byte(nil), object.data...), inspection, nil
}
func (store *chain1ObjectStore) CopyImmutable(_ context.Context, input objectstore.ImmutableObjectCopy) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	source, ok := store.objects[input.SourceKey]
	if !ok {
		return objectstore.ErrArtifactUnavailable
	}
	if existing, ok := store.objects[input.Destination.Key]; ok {
		if existing.checksum == input.Destination.SHA256 && bytes.Equal(existing.data, source.data) {
			return nil
		}
		return objectstore.ErrObjectConflict
	}
	source.contentType, source.checksum = input.Destination.ContentType, input.Destination.SHA256
	source.metadata = map[string]string{"sha256": input.Destination.SHA256, "size-bytes": strconv.FormatInt(input.Destination.SizeBytes, 10)}
	store.objects[input.Destination.Key] = source
	return nil
}
func (store *chain1ObjectStore) countPrefix(prefix string) int {
	store.mu.Lock()
	defer store.mu.Unlock()
	count := 0
	for key := range store.objects {
		if strings.HasPrefix(key, prefix) {
			count++
		}
	}
	return count
}
func chain1CloneMetadata(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

type chain1RunView struct {
	Run struct {
		Status imageagent.RunStatus `json:"status"`
	} `json:"run"`
	Plan struct {
		Revision int64 `json:"revision"`
	} `json:"plan"`
	ResultDigest string `json:"result_digest"`
}

func chain1WaitRun(t *testing.T, server *httptest.Server, runID string, want imageagent.RunStatus, timeout time.Duration) chain1RunView {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last chain1RunView
	for time.Now().Before(deadline) {
		status, raw, err := chain1RawRequest(server.Client(), http.MethodGet, server.URL+"/api/organization/image-agent/runs/"+runID, "", "", chain1Organization)
		if err == nil && status == http.StatusOK && json.Unmarshal(raw, &last) == nil && last.Run.Status == want {
			return last
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("run %s did not reach %s; last=%+v", runID, want, last)
	return last
}

func chain1WaitInventory(t *testing.T, repository productasset.Repository, version uint64, timeout time.Duration) productasset.ApprovedAssetInventory {
	t.Helper()
	deadline := time.Now().Add(timeout)
	scope := productasset.InventoryScope{TenantID: chain1Organization, ProductKey: chain1ProductKey, TargetPlatform: "shein", SourceSnapshotVersion: version}
	for time.Now().Before(deadline) {
		if inventory, err := repository.GetApprovedInventory(context.Background(), scope); err == nil {
			return inventory
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("exact approved inventory did not become ready")
	return productasset.ApprovedAssetInventory{}
}

func chain1WaitCounter(t *testing.T, counter *atomic.Int32, minimum int32, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if counter.Load() >= minimum {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("counter did not reach %d; got %d", minimum, counter.Load())
}

func chain1RestartTemporal(t *testing.T) {
	t.Helper()
	compose, project := os.Getenv("CHAIN1_COMPOSE_FILE"), os.Getenv("CHAIN1_COMPOSE_PROJECT")
	require.NotEmpty(t, compose)
	require.NotEmpty(t, project)
	command := exec.Command("docker", "compose", "-p", project, "-f", compose, "restart", "chain1-temporal")
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
}
