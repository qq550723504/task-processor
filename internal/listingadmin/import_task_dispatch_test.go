package listingadmin

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
	"task-processor/internal/model"
)

func TestListDispatchCandidatesFairAlternatesStoresBeforeSecondTask(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	oldest := now.Add(-3 * time.Minute)
	middle := now.Add(-2 * time.Minute)
	newest := now.Add(-1 * time.Minute)
	rows := []listingProductImportTask{
		{ID: 1, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "s100-high", Status: model.TaskStatusPending.Int16(), Priority: 90, CreateTime: &oldest, UpdateTime: &oldest, Deleted: 0},
		{ID: 2, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "s100-second", Status: model.TaskStatusPendingRetry.Int16(), Priority: 80, CreateTime: &middle, UpdateTime: &middle, Deleted: 0},
		{ID: 3, TenantID: 10, StoreID: 200, Platform: "legacy", TargetPlatform: "shein", Region: "us", ProductID: "s200-high", Status: model.TaskStatusCrawled.Int16(), Priority: 70, CreateTime: &newest, UpdateTime: &newest, Deleted: 0},
		{ID: 4, TenantID: 10, StoreID: 300, Platform: "amazon", Region: "us", ProductID: "wrong-platform", Status: model.TaskStatusPending.Int16(), Priority: 100, CreateTime: &oldest, UpdateTime: &oldest, Deleted: 0},
		{ID: 5, TenantID: 10, StoreID: 400, Platform: "shein", Region: "us", ProductID: "excluded", Status: model.TaskStatusPending.Int16(), Priority: 100, CreateTime: &oldest, UpdateTime: &oldest, Deleted: 0},
	}
	seedDispatchTasks(t, db, rows)

	repo := NewGormImportTaskRepository(db)
	tasks, err := repo.ListDispatchCandidatesFair(context.Background(), DispatchCandidateRequest{
		Platform:         "shein",
		Limit:            3,
		PerStoreLimit:    2,
		ExcludedStoreIDs: []int64{400},
	})
	if err != nil {
		t.Fatalf("ListDispatchCandidatesFair() error = %v", err)
	}

	got := taskIDs(tasks)
	want := []int64{1, 3, 2}
	if !equalInt64s(got, want) {
		t.Fatalf("task IDs = %v, want %v", got, want)
	}
}

func TestClaimForDispatchSucceedsOnlyFromExpectedStatus(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 11, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "claim", Status: model.TaskStatusPending.Int16(), ErrorMessage: "old error", ReasonCode: "OLD", Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	claimed, err := repo.ClaimForDispatch(context.Background(), DispatchClaim{
		TaskID:         11,
		PreviousStatus: model.TaskStatusPendingRetry.Int16(),
		ProcessingNode: "node-a",
		Remark:         "dispatching to queue",
	})
	if err != nil {
		t.Fatalf("ClaimForDispatch(wrong status) error = %v", err)
	}
	if claimed {
		t.Fatalf("ClaimForDispatch(wrong status) = true, want false")
	}

	claimed, err = repo.ClaimForDispatch(context.Background(), DispatchClaim{
		TaskID:         11,
		PreviousStatus: model.TaskStatusPending.Int16(),
		ProcessingNode: "node-a",
		Remark:         "dispatching to queue",
	})
	if err != nil {
		t.Fatalf("ClaimForDispatch(expected status) error = %v", err)
	}
	if !claimed {
		t.Fatalf("ClaimForDispatch(expected status) = false, want true")
	}

	var row listingProductImportTask
	if err := db.Table("listing_product_import_task").Where("id = ?", int64(11)).Take(&row).Error; err != nil {
		t.Fatalf("load claimed row: %v", err)
	}
	if row.Status != model.TaskStatusQueued.Int16() || row.ProcessingNode != "node-a" || row.ErrorMessage != "" || row.ReasonCode != "" || row.Remark != "dispatching to queue" {
		t.Fatalf("claimed row = %+v, want queued with node, cleared error fields, and remark", row)
	}
}

func TestRollbackDispatchRestoresPreviousStatusWithVisibleReason(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 21, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "rollback", Status: model.TaskStatusQueued.Int16(), ProcessingNode: "node-a", Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	if err := repo.RollbackDispatch(context.Background(), 21, model.TaskStatusCrawled.Int16(), "RabbitMQ publish failed"); err != nil {
		t.Fatalf("RollbackDispatch() error = %v", err)
	}

	var row listingProductImportTask
	if err := db.Table("listing_product_import_task").Where("id = ?", int64(21)).Take(&row).Error; err != nil {
		t.Fatalf("load rolled back row: %v", err)
	}
	if row.Status != model.TaskStatusCrawled.Int16() || row.ErrorMessage != "RabbitMQ publish failed" || row.Remark != "RabbitMQ publish failed" || row.ProcessingNode != "" {
		t.Fatalf("rolled back row = %+v, want previous status with visible reason and cleared node", row)
	}
}

func TestCountQueuedByStoreGroupsAcrossTenantsForPlatform(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 31, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "queued-a", Status: model.TaskStatusQueued.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 32, TenantID: 20, StoreID: 100, Platform: "legacy", TargetPlatform: "shein", Region: "us", ProductID: "queued-b", Status: model.TaskStatusQueued.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 33, TenantID: 10, StoreID: 200, Platform: "shein", Region: "us", ProductID: "queued-c", Status: model.TaskStatusQueued.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 34, TenantID: 10, StoreID: 200, Platform: "shein", Region: "us", ProductID: "pending", Status: model.TaskStatusPending.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 35, TenantID: 10, StoreID: 300, Platform: "amazon", Region: "us", ProductID: "wrong-platform", Status: model.TaskStatusQueued.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	counts, err := repo.CountQueuedByStore(context.Background(), "shein", []int64{100, 200, 300})
	if err != nil {
		t.Fatalf("CountQueuedByStore() error = %v", err)
	}
	if counts[100] != 2 || counts[200] != 1 {
		t.Fatalf("counts = %v, want store 100=2 and store 200=1", counts)
	}
	if _, ok := counts[300]; ok {
		t.Fatalf("counts includes store 300: %v, want wrong platform omitted", counts)
	}
}

func newImportTaskDispatchTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductImportTask{}); err != nil {
		t.Fatalf("migrate import task: %v", err)
	}
	return db
}

func seedDispatchTasks(t *testing.T, db *gorm.DB, rows []listingProductImportTask) {
	t.Helper()
	for _, row := range rows {
		if err := db.Table("listing_product_import_task").Create(&row).Error; err != nil {
			t.Fatalf("seed row %d: %v", row.ID, err)
		}
	}
}

func taskIDs(tasks []ImportTask) []int64 {
	ids := make([]int64, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

func equalInt64s(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
