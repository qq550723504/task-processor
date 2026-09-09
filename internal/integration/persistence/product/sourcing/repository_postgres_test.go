package sourcingpersistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	catalogpersistence "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

func postgresFixture(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("ISSUE378_TEST_DSN")
	if dsn == "" {
		t.Skip("result=SKIP: ISSUE378_TEST_DSN must target task-isolated PostgreSQL")
	}
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := "issue378_" + strings.ReplaceAll(uuid.NewString(), "-", "_")
	require.NoError(t, root.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() { require.NoError(t, root.Exec("DROP SCHEMA "+schema+" CASCADE").Error) })
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		if err := catalogpersistence.AutoMigrate(tx); err != nil {
			return err
		}
		return InstallSchema(tx)
	}))
	return db
}

type testCatalogBridge struct{ db *gorm.DB }

func testCatalogBridgeFactory(db *gorm.DB) (CatalogBridge, error) { return &testCatalogBridge{db}, nil }

func (b *testCatalogBridge) Publish(ctx context.Context, publication sourcing.AtomicPublication) (CatalogBinding, error) {
	writer, err := catalogpersistence.NewTransactionWriter(b.db)
	if err != nil {
		return CatalogBinding{}, err
	}
	publisher, err := catalog.NewPublisher(writer)
	if err != nil {
		return CatalogBinding{}, err
	}
	published, err := publisher.Publish(ctx, catalog.PublishRequest{
		Identity:            catalog.SnapshotIdentity{TenantID: publication.OrganizationID, ProductKey: publication.ProductKey},
		ExpectedBaseVersion: publication.ExpectedBaseVersion, PublicationID: publication.PublicationID, Snapshot: publication.Snapshot,
	})
	if err != nil {
		return CatalogBinding{}, err
	}
	raw, err := json.Marshal(published.Snapshot)
	return CatalogBinding{Version: published.Version, PublicationID: published.PublicationID, SnapshotJSON: raw}, err
}

func (b *testCatalogBridge) Read(ctx context.Context, organizationID, productKey string, version uint64) (CatalogBinding, bool, error) {
	reader, err := catalogpersistence.NewBoundedSnapshotReader(b.db, sourcing.MaxEncodedSnapshotBytes)
	if err != nil {
		return CatalogBinding{}, false, err
	}
	published, err := reader.GetSnapshot(ctx, catalog.SnapshotIdentity{TenantID: organizationID, ProductKey: productKey}, version)
	if errors.Is(err, catalog.ErrSnapshotNotReady) {
		return CatalogBinding{}, false, nil
	}
	if err != nil {
		return CatalogBinding{}, false, err
	}
	raw, err := json.Marshal(published.Snapshot)
	return CatalogBinding{Version: published.Version, PublicationID: published.PublicationID, SnapshotJSON: raw}, true, err
}

func (b *testCatalogBridge) LockPublicationSlot(ctx context.Context, organizationID, productKey, publicationID string) (bool, error) {
	return catalogpersistence.LockPublicationSlot(ctx, b.db, catalog.SnapshotIdentity{TenantID: organizationID, ProductKey: productKey}, publicationID)
}

func atomicFixture(t *testing.T, publicationID, productKey string, expected *uint64) sourcing.AtomicPublication {
	t.Helper()
	envelope, err := sourcing.NormalizePublicationEnvelope(sourcing.SourceEnvelope{
		Identity:         sourcing.SourceIdentity{SourceType: sourcing.SourceTypeManualImport, SourcePlatform: "controlled", SourceID: "source-1", SourceVersion: "v1"},
		RawReference:     sourcing.RawSourceReference{ReferenceType: "snapshot", ReferenceID: "raw-1", Checksum: "sha256:abc"},
		ProductCandidate: sourcing.ProductCandidate{Title: "Bottle", Attributes: map[string]string{"color": "blue"}},
		Warnings:         []sourcing.SourceWarning{{Code: "missing_weight", Field: "weight", Message: "unavailable"}},
		MissingFacts:     []sourcing.MissingFact{{Field: "weight", Reason: "source omitted"}},
		Trace:            sourcing.SourceTrace{SourceRunID: "run-1", RequestID: "request-1"},
	})
	require.NoError(t, err)
	snapshot, err := sourcing.ToSnapshot(envelope)
	require.NoError(t, err)
	envelopeJSON, err := json.Marshal(envelope)
	require.NoError(t, err)
	snapshotJSON, err := json.Marshal(snapshot)
	require.NoError(t, err)
	producer := sourcing.ProducerDescriptor{Kind: "controlled_snapshot", Version: "v1"}
	inputHash, err := sourcing.CanonicalPublicationInputHash(producer, envelope, envelopeJSON, productKey, expected)
	require.NoError(t, err)
	return sourcing.AtomicPublication{
		OrganizationID: "org-a", ActorID: "actor-a", PublicationID: publicationID,
		InputHash: inputHash, Producer: producer, ProductKey: productKey, ExpectedBaseVersion: expected,
		Envelope: envelope, EnvelopeJSON: envelopeJSON, Snapshot: snapshot, SnapshotJSON: snapshotJSON,
	}
}

