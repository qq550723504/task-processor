package tests

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/assetpublication"
	a1688 "task-processor/internal/integration/crawler/a1688"
	assetdb "task-processor/internal/integration/persistence/product/asset"
	catalogdb "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/listing/preview"
	"task-processor/internal/listing/readiness"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

// These are controlled domain contracts, not an HTTP import or production
// cutover: source-account authorization is deliberately absent from this fixture.
func issue30Envelope() sourcing.SourceEnvelope {
	return a1688.Alibaba1688SourceEnvelope(a1688.Alibaba1688SourceEnvelopeInput{
		Request:     a1688.Alibaba1688CrawlRequestInput{URL: "https://detail.1688.com/offer/123.html"},
		Product:     &a1688.Alibaba1688ProductSnapshot{ID: "123", URL: "https://detail.1688.com/offer/123.html", Title: "Bottle", Brand: "Source brand", MinPrice: 10, Currency: "CNY", Category: "Home > Drinkware", Images: []string{"https://source.example.test/bottle.jpg"}, CrawledAt: time.Date(2026, 9, 5, 1, 2, 3, 0, time.UTC)},
		RawSnapshot: `{"offerId":"123","title":"Bottle"}`, SourceRunID: "source-run-1", RequestID: "request-1",
	})
}

func issue30OpenDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
func issue30Publisher(t *testing.T, db *gorm.DB) (*sourcing.Publisher, catalog.Repository) {
	t.Helper()
	repo, err := catalogdb.NewRepository(db)
	require.NoError(t, err)
	catalogPublisher, err := catalog.NewPublisher(repo)
	require.NoError(t, err)
	publisher, err := sourcing.NewPublisher(catalogPublisher)
	require.NoError(t, err)
	return publisher, repo
}
func issue30Request(t *testing.T, envelope sourcing.SourceEnvelope) sourcing.PublishRequest {
	t.Helper()
	key, id, err := sourcing.PublicationIdentity(envelope)
	require.NoError(t, err)
	return sourcing.PublishRequest{TenantID: "org-a", ProductKey: key, PublicationID: id, Envelope: envelope}
}

func TestIssue30CatalogReplayConflictAndDurableLineage(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "catalog.db")
	db := issue30OpenDB(t, path)
	require.NoError(t, catalogdb.AutoMigrate(db))
	publisher, _ := issue30Publisher(t, db)
	envelope := issue30Envelope()
	envelope.Trace.Notes = []string{"controlled fixture; no live crawl"}
	envelope.Warnings = append(envelope.Warnings, sourcing.SourceWarning{Code: "missing_dimensions", Field: "dimensions", Message: "dimensions not provided"})
	request := issue30Request(t, envelope)
	first, err := publisher.Publish(ctx, request)
	require.NoError(t, err)
	require.Equal(t, uint64(1), first.Version)
	// Lose the first response and reopen all repository state before replay.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	reopened := issue30OpenDB(t, path)
	publisher, repo := issue30Publisher(t, reopened)
	replay, err := publisher.Publish(ctx, request)
	require.NoError(t, err)
	require.Equal(t, first, replay)
	persisted, err := repo.GetCurrentSnapshot(ctx, first.Identity)
	require.NoError(t, err)
	require.Equal(t, first, persisted)
	source := persisted.Snapshot.Sources[0]
	require.Equal(t, envelope.Identity.SourceID, source.SourceID)
	require.Equal(t, "1688", source.Platform)
	require.Equal(t, envelope.RawReference.ReferenceID, source.Detail)
	require.Equal(t, envelope.RawReference.ReferenceType, source.ReferenceType)
	require.Equal(t, envelope.RawReference.SnapshotID, source.SnapshotID)
	require.Equal(t, envelope.Identity.SourceVersion, source.SourceVersion)
	require.Equal(t, envelope.RawReference.URL, source.URL)
	require.Equal(t, envelope.RawReference.Checksum, source.Checksum)
	require.Equal(t, envelope.RawReference.CapturedAt, source.CapturedAt)
	require.Equal(t, envelope.RawReference.Metadata, source.Metadata)
	require.Equal(t, "source-run-1", source.SourceRunID)
	require.Equal(t, "request-1", source.RequestID)
	require.Equal(t, envelope.Trace.Notes, source.Notes)
	require.True(t, persisted.Snapshot.Review.NeedsReview)
	require.Contains(t, persisted.Snapshot.Warnings, catalog.Warning{Code: "missing_dimensions", Field: "dimensions", Message: "dimensions not provided"})
	// Same run, different payload must conflict, never overwrite or append.
	changed := envelope
	changed.ProductCandidate.Title = "Changed"
	changedRequest := issue30Request(t, changed)
	require.Equal(t, request.PublicationID, changedRequest.PublicationID)
	_, err = publisher.Publish(ctx, changedRequest)
	require.ErrorIs(t, err, catalog.ErrPublicationConflict)
	after, err := repo.GetCurrentSnapshot(ctx, first.Identity)
	require.NoError(t, err)
	require.Equal(t, first, after)
	// Changed durable lineage under the same explicit run is also a conflict.
	changed = envelope
	changed.Trace.RequestID = "different-request"
	_, err = publisher.Publish(ctx, issue30Request(t, changed))
	require.ErrorIs(t, err, catalog.ErrPublicationConflict)
	// Organization identity scopes both replay and current reads.
	_, err = repo.GetCurrentSnapshot(ctx, catalog.SnapshotIdentity{TenantID: "org-b", ProductKey: request.ProductKey})
	require.ErrorIs(t, err, catalog.ErrSnapshotNotReady)
	other := request
	other.TenantID = "org-b"
	otherResult, err := publisher.Publish(ctx, other)
	require.NoError(t, err)
	require.Equal(t, uint64(1), otherResult.Version)
	// Concurrent replays do not allocate new versions.
	var group sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			got, err := publisher.Publish(ctx, request)
			if err == nil && got.Version != 1 {
				err = fmt.Errorf("replay version %d", got.Version)
			}
			errs <- err
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = publisher.Publish(canceled, request)
	require.ErrorIs(t, err, context.Canceled)
}

