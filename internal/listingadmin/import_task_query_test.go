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

func TestFindImportTaskRowsUsesRequestOwnerScope(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductImportTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []listingProductImportTask{
		{TenantID: 101, OwnerUserID: "user-a", StoreID: 11, Platform: "Amazon", Region: "US", CategoryID: 1, ProductID: "A-1", Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-b", StoreID: 11, Platform: "Amazon", Region: "US", CategoryID: 1, ProductID: "B-1", Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", StoreID: 11, Platform: "Amazon", Region: "US", CategoryID: 1, ProductID: "A-2", Status: 1, Deleted: 1},
	} {
		if err := db.Table("listing_product_import_task").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, total, page, pageSize, err := findImportTaskRows(ctx, db.Table("listing_product_import_task"), ImportTaskQuery{
		TenantID: 101,
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("findImportTaskRows: %v", err)
	}
	if total != 1 || page != 1 || pageSize != 20 {
		t.Fatalf("result meta = total:%d page:%d pageSize:%d, want 1/1/20", total, page, pageSize)
	}
	if len(rows) != 1 || rows[0].ProductID != "A-1" {
		t.Fatalf("rows = %+v, want only active owner-scoped row", rows)
	}
}

func TestFindImportTaskRowsAppliesResourceFilters(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductImportTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	storeID := int64(21)
	categoryID := int64(31)
	status := int16(2)
	for _, row := range []listingProductImportTask{
		{TenantID: 101, OwnerUserID: "user-a", StoreID: 21, Platform: "Amazon", Region: "US", CategoryID: 31, ProductID: "ABC-1", Status: 2, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", StoreID: 21, Platform: "Amazon", Region: "CA", CategoryID: 31, ProductID: "ABC-1", Status: 2, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", StoreID: 22, Platform: "Amazon", Region: "US", CategoryID: 31, ProductID: "XYZ-1", Status: 2, Deleted: 0},
	} {
		if err := db.Table("listing_product_import_task").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, total, _, _, err := findImportTaskRows(ctx, db.Table("listing_product_import_task"), ImportTaskQuery{
		TenantID:   101,
		StoreID:    &storeID,
		Platform:   "Amazon",
		Region:     "US",
		CategoryID: &categoryID,
		ProductID:  "ABC",
		Status:     &status,
	})
	if err != nil {
		t.Fatalf("findImportTaskRows: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].ProductID != "ABC-1" {
		t.Fatalf("rows = %+v total=%d, want only fully matched row", rows, total)
	}
}

func TestGormImportTaskRepositoryLifecycleHelpers(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductImportTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	for _, row := range []listingProductImportTask{
		{ID: 1, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "A", Status: model.TaskStatusPending.Int16(), Priority: 10, CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 2, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "B", Status: model.TaskStatusPendingRetry.Int16(), Priority: 20, CreateTime: &now, UpdateTime: &now, Deleted: 0},
		{ID: 3, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "C", Status: model.TaskStatusPublished.Int16(), Priority: 30, CreateTime: &now, UpdateTime: &now, Deleted: 0},
	} {
		if err := db.Table("listing_product_import_task").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	repo := NewGormImportTaskRepository(db)
	tasks, err := repo.ListPendingAndRetryTasks(context.Background(), 10, 10, []int64{100})
	if err != nil {
		t.Fatalf("ListPendingAndRetryTasks() error = %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("ListPendingAndRetryTasks() len = %d, want 2", len(tasks))
	}

	task, err := repo.GetImportTaskByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetImportTaskByID() error = %v", err)
	}
	if task == nil || task.ProductID != "A" {
		t.Fatalf("GetImportTaskByID() = %+v, want task A", task)
	}

	expected := model.TaskStatusPending.Int16()
	handled, err := repo.UpdateImportTaskStatus(context.Background(), &ImportTaskStatusUpdate{
		ID:                    1,
		Status:                model.TaskStatusProcessing.Int16(),
		ExpectedCurrentStatus: &expected,
	})
	if err != nil || !handled {
		t.Fatalf("UpdateImportTaskStatus() handled=%v err=%v", handled, err)
	}

	task, err = repo.GetImportTaskByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetImportTaskByID(updated) error = %v", err)
	}
	if task == nil || task.Status != model.TaskStatusProcessing.Int16() {
		t.Fatalf("updated task = %+v, want processing", task)
	}
}

func TestGormImportTaskRepositoryUpdateDraftSetsPublishedTime(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductImportTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().Add(-5 * time.Minute)
	row := listingProductImportTask{
		ID:         11,
		TenantID:   10,
		StoreID:    976,
		Platform:   "shein",
		Region:     "us",
		ProductID:  "draft-product",
		Status:     model.TaskStatusProcessing.Int16(),
		Priority:   10,
		CreateTime: &now,
		UpdateTime: &now,
		Deleted:    0,
	}
	if err := db.Table("listing_product_import_task").Create(&row).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}

	expected := model.TaskStatusProcessing.Int16()
	repo := NewGormImportTaskRepository(db)
	handled, err := repo.UpdateImportTaskStatus(context.Background(), &ImportTaskStatusUpdate{
		ID:                    row.ID,
		Status:                model.TaskStatusDraft.Int16(),
		ExpectedCurrentStatus: &expected,
	})
	if err != nil || !handled {
		t.Fatalf("UpdateImportTaskStatus() handled=%v err=%v", handled, err)
	}

	var updated listingProductImportTask
	if err := db.Table("listing_product_import_task").Where("id = ?", row.ID).Take(&updated).Error; err != nil {
		t.Fatalf("load updated row: %v", err)
	}
	if updated.Status != model.TaskStatusDraft.Int16() {
		t.Fatalf("status = %d, want draft", updated.Status)
	}
	if updated.PublishedTime == nil {
		t.Fatal("published_time is nil, want set when moving to draft")
	}
}

func TestGormImportTaskRepositoryRecoversTimedOutProcessingTasks(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductImportTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now()
	expired := now.Add(-45 * time.Minute)
	fresh := now.Add(-5 * time.Minute)
	rows := []listingProductImportTask{
		{ID: 101, TenantID: 10, StoreID: 976, Platform: "shein", Region: "us", ProductID: "expired", Status: model.TaskStatusProcessing.Int16(), Priority: 10, CreateTime: &expired, UpdateTime: &expired, Deleted: 0},
		{ID: 102, TenantID: 10, StoreID: 976, Platform: "shein", Region: "us", ProductID: "fresh", Status: model.TaskStatusProcessing.Int16(), Priority: 10, CreateTime: &fresh, UpdateTime: &fresh, Deleted: 0},
		{ID: 103, TenantID: 10, StoreID: 976, Platform: "shein", Region: "us", ProductID: "queued", Status: model.TaskStatusQueued.Int16(), Priority: 10, CreateTime: &expired, UpdateTime: &expired, Deleted: 0},
	}
	for _, row := range rows {
		if err := db.Table("listing_product_import_task").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	repo := NewGormImportTaskRepository(db)
	candidates, err := repo.ListTimedOutProcessingTasks(context.Background(), now.Add(-30*time.Minute), 10)
	if err != nil {
		t.Fatalf("ListTimedOutProcessingTasks() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].ID != 101 {
		t.Fatalf("candidates = %+v, want only expired processing task", candidates)
	}

	recovered, err := repo.RecoverTimedOutProcessingTasks(context.Background(), []int64{101, 102, 103}, ProcessingTimeoutRecovery{
		TimeoutMinutes: 30,
		ErrorMessage:   "Task processing lease expired, recovered by management watchdog",
		ReasonCode:     "PROCESSING_TIMEOUT",
		Stage:          "processing_timeout_recovery",
		Remark:         "Recovered after processing timeout watchdog (30 minutes)",
	})
	if err != nil {
		t.Fatalf("RecoverTimedOutProcessingTasks() error = %v", err)
	}
	if recovered != 1 {
		t.Fatalf("recovered = %d, want 1", recovered)
	}

	task, err := repo.GetImportTaskByID(context.Background(), 101)
	if err != nil {
		t.Fatalf("GetImportTaskByID(expired) error = %v", err)
	}
	if task == nil || task.Status != model.TaskStatusPendingRetry.Int16() || task.ReasonCode != "PROCESSING_TIMEOUT" || task.Stage != "processing_timeout_recovery" {
		t.Fatalf("expired task = %+v, want recovered retry with structured reason", task)
	}

	task, err = repo.GetImportTaskByID(context.Background(), 102)
	if err != nil {
		t.Fatalf("GetImportTaskByID(fresh) error = %v", err)
	}
	if task == nil || task.Status != model.TaskStatusProcessing.Int16() {
		t.Fatalf("fresh task = %+v, want still processing", task)
	}
}

func TestGormImportTaskRepositoryRecoverTimedOutProcessingTasksUsesExplicitCutoff(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductImportTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now()
	listedByCoordinator := now.Add(-15 * time.Minute)
	explicitCutoff := now.Add(-10 * time.Minute)
	row := listingProductImportTask{
		ID:         151,
		TenantID:   10,
		StoreID:    976,
		Platform:   "shein",
		Region:     "us",
		ProductID:  "listed-by-injected-clock",
		Status:     model.TaskStatusProcessing.Int16(),
		Priority:   10,
		CreateTime: &listedByCoordinator,
		UpdateTime: &listedByCoordinator,
		Deleted:    0,
	}
	if err := db.Table("listing_product_import_task").Create(&row).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}

	repo := NewGormImportTaskRepository(db)
	recovered, err := repo.RecoverTimedOutProcessingTasks(context.Background(), []int64{151}, ProcessingTimeoutRecovery{
		TimeoutMinutes: 30,
		TimeoutBefore:  explicitCutoff,
		ReasonCode:     "PROCESSING_TIMEOUT",
		Stage:          "processing_timeout_recovery",
	})
	if err != nil {
		t.Fatalf("RecoverTimedOutProcessingTasks() error = %v", err)
	}
	if recovered != 1 {
		t.Fatalf("recovered = %d, want 1 using explicit cutoff", recovered)
	}
}

func TestGormImportTaskRepositoryRecoversStaleQueuedTasks(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductImportTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now()
	expired := now.Add(-3 * time.Hour)
	fresh := now.Add(-5 * time.Minute)
	rows := []listingProductImportTask{
		{ID: 201, TenantID: 10, StoreID: 976, Platform: "shein", Region: "us", ProductID: "expired-queued", Status: model.TaskStatusQueued.Int16(), ProcessingNode: "dispatch-token-201", Priority: 10, CreateTime: &expired, UpdateTime: &expired, Deleted: 0},
		{ID: 202, TenantID: 10, StoreID: 976, Platform: "shein", Region: "us", ProductID: "fresh-queued", Status: model.TaskStatusQueued.Int16(), ProcessingNode: "dispatch-token-202", Priority: 10, CreateTime: &fresh, UpdateTime: &fresh, Deleted: 0},
		{ID: 203, TenantID: 10, StoreID: 976, Platform: "shein", Region: "us", ProductID: "processing", Status: model.TaskStatusProcessing.Int16(), Priority: 10, CreateTime: &expired, UpdateTime: &expired, Deleted: 0},
	}
	for _, row := range rows {
		if err := db.Table("listing_product_import_task").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	repo := NewGormImportTaskRepository(db)
	candidates, err := repo.ListStaleQueuedTasks(context.Background(), now.Add(-120*time.Minute), 10)
	if err != nil {
		t.Fatalf("ListStaleQueuedTasks() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].ID != 201 {
		t.Fatalf("candidates = %+v, want only expired queued task", candidates)
	}

	recovered, err := repo.RecoverStaleQueuedTasks(context.Background(), []int64{201, 202, 203}, StaleQueuedRecovery{
		TimeoutMinutes: 120,
		ErrorMessage:   "Task stayed queued too long, recovered by scheduler watchdog",
		ReasonCode:     "STALE_QUEUED",
		Stage:          "queued_timeout_recovery",
		Remark:         "Recovered from stale queued state by scheduler watchdog (120 minutes)",
	})
	if err != nil {
		t.Fatalf("RecoverStaleQueuedTasks() error = %v", err)
	}
	if recovered != 1 {
		t.Fatalf("recovered = %d, want 1", recovered)
	}

	task, err := repo.GetImportTaskByID(context.Background(), 201)
	if err != nil {
		t.Fatalf("GetImportTaskByID(expired queued) error = %v", err)
	}
	if task == nil || task.Status != model.TaskStatusPending.Int16() || task.ProcessingNode != "" || task.ReasonCode != "STALE_QUEUED" || task.Stage != "queued_timeout_recovery" {
		t.Fatalf("expired queued task = %+v, want recovered pending with cleared processing node and structured reason", task)
	}

	task, err = repo.GetImportTaskByID(context.Background(), 202)
	if err != nil {
		t.Fatalf("GetImportTaskByID(fresh queued) error = %v", err)
	}
	if task == nil || task.Status != model.TaskStatusQueued.Int16() || task.ProcessingNode != "dispatch-token-202" {
		t.Fatalf("fresh queued task = %+v, want still queued with processing node preserved", task)
	}
}

func TestGormImportTaskRepositoryRecoverStaleQueuedTasksUsesExplicitCutoff(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductImportTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now()
	listedByCoordinator := now.Add(-60 * time.Minute)
	explicitCutoff := now.Add(-30 * time.Minute)
	row := listingProductImportTask{
		ID:             251,
		TenantID:       10,
		StoreID:        976,
		Platform:       "shein",
		Region:         "us",
		ProductID:      "queued-by-injected-clock",
		Status:         model.TaskStatusQueued.Int16(),
		ProcessingNode: "dispatch-token-251",
		Priority:       10,
		CreateTime:     &listedByCoordinator,
		UpdateTime:     &listedByCoordinator,
		Deleted:        0,
	}
	if err := db.Table("listing_product_import_task").Create(&row).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}

	repo := NewGormImportTaskRepository(db)
	recovered, err := repo.RecoverStaleQueuedTasks(context.Background(), []int64{251}, StaleQueuedRecovery{
		TimeoutMinutes: 120,
		TimeoutBefore:  explicitCutoff,
		ReasonCode:     "STALE_QUEUED",
		Stage:          "queued_timeout_recovery",
	})
	if err != nil {
		t.Fatalf("RecoverStaleQueuedTasks() error = %v", err)
	}
	if recovered != 1 {
		t.Fatalf("recovered = %d, want 1 using explicit cutoff", recovered)
	}
}