func TestPostgresAtomicPublicationReplayReadRestartAndIsolation(t *testing.T) {
	db := postgresFixture(t)
	store, err := NewRepository(db, testCatalogBridgeFactory)
	require.NoError(t, err)
	publication := atomicFixture(t, "pub-1", "product-1", uint64Pointer(0))

	receipt, err := store.Publish(context.Background(), publication)
	require.NoError(t, err)
	require.Equal(t, uint64(1), receipt.CatalogVersion)
	require.Equal(t, "pub-1", receipt.CatalogPublicationID)

	replayed, err := store.Publish(context.Background(), publication)
	require.NoError(t, err)
	require.Equal(t, receipt, replayed)
	verified, err := store.Verify(context.Background(), publication)
	require.NoError(t, err)
	require.Equal(t, receipt, verified)

	restarted, err := NewRepository(db, testCatalogBridgeFactory)
	require.NoError(t, err)
	persisted, err := restarted.Read(context.Background(), "org-a", "pub-1")
	require.NoError(t, err)
	require.Equal(t, "missing_weight", persisted.Envelope.Warnings[0].Code)
	require.Equal(t, "weight", persisted.Envelope.MissingFacts[0].Field)
	require.Equal(t, receipt.CatalogVersion, persisted.Receipt.CatalogVersion)
	require.Equal(t, "Bottle", persisted.Snapshot.Title)

	crossOrg := publication
	crossOrg.OrganizationID = "org-b"
	_, err = restarted.Verify(context.Background(), crossOrg)
	require.ErrorIs(t, err, sourcing.ErrSourcePublicationNotFound)
	_, err = restarted.Read(context.Background(), "org-b", "pub-1")
	require.ErrorIs(t, err, sourcing.ErrSourcePublicationNotFound)

	different := atomicFixture(t, "pub-1", "product-1", uint64Pointer(0))
	different.Envelope.ProductCandidate.Title = "Cup"
	different.Snapshot.Title = "Cup"
	different.EnvelopeJSON, _ = json.Marshal(different.Envelope)
	different.SnapshotJSON, _ = json.Marshal(different.Snapshot)
	different.InputHash, _ = sourcing.CanonicalPublicationInputHash(different.Producer, different.Envelope, different.EnvelopeJSON, different.ProductKey, different.ExpectedBaseVersion)
	_, err = restarted.Publish(context.Background(), different)
	require.ErrorIs(t, err, catalog.ErrPublicationConflict)

	assertCounts(t, db, 1, 1, 1, 1)
}