func TestIssue30ContentPublicationPreservesProductAcrossSourceVersions(t *testing.T) {
	db := issue30OpenDB(t, filepath.Join(t.TempDir(), "content.db"))
	require.NoError(t, catalogdb.AutoMigrate(db))
	publisher, repository := issue30Publisher(t, db)
	envelope := issue30Envelope()
	envelope.Trace.SourceRunID = ""
	envelope.Identity.SourceVersion = "v1"
	firstRequest := issue30Request(t, envelope)
	first, err := publisher.Publish(context.Background(), firstRequest)
	require.NoError(t, err)
	envelope.Identity.SourceVersion = "v2"
	envelope.RawReference.ReferenceID = "evidence-v2"
	secondRequest := issue30Request(t, envelope)
	require.Equal(t, firstRequest.ProductKey, secondRequest.ProductKey)
	require.NotEqual(t, firstRequest.PublicationID, secondRequest.PublicationID)
	second, err := publisher.Publish(context.Background(), secondRequest)
	require.NoError(t, err)
	require.Equal(t, first.Version+1, second.Version)
	historical, err := repository.(catalog.VersionedSnapshotReader).GetSnapshot(context.Background(), first.Identity, first.Version)
	require.NoError(t, err)
	require.Equal(t, first, historical)
}

type issue30Projection struct{ value imageagent.RunProjection }

func (p issue30Projection) GetProjection(_ context.Context, scope imageagent.RunScope) (imageagent.RunProjection, error) {
	if scope.TenantID != p.value.Run.TenantID || scope.OwnerUserID != p.value.Run.UserID || scope.RunID != p.value.Run.ID {
		return imageagent.RunProjection{}, imageagent.ErrValidation
	}
	return p.value, nil
}

type issue30PublicURLs struct{}

func (issue30PublicURLs) PublicURL(key string) string { return "https://approved.example.test/" + key }

