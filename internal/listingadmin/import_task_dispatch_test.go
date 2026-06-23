package listingadmin

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
	"task-processor/internal/model"
)

type legacyImportTaskWithoutProcessingNode struct {
	ID          int64  `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID    int64  `gorm:"column:tenant_id"`
	OwnerUserID string `gorm:"column:owner_user_id"`
	StoreID     int64  `gorm:"column:store_id"`
	Platform    string `gorm:"column:platform"`
	Region      string `gorm:"column:region"`
	CategoryID  int64  `gorm:"column:category_id"`
	ProductID   string `gorm:"column:product_id"`
	Status      int16  `gorm:"column:status"`
	Deleted     int16  `gorm:"column:deleted"`
	CreatedBy   string `gorm:"column:created_by"`
	UpdatedBy   string `gorm:"column:updated_by"`
}

func (legacyImportTaskWithoutProcessingNode) TableName() string {
	return "listing_product_import_task"
}

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
		{ID: 6, TenantID: 10, StoreID: 500, Platform: "shein", TargetPlatform: "", Region: "us", ProductID: "empty-target-platform", Status: model.TaskStatusPending.Int16(), Priority: 100, CreateTime: &oldest, UpdateTime: &oldest, Deleted: 0},
	}
	seedDispatchTasks(t, db, rows)
	setNullTargetPlatforms(t, db, 1, 2, 4, 5)

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

func TestClaimForDispatchRejectsBlankProcessingNode(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 12, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "blank-node", Status: model.TaskStatusPending.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	claimed, err := repo.ClaimForDispatch(context.Background(), DispatchClaim{
		TaskID:         12,
		PreviousStatus: model.TaskStatusPending.Int16(),
		ProcessingNode: "  ",
		Remark:         "dispatching to queue",
	})
	if err == nil {
		t.Fatal("ClaimForDispatch(blank processing node) error = nil, want error")
	}
	if claimed {
		t.Fatal("ClaimForDispatch(blank processing node) = true, want false")
	}

	var row listingProductImportTask
	if err := db.Table("listing_product_import_task").Where("id = ?", int64(12)).Take(&row).Error; err != nil {
		t.Fatalf("load rejected claim row: %v", err)
	}
	if row.Status != model.TaskStatusPending.Int16() || row.ProcessingNode != "" || row.Remark != "" {
		t.Fatalf("rejected claim row = %+v, want unchanged pending row", row)
	}
}

func TestClaimForDispatchRejectsInvalidPreviousStatus(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 13, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "invalid-status", Status: model.TaskStatusPublished.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	claimed, err := repo.ClaimForDispatch(context.Background(), DispatchClaim{
		TaskID:         13,
		PreviousStatus: model.TaskStatusPublished.Int16(),
		ProcessingNode: "node-a",
		Remark:         "dispatching to queue",
	})
	if err == nil {
		t.Fatal("ClaimForDispatch(invalid previous status) error = nil, want error")
	}
	if claimed {
		t.Fatal("ClaimForDispatch(invalid previous status) = true, want false")
	}

	var row listingProductImportTask
	if err := db.Table("listing_product_import_task").Where("id = ?", int64(13)).Take(&row).Error; err != nil {
		t.Fatalf("load invalid status claim row: %v", err)
	}
	if row.Status != model.TaskStatusPublished.Int16() || row.ProcessingNode != "" || row.Remark != "" {
		t.Fatalf("invalid status claim row = %+v, want unchanged published row", row)
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
	if err := repo.RollbackDispatch(context.Background(), 21, model.TaskStatusCrawled.Int16(), "node-a", "RabbitMQ publish failed"); err != nil {
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

func TestRollbackDispatchReturnsNotFoundWhenNoQueuedRowRestored(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 22, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "not-queued", Status: model.TaskStatusPending.Int16(), ProcessingNode: "node-a", Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	if err := repo.RollbackDispatch(context.Background(), 22, model.TaskStatusCrawled.Int16(), "node-a", "publish failed"); !errors.Is(err, ErrImportTaskNotFound) {
		t.Fatalf("RollbackDispatch() error = %v, want ErrImportTaskNotFound", err)
	}
}

func TestRollbackDispatchRejectsStaleProcessingNode(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 23, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "reclaimed", Status: model.TaskStatusQueued.Int16(), ProcessingNode: "node-b", ErrorMessage: "new claim", Remark: "new claim", Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})

	repo := NewGormImportTaskRepository(db)
	if err := repo.RollbackDispatch(context.Background(), 23, model.TaskStatusPending.Int16(), "node-a", "stale publish failed"); !errors.Is(err, ErrImportTaskNotFound) {
		t.Fatalf("RollbackDispatch(stale node) error = %v, want ErrImportTaskNotFound", err)
	}

	var row listingProductImportTask
	if err := db.Table("listing_product_import_task").Where("id = ?", int64(23)).Take(&row).Error; err != nil {
		t.Fatalf("load stale rollback row: %v", err)
	}
	if row.Status != model.TaskStatusQueued.Int16() || row.ProcessingNode != "node-b" || row.ErrorMessage != "new claim" || row.Remark != "new claim" {
		t.Fatalf("stale rollback row = %+v, want newer queued claim unchanged", row)
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
		{ID: 36, TenantID: 10, StoreID: 400, Platform: "shein", TargetPlatform: "", Region: "us", ProductID: "empty-target-platform", Status: model.TaskStatusQueued.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})
	setNullTargetPlatforms(t, db, 31, 33, 34, 35)

	repo := NewGormImportTaskRepository(db)
	counts, err := repo.CountQueuedByStore(context.Background(), "shein", []int64{100, 200, 300, 400})
	if err != nil {
		t.Fatalf("CountQueuedByStore() error = %v", err)
	}
	if counts[100] != 2 || counts[200] != 1 {
		t.Fatalf("counts = %v, want store 100=2 and store 200=1", counts)
	}
	if _, ok := counts[300]; ok {
		t.Fatalf("counts includes store 300: %v, want wrong platform omitted", counts)
	}
	if _, ok := counts[400]; ok {
		t.Fatalf("counts includes store 400: %v, want empty target platform omitted", counts)
	}
}

func TestDispatchQueriesReturnEmptyForBlankPlatform(t *testing.T) {
	t.Parallel()

	db := newImportTaskDispatchTestDB(t)
	now := time.Now()
	seedDispatchTasks(t, db, []listingProductImportTask{
		{ID: 41, TenantID: 10, StoreID: 100, Platform: "", Region: "us", ProductID: "blank-candidate", Status: model.TaskStatusPending.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 42, TenantID: 10, StoreID: 100, Platform: "", Region: "us", ProductID: "blank-queued", Status: model.TaskStatusQueued.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	})
	setNullTargetPlatforms(t, db, 41, 42)

	repo := NewGormImportTaskRepository(db)
	candidates, err := repo.ListDispatchCandidatesFair(context.Background(), DispatchCandidateRequest{
		Platform:      "  ",
		Limit:         10,
		PerStoreLimit: 1,
	})
	if err != nil {
		t.Fatalf("ListDispatchCandidatesFair(blank platform) error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("ListDispatchCandidatesFair(blank platform) len = %d, want 0", len(candidates))
	}

	counts, err := repo.CountQueuedByStore(context.Background(), "  ", []int64{100})
	if err != nil {
		t.Fatalf("CountQueuedByStore(blank platform) error = %v", err)
	}
	if len(counts) != 0 {
		t.Fatalf("CountQueuedByStore(blank platform) = %v, want empty map", counts)
	}
}

func TestAutoMigrateImportTaskRepositoryEnsuresProcessingNodeColumn(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&legacyImportTaskWithoutProcessingNode{}); err != nil {
		t.Fatalf("migrate legacy import task: %v", err)
	}
	if db.Migrator().HasColumn("listing_product_import_task", "processing_node") {
		t.Fatal("legacy table unexpectedly has processing_node before migration")
	}

	if err := AutoMigrateImportTaskRepository(db); err != nil {
		t.Fatalf("AutoMigrateImportTaskRepository() error = %v", err)
	}
	if !db.Migrator().HasColumn("listing_product_import_task", "processing_node") {
		t.Fatal("processing_node column missing after migration")
	}
	columnTypes, err := db.Migrator().ColumnTypes("listing_product_import_task")
	if err != nil {
		t.Fatalf("load column types: %v", err)
	}
	for _, columnType := range columnTypes {
		if columnType.Name() != "processing_node" {
			continue
		}
		if length, ok := columnType.Length(); !ok || length < 128 {
			t.Fatalf("processing_node length = %d, %v; want at least 128", length, ok)
		}
		return
	}
	t.Fatal("processing_node column type metadata missing after migration")
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

func setNullTargetPlatforms(t *testing.T, db *gorm.DB, ids ...int64) {
	t.Helper()
	if err := db.Table("listing_product_import_task").Where("id IN ?", ids).Update("target_platform", nil).Error; err != nil {
		t.Fatalf("set null target platforms: %v", err)
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