func TestPostgresAtomicPublicationConcurrencyAndExpectedVersionRace(t *testing.T) {
	db := postgresFixture(t)
	store, err := NewRepository(db, testCatalogBridgeFactory)
	require.NoError(t, err)
	publication := atomicFixture(t, "pub-concurrent", "product-concurrent", uint64Pointer(0))

	const workers = 12
	results := make(chan sourcing.PublicationReceipt, workers)
	errorsFound := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			receipt, publishErr := store.Publish(context.Background(), publication)
			results <- receipt
			errorsFound <- publishErr
		}()
	}
	wg.Wait()
	close(results)
	close(errorsFound)
	for publishErr := range errorsFound {
		require.NoError(t, publishErr)
	}
	for receipt := range results {
		require.Equal(t, uint64(1), receipt.CatalogVersion)
	}

	left := atomicFixture(t, "pub-race-left", "product-concurrent", uint64Pointer(1))
	right := atomicFixture(t, "pub-race-right", "product-concurrent", uint64Pointer(1))
	right.Envelope.ProductCandidate.Title = "Cup"
	right.Snapshot.Title = "Cup"
	right.EnvelopeJSON, _ = json.Marshal(right.Envelope)
	right.SnapshotJSON, _ = json.Marshal(right.Snapshot)
	right.InputHash, _ = sourcing.CanonicalPublicationInputHash(right.Producer, right.Envelope, right.EnvelopeJSON, right.ProductKey, right.ExpectedBaseVersion)
	errCh := make(chan error, 2)
	go func() { _, e := store.Publish(context.Background(), left); errCh <- e }()
	go func() { _, e := store.Publish(context.Background(), right); errCh <- e }()
	first, second := <-errCh, <-errCh
	require.True(t, first == nil && errors.Is(second, catalog.ErrStaleSnapshot) || second == nil && errors.Is(first, catalog.ErrStaleSnapshot), "errors: %v / %v", first, second)

	var versions, evidence, receipts int64
	require.NoError(t, db.Model(&catalogpersistence.SnapshotVersionRecord{}).Count(&versions).Error)
	require.NoError(t, db.Model(&publicationRecord{}).Count(&evidence).Error)
	require.NoError(t, db.Model(&receiptRecord{}).Count(&receipts).Error)
	require.Equal(t, int64(2), versions)
	require.Equal(t, int64(2), evidence)
	require.Equal(t, int64(2), receipts)
}

func TestPostgresAtomicPublicationFaultsRollbackAndUnknownCommitReadback(t *testing.T) {
	for _, stage := range []string{"before_catalog", "after_catalog", "after_evidence", "before_commit", "after_commit"} {
		t.Run(stage, func(t *testing.T) {
			db := postgresFixture(t)
			fault := errors.New("injected " + stage)
			store := &repository{db: db, catalog: testCatalogBridgeFactory, now: time.Now, fault: func(got string) error {
				if got == stage {
					return fault
				}
				return nil
			}}
			publication := atomicFixture(t, "pub-fault", "product-fault", uint64Pointer(0))
			_, err := store.Publish(context.Background(), publication)
			if stage == "after_commit" {
				require.ErrorIs(t, err, sourcing.ErrSourcePublicationOutcomeUnknown)
				store.fault = nil
				receipt, verifyErr := store.Verify(context.Background(), publication)
				require.NoError(t, verifyErr)
				require.Equal(t, uint64(1), receipt.CatalogVersion)
				assertCounts(t, db, 1, 1, 1, 1)
				return
			}
			require.ErrorIs(t, err, sourcing.ErrSourcePublicationUnavailable)
			assertCounts(t, db, 0, 0, 0, 0)
		})
	}
}