func TestIssue30ProductApprovalAndReadinessUseDurableCurrentOwners(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "product-assets.db")
	db := issue30OpenDB(t, path)
	require.NoError(t, catalogdb.AutoMigrate(db))
	require.NoError(t, assetdb.AutoMigrate(db))
	publisher, _ := issue30Publisher(t, db)
	envelope := issue30Envelope()
	// The fixture supplies every fact required by the current adapter; do not
	// clear source warnings to manufacture readiness.
	require.Empty(t, envelope.Warnings)
	published, err := publisher.Publish(ctx, issue30Request(t, envelope))
	require.NoError(t, err)
	assets, err := assetdb.NewRepository(db)
	require.NoError(t, err)
	scope := asset.InventoryScope{TenantID: published.Identity.TenantID, ProductKey: published.Identity.ProductKey, TargetPlatform: "shein", SourceSnapshotVersion: published.Version}
	_, err = assets.GetApprovedInventory(ctx, scope)
	require.ErrorIs(t, err, asset.ErrApprovedAssetsNotReady)
	require.False(t, readiness.ProductInputs(published, nil, "shein").Ready)
	require.NotEmpty(t, published.Snapshot.Images)
	owner, err := imageagent.ArtifactOwnerKey("actor")
	require.NoError(t, err)
	hash := strings.Repeat("1", 64)
	slot := imageagent.Slot{ID: "main-1", Role: imageagent.SlotRoleMain}
	projection := imageagent.RunProjection{
		Run:          imageagent.Run{ID: "image-run-1", TenantID: "org-a", UserID: "actor", TargetPlatform: "shein", Status: imageagent.RunStatusAwaitingFinalApproval, ActivePlanRevision: 1},
		Plan:         imageagent.Plan{Revision: 1, Slots: []imageagent.Slot{slot}},
		AssetCatalog: imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: published.Identity.ProductKey, SourceSnapshotVersion: published.Version}},
		Slots:        []imageagent.SlotProjection{{Slot: imageagent.Slot{ID: slot.ID, Role: slot.Role, Status: imageagent.SlotStatusAccepted}, Attempt: 1, Candidates: []imageagent.AssetCandidate{{AssetID: "approved-1", SourceAssetID: envelope.AssetCandidates[0].SourceID, Width: 1200, Height: 1200, DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: fmt.Sprintf("image-agent/public/org-a/%s/image-run-1/1/main-1/1/0-%s.png", owner, hash), SHA256: hash}}}}},
	}
	projection.ResultDigest, err = imageagent.ResultDigestV3(projection.Plan, projection.Slots)
	require.NoError(t, err)
	approval, err := assetpublication.NewPublisher(issue30Projection{projection}, assets, issue30PublicURLs{})
	require.NoError(t, err)
	input := imageagent.PublishApprovedV3Input{RunID: projection.Run.ID, TenantID: "org-a", UserID: "actor", PlanRevision: 1, CandidateAssetIDs: []string{"approved-1"}, IdempotencyKey: "explicit-approval-1"}
	invalid := input
	invalid.UserID = "other"
	_, err = approval.PublishApprovedV3(ctx, invalid)
	require.Error(t, err)
	_, err = assets.GetApprovedInventory(ctx, scope)
	require.ErrorIs(t, err, asset.ErrApprovedAssetsNotReady)
	ack, err := approval.PublishApprovedV3(ctx, input)
	require.NoError(t, err)
	replay, err := approval.PublishApprovedV3(ctx, input)
	require.NoError(t, err)
	require.Equal(t, ack, replay)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	reopened := issue30OpenDB(t, path)
	assets, err = assetdb.NewRepository(reopened)
	require.NoError(t, err)
	_, catalogRepo := issue30Publisher(t, reopened)
	persisted, err := catalogRepo.GetCurrentSnapshot(ctx, published.Identity)
	require.NoError(t, err)
	inventory, err := assets.GetApprovedInventory(ctx, scope)
	require.NoError(t, err)
	require.Len(t, inventory.Assets, 1)
	require.Equal(t, envelope.AssetCandidates[0].SourceID, inventory.Assets[0].SourceAssetID)
	require.NotEqual(t, persisted.Snapshot.Images[0].URL, inventory.Assets[0].URL)
	require.True(t, readiness.ProductInputs(persisted, &inventory, "shein").Ready)
	attachment := preview.BuildAttachment(preview.AttachmentInput{CatalogProduct: &persisted.Snapshot, ApprovedAssetInventory: &inventory})
	require.Equal(t, persisted.Snapshot.Sources, attachment.CatalogProduct.Sources)
	require.Equal(t, inventory.Assets, attachment.ApprovedAssetInventory.Assets)
	for _, other := range []asset.InventoryScope{
		{TenantID: "org-b", ProductKey: scope.ProductKey, TargetPlatform: "shein", SourceSnapshotVersion: 1},
		{TenantID: "org-a", ProductKey: "other-product", TargetPlatform: "shein", SourceSnapshotVersion: 1},
		{TenantID: "org-a", ProductKey: scope.ProductKey, TargetPlatform: "amazon", SourceSnapshotVersion: 1},
		{TenantID: "org-a", ProductKey: scope.ProductKey, TargetPlatform: "shein", SourceSnapshotVersion: 2},
	} {
		_, err = assets.GetApprovedInventory(ctx, other)
		require.ErrorIs(t, err, asset.ErrApprovedAssetsNotReady)
	}
	// Approved images do not conceal later missing source facts.
	persisted.Snapshot.Review = &catalog.ReviewState{NeedsReview: true, Reasons: []string{"missing dimensions"}}
	require.False(t, readiness.ProductInputs(persisted, &inventory, "shein").Ready)
}
