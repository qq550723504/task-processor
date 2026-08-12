package listingadmin

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/model"
)

func TestRecoverStore986PlatformCohortDryRunDoesNotWrite(t *testing.T) {
	t.Parallel()

	repo, db := newPlatformRecoveryRepository(t)
	id := seedPlatformRecoveryTask(t, db, 1, 986, "Amazon", "Amazon", "SHEIN", "P-1", model.TaskStatusPending.Int16())

	report, err := repo.RecoverStore986PlatformCohort(context.Background(), PlatformRecoveryRequest{
		StoreID:       986,
		ExpectedCount: 1,
	})
	if err != nil {
		t.Fatalf("RecoverStore986PlatformCohort() error = %v", err)
	}
	if !report.DryRun || !equalInt64s(report.SelectedIDs, []int64{id}) || len(report.UpdatedIDs) != 0 {
		t.Fatalf("report = %+v, want dry-run selection without updates", report)
	}
	assertPlatformRecoveryTask(t, db, id, "Amazon", "Amazon", "SHEIN", model.TaskStatusPending.Int16())
}

func TestRecoverStore986PlatformCohortRejectsWrongCountWithoutWriting(t *testing.T) {
	t.Parallel()

	repo, db := newPlatformRecoveryRepository(t)
	id := seedPlatformRecoveryTask(t, db, 1, 986, "Amazon", "Amazon", "SHEIN", "P-1", model.TaskStatusPending.Int16())

	if _, err := repo.RecoverStore986PlatformCohort(context.Background(), PlatformRecoveryRequest{
		StoreID:       986,
		ExpectedCount: 200,
		Execute:       true,
	}); err == nil {
		t.Fatal("RecoverStore986PlatformCohort() error = nil, want expected-count rejection")
	}
	assertPlatformRecoveryTask(t, db, id, "Amazon", "Amazon", "SHEIN", model.TaskStatusPending.Int16())
}

func TestRecoverStore986PlatformCohortRejectsInvalidScopeBeforeQuerying(t *testing.T) {
	t.Parallel()

	repo, _ := newPlatformRecoveryRepository(t)
	for _, request := range []PlatformRecoveryRequest{
		{StoreID: 985, ExpectedCount: 1},
		{StoreID: 986, ExpectedCount: 0},
	} {
		if _, err := repo.RecoverStore986PlatformCohort(context.Background(), request); err == nil {
			t.Fatalf("RecoverStore986PlatformCohort(%+v) error = nil, want scope validation error", request)
		}
	}
}

func TestRecoverStore986PlatformCohortRejectsActiveCaseFoldedDuplicate(t *testing.T) {
	t.Parallel()

	repo, db := newPlatformRecoveryRepository(t)
	id := seedPlatformRecoveryTask(t, db, 1, 986, "Amazon", "Amazon", "SHEIN", "P-1", model.TaskStatusPending.Int16())
	seedPlatformRecoveryTask(t, db, 2, 986, "amazon", "amazon", "shein", "P-1", model.TaskStatusPending.Int16())

	if _, err := repo.RecoverStore986PlatformCohort(context.Background(), PlatformRecoveryRequest{
		StoreID:       986,
		ExpectedCount: 2,
		Execute:       true,
	}); err == nil {
		t.Fatal("RecoverStore986PlatformCohort() error = nil, want duplicate rejection")
	}
	assertPlatformRecoveryTask(t, db, id, "Amazon", "Amazon", "SHEIN", model.TaskStatusPending.Int16())
}

func TestRecoverStore986PlatformCohortNormalizesOnlyEligiblePendingRows(t *testing.T) {
	t.Parallel()

	repo, db := newPlatformRecoveryRepository(t)
	id := seedPlatformRecoveryTask(t, db, 1, 986, "Amazon", "Amazon", "SHEIN", "P-1", model.TaskStatusPending.Int16())
	skipped := seedPlatformRecoveryTask(t, db, 2, 986, "Amazon", "Amazon", "SHEIN", "P-2", model.TaskStatusPublished.Int16())

	report, err := repo.RecoverStore986PlatformCohort(context.Background(), PlatformRecoveryRequest{
		StoreID:       986,
		ExpectedCount: 1,
		Execute:       true,
	})
	if err != nil {
		t.Fatalf("RecoverStore986PlatformCohort() error = %v", err)
	}
	if report.DryRun || !equalInt64s(report.SelectedIDs, []int64{id}) || !equalInt64s(report.UpdatedIDs, []int64{id}) {
		t.Fatalf("report = %+v, want one updated selected task", report)
	}
	assertPlatformRecoveryTask(t, db, id, "amazon", "amazon", "shein", model.TaskStatusPending.Int16())
	assertPlatformRecoveryTask(t, db, skipped, "Amazon", "Amazon", "SHEIN", model.TaskStatusPublished.Int16())
}

func newPlatformRecoveryRepository(t *testing.T) (*GormImportTaskRepository, *gorm.DB) {
	t.Helper()
	db := newImportTaskDispatchTestDB(t)
	return NewGormImportTaskRepository(db), db
}

func seedPlatformRecoveryTask(t *testing.T, db *gorm.DB, id, storeID int64, platform, sourcePlatform, targetPlatform, productID string, status int16) int64 {
	t.Helper()
	now := time.Now()
	row := listingProductImportTask{
		ID:             id,
		TenantID:       1,
		StoreID:        storeID,
		Platform:       platform,
		SourcePlatform: sourcePlatform,
		TargetPlatform: targetPlatform,
		Region:         "US",
		ProductID:      productID,
		Status:         status,
		Priority:       5,
		CreateTime:     &now,
		UpdateTime:     &now,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed platform recovery task %d: %v", id, err)
	}
	return row.ID
}

func assertPlatformRecoveryTask(t *testing.T, db *gorm.DB, id int64, platform, sourcePlatform, targetPlatform string, status int16) {
	t.Helper()
	var row listingProductImportTask
	if err := db.Where("id = ?", id).Take(&row).Error; err != nil {
		t.Fatalf("load task %d: %v", id, err)
	}
	if row.Platform != platform || row.SourcePlatform != sourcePlatform || row.TargetPlatform != targetPlatform || row.Status != status {
		t.Fatalf("task %d = %+v, want platforms %q/%q/%q status %d", id, row, platform, sourcePlatform, targetPlatform, status)
	}
}