func TestPostgresAtomicPublicationCancelAndCorruptFacts(t *testing.T) {
	db := postgresFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	store := &repository{db: db, catalog: testCatalogBridgeFactory, now: time.Now, fault: func(stage string) error {
		if stage == "after_catalog" {
			cancel()
			return ctx.Err()
		}
		return nil
	}}
	publication := atomicFixture(t, "pub-cancel", "product-cancel", uint64Pointer(0))
	_, err := store.Publish(ctx, publication)
	require.ErrorIs(t, err, context.Canceled)
	assertCounts(t, db, 0, 0, 0, 0)

	store.fault = nil
	publication = atomicFixture(t, "pub-corrupt", "product-corrupt", uint64Pointer(0))
	_, err = store.Publish(context.Background(), publication)
	require.NoError(t, err)
	require.NoError(t, db.Model(&receiptRecord{}).Where("organization_id = ? AND publication_id = ?", "org-a", "pub-corrupt").Update("snapshot_hash", fmt.Sprintf("%064d", 1)).Error)
	_, err = store.Verify(context.Background(), publication)
	require.ErrorIs(t, err, sourcing.ErrSourcePublicationStateInvalid)

	require.NoError(t, db.Where("organization_id = ? AND publication_id = ?", "org-a", "pub-corrupt").Delete(&receiptRecord{}).Error)
	_, err = store.Verify(context.Background(), publication)
	require.ErrorIs(t, err, sourcing.ErrSourcePublicationStateInvalid)

	missingEvidence := atomicFixture(t, "pub-missing-evidence", "product-missing-evidence", uint64Pointer(0))
	_, err = store.Publish(context.Background(), missingEvidence)
	require.NoError(t, err)
	require.NoError(t, db.Where("organization_id = ? AND publication_id = ?", "org-a", "pub-missing-evidence").Delete(&publicationRecord{}).Error)
	_, err = store.Verify(context.Background(), missingEvidence)
	require.ErrorIs(t, err, sourcing.ErrSourcePublicationStateInvalid)

	corruptHash := atomicFixture(t, "pub-corrupt-hash", "product-corrupt-hash", uint64Pointer(0))
	_, err = store.Publish(context.Background(), corruptHash)
	require.NoError(t, err)
	for _, model := range []any{&publicationRecord{}, &receiptRecord{}} {
		require.NoError(t, db.Model(model).Where("organization_id = ? AND publication_id = ?", "org-a", "pub-corrupt-hash").Update("input_hash", strings.Repeat("0", 64)).Error)
	}
	_, err = store.Read(context.Background(), "org-a", "pub-corrupt-hash")
	require.ErrorIs(t, err, sourcing.ErrSourcePublicationStateInvalid)

	actorMismatch := atomicFixture(t, "pub-actor-mismatch", "product-actor-mismatch", uint64Pointer(0))
	_, err = store.Publish(context.Background(), actorMismatch)
	require.NoError(t, err)
	require.NoError(t, db.Model(&receiptRecord{}).Where("organization_id = ? AND publication_id = ?", "org-a", "pub-actor-mismatch").Update("actor_id", "actor-b").Error)
	_, err = store.Read(context.Background(), "org-a", "pub-actor-mismatch")
	require.ErrorIs(t, err, sourcing.ErrSourcePublicationStateInvalid)

	legacyIdentity := atomicFixture(t, "pub-legacy-identity", "product-legacy-identity", uint64Pointer(0))
	_, err = store.Publish(context.Background(), legacyIdentity)
	require.NoError(t, err)
	legacyEnvelope := legacyIdentity.Envelope
	legacyEnvelope.Identity.Platform = "legacy-platform"
	legacyEnvelopeJSON, err := json.Marshal(legacyEnvelope)
	require.NoError(t, err)
	legacyInputHash, err := sourcing.CanonicalPublicationInputHash(
		legacyIdentity.Producer, legacyEnvelope, legacyEnvelopeJSON,
		legacyIdentity.ProductKey, legacyIdentity.ExpectedBaseVersion,
	)
	require.NoError(t, err)
	legacyEnvelopeHash := digest(legacyEnvelopeJSON)
	for _, model := range []any{&publicationRecord{}, &receiptRecord{}} {
		require.NoError(t, db.Model(model).
			Where("organization_id = ? AND publication_id = ?", "org-a", "pub-legacy-identity").
			Updates(map[string]any{"input_hash": legacyInputHash, "envelope_hash": legacyEnvelopeHash}).Error)
	}
	require.NoError(t, db.Model(&publicationRecord{}).
		Where("organization_id = ? AND publication_id = ?", "org-a", "pub-legacy-identity").
		Update("envelope_json", legacyEnvelopeJSON).Error)
	_, err = store.Read(context.Background(), "org-a", "pub-legacy-identity")
	require.ErrorIs(t, err, sourcing.ErrSourcePublicationStateInvalid)
}

func TestPostgresAtomicPublicationRejectsOrphanCatalogAndStaleBaseWithoutEvidence(t *testing.T) {
	db := postgresFixture(t)
	store, err := NewRepository(db, testCatalogBridgeFactory)
	require.NoError(t, err)
	orphan := atomicFixture(t, "pub-orphan", "product-orphan", uint64Pointer(0))
	catalogStore, err := catalogpersistence.NewRepository(db)
	require.NoError(t, err)
	publisher, err := catalog.NewPublisher(catalogStore)
	require.NoError(t, err)
	_, err = publisher.Publish(context.Background(), catalog.PublishRequest{
		Identity:      catalog.SnapshotIdentity{TenantID: orphan.OrganizationID, ProductKey: orphan.ProductKey},
		PublicationID: orphan.PublicationID, ExpectedBaseVersion: orphan.ExpectedBaseVersion, Snapshot: orphan.Snapshot,
	})
	require.NoError(t, err)
	_, err = store.Publish(context.Background(), orphan)
	require.ErrorIs(t, err, sourcing.ErrSourcePublicationStateInvalid)

	stale := atomicFixture(t, "pub-stale", "product-orphan", uint64Pointer(0))
	_, err = store.Publish(context.Background(), stale)
	require.ErrorIs(t, err, catalog.ErrStaleSnapshot)
	var evidence, receipts int64
	require.NoError(t, db.Model(&publicationRecord{}).Count(&evidence).Error)
	require.NoError(t, db.Model(&receiptRecord{}).Count(&receipts).Error)
	require.Zero(t, evidence)
	require.Zero(t, receipts)
}

