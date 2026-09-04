package storecenter_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"task-processor/internal/storecenter"

	"gorm.io/gorm"
)

func TestNoAuthoritativeHistorySourceManifestProducesStableApprovedResolution(t *testing.T) {
	approvedAt := time.Date(2026, 9, 4, 14, 0, 0, 0, time.UTC)
	manifest := storecenter.NoAuthoritativeHistorySourceManifest{
		SchemaVersion:     storecenter.NoAuthoritativeHistorySourceManifestV1,
		DecisionReference: "product-decision:store-service-history:phase1",
		ApprovedBy:        "repository-owner",
		ApprovedAt:        approvedAt,
	}
	resolver, err := storecenter.NewNoAuthoritativeHistorySourceResolver(manifest)
	if err != nil {
		t.Fatal(err)
	}

	resolution, freeze, err := resolver.Resolve(context.Background(), storecenter.StoreSnapshot{
		ID: "00000000-0000-4000-8000-000000000701", OrganizationID: "org-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Status != storecenter.HistoryConfirmedAbsent || resolution.SourceIdentity != manifest.DecisionReference || len(resolution.SourceSnapshotToken) != 64 {
		t.Fatalf("resolution = %+v, want approved confirmed-absent evidence", resolution)
	}
	if err := storecenter.ValidateLegacyServiceHistoryFreeze(resolution, freeze); err != nil {
		t.Fatalf("ValidateLegacyServiceHistoryFreeze() error = %v", err)
	}
	if err := freeze.Handoff(context.Background()); err != nil {
		t.Fatalf("Handoff() error = %v", err)
	}
	if err := freeze.Release(context.Background()); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	changed := manifest
	changed.ApprovedBy = "different-approver"
	changedResolver, err := storecenter.NewNoAuthoritativeHistorySourceResolver(changed)
	if err != nil {
		t.Fatal(err)
	}
	changedResolution, _, err := changedResolver.Resolve(context.Background(), storecenter.StoreSnapshot{
		ID: "00000000-0000-4000-8000-000000000701", OrganizationID: "org-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if changedResolution.SourceSnapshotToken == resolution.SourceSnapshotToken {
		t.Fatal("approval change did not change the immutable snapshot token")
	}
}

func TestNoAuthoritativeHistorySourceManifestRejectsMissingApproval(t *testing.T) {
	_, err := storecenter.NewNoAuthoritativeHistorySourceResolver(storecenter.NoAuthoritativeHistorySourceManifest{
		SchemaVersion:     storecenter.NoAuthoritativeHistorySourceManifestV1,
		DecisionReference: "product-decision:store-service-history:phase1",
		ApprovedAt:        time.Date(2026, 9, 4, 14, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, storecenter.ErrInvalidNoAuthoritativeHistoryManifest) {
		t.Fatalf("NewNoAuthoritativeHistorySourceResolver() error = %v, want invalid manifest", err)
	}
}

func TestGormStoreHistoryBackfillMapsLegacyRowsAndClearsUnsupportedExpandedService(t *testing.T) {
	db := openStoreDB(t)
	now := time.Date(2026, 9, 4, 15, 0, 0, 0, time.UTC)
	seedLegacyStoreRow(t, db, "00000000-0000-4000-8000-000000000711", "org-a", "active", 2, nil)
	seedLegacyStoreRow(t, db, "00000000-0000-4000-8000-000000000712", "org-a", "disabled", 3, nil)
	deletedAt := now.Add(-time.Hour)
	seedLegacyStoreRow(t, db, "00000000-0000-4000-8000-000000000713", "org-a", "deleting", 4, &deletedAt)
	seedLegacyStoreRow(t, db, "00000000-0000-4000-8000-000000000714", "org-a", "active", 7, nil)
	startedAt := now.Add(-24 * time.Hour)
	expiresAt := now.Add(29 * 24 * time.Hour)
	if err := db.Table("workbench_stores").Where("id = ?", "00000000-0000-4000-8000-000000000714").Updates(map[string]any{
		"record_status": "active", "service_status": "active",
		"service_started_at": startedAt, "service_expires_at": expiresAt,
	}).Error; err != nil {
		t.Fatal(err)
	}

	migrator := newNoHistoryMigrator(t, db, now)
	report, err := migrator.BackfillBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("BackfillBatch() error = %v", err)
	}
	if report.ScannedCount != 4 || report.UpdatedCount != 4 || report.HistoryConfirmedAbsentCount != 3 {
		t.Fatalf("BackfillBatch() report = %+v", report)
	}

	assertBackfilledStoreRow(t, db, "00000000-0000-4000-8000-000000000711", "active", "pending_activation", false, 3)
	assertBackfilledStoreRow(t, db, "00000000-0000-4000-8000-000000000712", "active", "suspended", false, 4)
	assertBackfilledStoreRow(t, db, "00000000-0000-4000-8000-000000000713", "deleted", "", false, 5)
	assertBackfilledStoreRow(t, db, "00000000-0000-4000-8000-000000000714", "active", "pending_activation", false, 8)

	second, err := migrator.BackfillBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("second BackfillBatch() error = %v", err)
	}
	if second.ScannedCount != 0 || second.UpdatedCount != 0 {
		t.Fatalf("second BackfillBatch() report = %+v, want idempotent no-op", second)
	}

	verification, err := migrator.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify() error = %v, report = %+v", err, verification)
	}
	if !verification.ReadyForConstraints || verification.HistoryConfirmedAbsentCount != 3 || verification.UnresolvedCount != 0 || verification.HistorySnapshotConflictCount != 0 {
		t.Fatalf("Verify() report = %+v, want ready", verification)
	}
}

func TestGormStoreHistoryVerificationBlocksUnresolvedAndManifestDrift(t *testing.T) {
	db := openStoreDB(t)
	now := time.Date(2026, 9, 4, 15, 0, 0, 0, time.UTC)
	seedLegacyStoreRow(t, db, "00000000-0000-4000-8000-000000000721", "org-a", "active", 2, nil)
	migrator := newNoHistoryMigrator(t, db, now)

	report, err := migrator.Verify(context.Background())
	if !errors.Is(err, storecenter.ErrStoreHistoryRolloutBlocked) || report.UnresolvedCount != 1 || report.ReadyForConstraints {
		t.Fatalf("Verify() = %+v, %v; want unresolved blocker", report, err)
	}

	if _, err := migrator.BackfillBatch(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if err := db.Table("workbench_stores").Where("id = ?", "00000000-0000-4000-8000-000000000721").Update("service_history_snapshot_token", "different-manifest-token").Error; err != nil {
		t.Fatal(err)
	}
	report, err = migrator.Verify(context.Background())
	if !errors.Is(err, storecenter.ErrStoreHistoryRolloutBlocked) || report.HistorySnapshotConflictCount != 1 || report.ReadyForConstraints {
		t.Fatalf("Verify() = %+v, %v; want snapshot conflict blocker", report, err)
	}
}

func TestGormStoreHistoryVerificationStreamsRows(t *testing.T) {
	db := openStoreDB(t)
	now := time.Date(2026, 9, 4, 15, 0, 0, 0, time.UTC)
	seedLegacyStoreRow(t, db, "00000000-0000-4000-8000-000000000723", "org-a", "active", 2, nil)
	migrator := newNoHistoryMigrator(t, db, now)
	if _, err := migrator.BackfillBatch(context.Background(), 10); err != nil {
		t.Fatal(err)
	}

	errUnboundedDestination := errors.New("verification must not materialize a result slice")
	if err := db.Callback().Query().Before("gorm:query").Register("test:reject-unbounded-verification", func(tx *gorm.DB) {
		destination := reflect.ValueOf(tx.Statement.Dest)
		if destination.IsValid() && destination.Kind() == reflect.Ptr && !destination.IsNil() && destination.Elem().Kind() == reflect.Slice {
			tx.AddError(errUnboundedDestination)
		}
	}); err != nil {
		t.Fatal(err)
	}

	report, err := migrator.Verify(context.Background())
	if err != nil || !report.ReadyForConstraints || report.ScannedCount != 1 {
		t.Fatalf("Verify() = %+v, %v; want one streamed row", report, err)
	}
}

func TestGormStoreHistoryVerificationFencesLegacyProvisioningActivation(t *testing.T) {
	db := openStoreDB(t)
	now := time.Date(2026, 9, 4, 15, 0, 0, 0, time.UTC)
	storeID := "00000000-0000-4000-8000-000000000725"
	seedLegacyStoreRow(t, db, storeID, "org-a", "provisioning", 1, nil)
	// Reproduce a row mapped by the original migrator: it had expanded state
	// but no history evidence and Verify incorrectly reported readiness.
	if err := db.Table("workbench_stores").Where("id = ?", storeID).Update("record_status", "provisioning").Error; err != nil {
		t.Fatal(err)
	}
	migrator := newNoHistoryMigrator(t, db, now)

	report, err := migrator.Verify(context.Background())
	if !errors.Is(err, storecenter.ErrStoreHistoryRolloutBlocked) || report.UnresolvedCount != 1 || report.ReadyForConstraints {
		t.Fatalf("Verify() = %+v, %v; want provisioning row fenced before cutover", report, err)
	}

	backfill, err := migrator.BackfillBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("BackfillBatch() error = %v", err)
	}
	if backfill.ScannedCount != 1 || backfill.UpdatedCount != 1 || backfill.HistoryConfirmedAbsentCount != 1 {
		t.Fatalf("BackfillBatch() report = %+v, want provisioning history evidence", backfill)
	}

	repository, err := storecenter.NewGormStoreRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	store, err := repository.Get(context.Background(), "org-a", storeID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.TransitionTo(storecenter.StoreStatusActive, "recovery", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repository.Save(context.Background(), "org-a", store, 2); err != nil {
		t.Fatal(err)
	}

	report, err = migrator.Verify(context.Background())
	if err != nil || !report.ReadyForConstraints || report.HistoryConfirmedAbsentCount != 1 || report.UnresolvedCount != 0 {
		t.Fatalf("Verify() after activation = %+v, %v; want preserved definitive evidence", report, err)
	}
}

func TestGormStoreHistoryVerificationAcceptsNewStoreEvidenceAcrossCompatibilityWrites(t *testing.T) {
	db := openStoreDB(t)
	repository, err := storecenter.NewGormStoreRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 9, 4, 16, 0, 0, 0, time.UTC)
	store := newPersistenceStore(t, "org-new", "00000000-0000-4000-8000-000000000731", "00000000-0000-4000-8000-000000000732", "00000000-0000-4000-8000-000000000733", "New", "SG", "new-external", createdAt)
	if _, _, err := repository.CreateOrReplay(context.Background(), "org-new", store); err != nil {
		t.Fatal(err)
	}
	if err := store.TransitionTo(storecenter.StoreStatusActive, "creator", createdAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repository.Save(context.Background(), "org-new", store, 1); err != nil {
		t.Fatal(err)
	}

	migrator := newNoHistoryMigrator(t, db, createdAt.Add(2*time.Minute))
	report, err := migrator.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify() error = %v, report = %+v", err, report)
	}
	if !report.ReadyForConstraints || report.HistoryNotApplicableCount != 1 || report.HistoryConfirmedAbsentCount != 0 {
		t.Fatalf("Verify() report = %+v, want one new-store exclusion", report)
	}

	var row struct {
		Status   *string `gorm:"column:service_history_resolution_status"`
		Source   *string `gorm:"column:service_history_source_identity"`
		Token    *string `gorm:"column:service_history_snapshot_token"`
		CreateFP string  `gorm:"column:create_request_fingerprint"`
	}
	if err := db.Table("workbench_stores").Where("id = ?", store.ID()).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status == nil || *row.Status != "not_applicable_new" || row.Source == nil || *row.Source != "store-create" || row.Token == nil || *row.Token != row.CreateFP {
		t.Fatalf("new Store history evidence = %+v", row)
	}
}

func newNoHistoryMigrator(t *testing.T, db *gorm.DB, now time.Time) *storecenter.GormStoreHistoryMigrator {
	t.Helper()
	migrator, err := storecenter.NewGormStoreHistoryMigrator(db, storecenter.NoAuthoritativeHistorySourceManifest{
		SchemaVersion:     storecenter.NoAuthoritativeHistorySourceManifestV1,
		DecisionReference: "product-decision:store-service-history:phase1",
		ApprovedBy:        "repository-owner",
		ApprovedAt:        time.Date(2026, 9, 4, 14, 0, 0, 0, time.UTC),
	}, "store-history-migration", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	return migrator
}

func seedLegacyStoreRow(t *testing.T, db *gorm.DB, id, organizationID, lifecycle string, version int64, deletedAt *time.Time) {
	t.Helper()
	createdAt := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	values := map[string]any{
		"id": id, "organization_id": organizationID, "name": "Legacy", "platform": "shein", "region": "SG",
		"external_store_id": id, "lifecycle_status": lifecycle, "connection_ref": "", "quota_allocation_id": "00000000-0000-4000-8000-000000000799",
		"version": version, "created_by": "legacy", "updated_by": "legacy", "created_at": createdAt, "updated_at": createdAt,
		"create_idempotency_key": id, "delete_operation_key": "", "identity_key": id, "create_request_fingerprint": id,
		"record_status": nil, "service_status": nil, "service_started_at": nil, "service_expires_at": nil,
		"service_history_resolution_status": nil, "service_history_source_identity": nil,
		"service_history_snapshot_token": nil, "service_history_resolved_at": nil,
	}
	if lifecycle == "deleting" {
		values["delete_operation_key"] = "00000000-0000-4000-8000-000000000798"
	}
	if deletedAt != nil {
		values["deleted_at"] = *deletedAt
	}
	if err := db.Table("workbench_stores").Create(values).Error; err != nil {
		t.Fatal(err)
	}
}

func assertBackfilledStoreRow(t *testing.T, db *gorm.DB, id, wantRecord, wantService string, wantPeriod bool, wantVersion int64) {
	t.Helper()
	var row struct {
		RecordStatus      *string    `gorm:"column:record_status"`
		ServiceStatus     *string    `gorm:"column:service_status"`
		ServiceStartedAt  *time.Time `gorm:"column:service_started_at"`
		ServiceExpiresAt  *time.Time `gorm:"column:service_expires_at"`
		HistoryStatus     *string    `gorm:"column:service_history_resolution_status"`
		HistoryIdentity   *string    `gorm:"column:service_history_source_identity"`
		HistoryToken      *string    `gorm:"column:service_history_snapshot_token"`
		HistoryResolvedAt *time.Time `gorm:"column:service_history_resolved_at"`
		Version           int64      `gorm:"column:version"`
	}
	if err := db.Table("workbench_stores").Unscoped().Where("id = ?", id).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.RecordStatus == nil || *row.RecordStatus != wantRecord || row.Version != wantVersion {
		t.Fatalf("row %s record/version = %#v/%d, want %q/%d", id, row.RecordStatus, row.Version, wantRecord, wantVersion)
	}
	if wantService == "" {
		if row.ServiceStatus != nil || row.HistoryStatus != nil {
			t.Fatalf("row %s service/history = %#v/%#v, want nil", id, row.ServiceStatus, row.HistoryStatus)
		}
		return
	}
	if row.ServiceStatus == nil || *row.ServiceStatus != wantService || row.HistoryStatus == nil || *row.HistoryStatus != "confirmed_absent" || row.HistoryIdentity == nil || row.HistoryToken == nil || len(*row.HistoryToken) != 64 || row.HistoryResolvedAt == nil {
		t.Fatalf("row %s missing service history evidence: %+v", id, row)
	}
	if wantPeriod != (row.ServiceStartedAt != nil && row.ServiceExpiresAt != nil) {
		t.Fatalf("row %s period = %v/%v, wantPeriod=%v", id, row.ServiceStartedAt, row.ServiceExpiresAt, wantPeriod)
	}
}