func TestPostgresAtomicPublicationDoesNotAdoptConcurrentOrphanCatalogPublication(t *testing.T) {
	db := postgresFixture(t)
	reachedCatalogBoundary := make(chan struct{})
	releaseCatalogBoundary := make(chan struct{})
	store := &repository{db: db, catalog: testCatalogBridgeFactory, now: time.Now, fault: func(stage string) error {
		if stage == "before_catalog" {
			close(reachedCatalogBoundary)
			<-releaseCatalogBoundary
		}
		return nil
	}}
	publication := atomicFixture(t, "pub-catalog-race", "product-catalog-race", uint64Pointer(0))

	sourceResult := make(chan error, 1)
	go func() {
		_, publishErr := store.Publish(context.Background(), publication)
		sourceResult <- publishErr
	}()
	<-reachedCatalogBoundary

	catalogStore, err := catalogpersistence.NewRepository(db)
	require.NoError(t, err)
	publisher, err := catalog.NewPublisher(catalogStore)
	require.NoError(t, err)
	concurrentCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	_, concurrentErr := publisher.Publish(concurrentCtx, catalog.PublishRequest{
		Identity:            catalog.SnapshotIdentity{TenantID: publication.OrganizationID, ProductKey: publication.ProductKey},
		PublicationID:       publication.PublicationID,
		ExpectedBaseVersion: publication.ExpectedBaseVersion,
		Snapshot:            publication.Snapshot,
	})
	cancel()
	close(releaseCatalogBoundary)
	sourceErr := <-sourceResult

	if concurrentErr == nil {
		require.ErrorIs(t, sourceErr, sourcing.ErrSourcePublicationStateInvalid,
			"source publication must not adopt a Catalog fact committed after its orphan check")
		return
	}
	require.ErrorIs(t, concurrentErr, context.DeadlineExceeded)
	require.NoError(t, sourceErr)

	// Once the atomic source transaction releases Catalog's stream lock, an
	// ordinary Catalog replay with the same payload remains idempotent.
	_, err = publisher.Publish(context.Background(), catalog.PublishRequest{
		Identity:            catalog.SnapshotIdentity{TenantID: publication.OrganizationID, ProductKey: publication.ProductKey},
		PublicationID:       publication.PublicationID,
		ExpectedBaseVersion: publication.ExpectedBaseVersion,
		Snapshot:            publication.Snapshot,
	})
	require.NoError(t, err)
	assertCounts(t, db, 1, 1, 1, 1)
}

func TestPostgresAtomicPublicationAcceptsExactTwoMiBBoundaries(t *testing.T) {
	db := postgresFixture(t)
	store, err := NewRepository(db, testCatalogBridgeFactory)
	require.NoError(t, err)

	envelopePublication := atomicFixture(t, "pub-limit-envelope", "product-limit-envelope", uint64Pointer(0))
	envelopePublication.Envelope.SupplierOrCostFacts.Facts = make(map[string]string, sourcing.MaxSourceEnvelopeCollectionItems)
	keys := make([]string, sourcing.MaxSourceEnvelopeCollectionItems)
	for index := range keys {
		keys[index] = fmt.Sprintf("padding-%03d", index)
		envelopePublication.Envelope.SupplierOrCostFacts.Facts[keys[index]] = ""
	}
	envelopePublication = refreshAtomic(t, envelopePublication)
	remaining := sourcing.MaxEncodedEnvelopeBytes - len(envelopePublication.EnvelopeJSON)
	for _, key := range keys {
		added := min(remaining, sourcing.MaxSourceEnvelopeStringBytes)
		envelopePublication.Envelope.SupplierOrCostFacts.Facts[key] = strings.Repeat("x", added)
		remaining -= added
	}
	require.Zero(t, remaining)
	envelopePublication = refreshAtomic(t, envelopePublication)
	require.Len(t, envelopePublication.EnvelopeJSON, sourcing.MaxEncodedEnvelopeBytes)
	require.Less(t, len(envelopePublication.SnapshotJSON), sourcing.MaxEncodedSnapshotBytes)
	_, err = store.Publish(context.Background(), envelopePublication)
	require.NoError(t, err)

	snapshotPublication := atomicFixture(t, "pub-limit-snapshot", "product-limit-snapshot", uint64Pointer(0))
	snapshotPublication.Envelope.Warnings = make([]sourcing.SourceWarning, sourcing.MaxSourceEnvelopeCollectionItems)
	for index := range snapshotPublication.Envelope.Warnings {
		snapshotPublication.Envelope.Warnings[index] = sourcing.SourceWarning{Code: "source_warning", Field: "field", Message: fmt.Sprintf("message-%03d", index)}
	}
	snapshotPublication = refreshAtomic(t, snapshotPublication)
	delta := sourcing.MaxEncodedSnapshotBytes - len(snapshotPublication.SnapshotJSON)
	require.Positive(t, delta)
	if delta%2 != 0 {
		snapshotPublication.Envelope.Warnings[0].Field += "x"
		snapshotPublication = refreshAtomic(t, snapshotPublication)
		delta = sourcing.MaxEncodedSnapshotBytes - len(snapshotPublication.SnapshotJSON)
	}
	require.Zero(t, delta%2)
	remaining = delta / 2
	for index := range snapshotPublication.Envelope.Warnings {
		capacity := sourcing.MaxSourceEnvelopeStringBytes - len(snapshotPublication.Envelope.Warnings[index].Message)
		added := min(remaining, capacity)
		snapshotPublication.Envelope.Warnings[index].Message += strings.Repeat("x", added)
		remaining -= added
	}
	require.Zero(t, remaining)
	snapshotPublication = refreshAtomic(t, snapshotPublication)
	require.Len(t, snapshotPublication.SnapshotJSON, sourcing.MaxEncodedSnapshotBytes)
	require.Less(t, len(snapshotPublication.EnvelopeJSON), sourcing.MaxEncodedEnvelopeBytes)
	_, err = store.Publish(context.Background(), snapshotPublication)
	require.NoError(t, err)

	persisted, err := store.Read(context.Background(), "org-a", "pub-limit-snapshot")
	require.NoError(t, err)
	require.Len(t, persisted.Snapshot.Review.Reasons[0], len(snapshotPublication.Envelope.Warnings[0].Message))
}

func assertCounts(t *testing.T, db *gorm.DB, versions, heads, evidence, receipts int64) {
	t.Helper()
	var gotVersions, gotHeads, gotEvidence, gotReceipts int64
	require.NoError(t, db.Model(&catalogpersistence.SnapshotVersionRecord{}).Count(&gotVersions).Error)
	require.NoError(t, db.Model(&catalogpersistence.SnapshotHeadRecord{}).Count(&gotHeads).Error)
	require.NoError(t, db.Model(&publicationRecord{}).Count(&gotEvidence).Error)
	require.NoError(t, db.Model(&receiptRecord{}).Count(&gotReceipts).Error)
	require.Equal(t, versions, gotVersions)
	require.Equal(t, heads, gotHeads)
	require.Equal(t, evidence, gotEvidence)
	require.Equal(t, receipts, gotReceipts)
}

func uint64Pointer(value uint64) *uint64 { return &value }

func refreshAtomic(t *testing.T, publication sourcing.AtomicPublication) sourcing.AtomicPublication {
	t.Helper()
	envelope, err := sourcing.NormalizePublicationEnvelope(publication.Envelope)
	require.NoError(t, err)
	snapshot, err := sourcing.ToSnapshot(envelope)
	require.NoError(t, err)
	envelopeJSON, err := json.Marshal(envelope)
	require.NoError(t, err)
	snapshotJSON, err := json.Marshal(snapshot)
	require.NoError(t, err)
	inputHash, err := sourcing.CanonicalPublicationInputHash(publication.Producer, envelope, envelopeJSON, publication.ProductKey, publication.ExpectedBaseVersion)
	require.NoError(t, err)
	publication.Envelope, publication.Snapshot = envelope, snapshot
	publication.EnvelopeJSON, publication.SnapshotJSON = envelopeJSON, snapshotJSON
	publication.InputHash = inputHash
	return publication
}
