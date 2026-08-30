package local

import (
	"testing"
	"time"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	"task-processor/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestLocalRuntimeReadsImportTaskFromResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeImportTaskTestDB(t)
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	if err := db.Table("listing_product_import_task").Create(&localImportTaskRow{
		ID:             702,
		TenantID:       246,
		StoreID:        986,
		Platform:       "shein",
		SourcePlatform: "amazon",
		TargetPlatform: "shein",
		Region:         "us",
		ProductID:      "RESOURCE-ONLY",
		Status:         model.TaskStatusPending.Int16(),
		RetryCount:     2,
		Priority:       8,
		CreateTime:     now,
		UpdateTime:     now,
	}).Error; err != nil {
		t.Fatalf("seed import task: %v", err)
	}

	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	task, err := runtime.GetRuntimeImportTask(702)
	if err != nil {
		t.Fatalf("GetRuntimeImportTask() error: %v", err)
	}
	if task == nil {
		t.Fatal("GetRuntimeImportTask() returned nil task")
	}
	if task.ID != 702 || task.StoreID != 986 || task.ProductID != "RESOURCE-ONLY" || task.Status != model.TaskStatusPending.Int16() || task.RetryCount != 2 || task.Priority != 8 {
		t.Fatalf("GetRuntimeImportTask() = %#v, want persisted runtime task", task)
	}
}

func TestLocalRuntimeUpdatesImportTaskFromResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeImportTaskTestDB(t)
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	if err := db.Table("listing_product_import_task").Create(&localImportTaskRow{
		ID:         703,
		TenantID:   246,
		StoreID:    986,
		Platform:   "shein",
		Region:     "us",
		ProductID:  "RESOURCE-UPDATE",
		Status:     model.TaskStatusPending.Int16(),
		RetryCount: 1,
		Priority:   3,
		CreateTime: now,
		UpdateTime: now,
	}).Error; err != nil {
		t.Fatalf("seed import task: %v", err)
	}

	expectedStatus := model.TaskStatusPending.Int16()
	retryCount := 4
	priority := 9
	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	err := runtime.UpdateRuntimeTaskStatus(&listingruntime.TaskStatusUpdate{
		ID:                    703,
		Status:                model.TaskStatusProcessing.Int16(),
		ErrorMessage:          "processing",
		ReasonCode:            "WORKER_CLAIMED",
		Stage:                 "dispatch",
		ExpectedCurrentStatus: &expectedStatus,
		RetryCount:            &retryCount,
		Priority:              &priority,
	})
	if err != nil {
		t.Fatalf("UpdateRuntimeTaskStatus() error: %v", err)
	}

	var row localImportTaskRow
	if err := db.Table("listing_product_import_task").Where("id = ?", 703).Take(&row).Error; err != nil {
		t.Fatalf("load updated import task: %v", err)
	}
	if row.Status != model.TaskStatusProcessing.Int16() || row.ErrorMessage != "processing" || row.ReasonCode != "WORKER_CLAIMED" || row.Stage != "dispatch" || row.RetryCount != 4 || row.Priority != 9 {
		t.Fatalf("updated import task = %#v, want runtime status update", row)
	}
}

func TestLocalDataProviderImportTaskCompatibilityUsesResources(t *testing.T) {
	db := newLocalRuntimeImportTaskTestDB(t)
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	if err := db.Table("listing_product_import_task").Create(&localImportTaskRow{
		ID: 704, TenantID: 246, StoreID: 986, Platform: "shein", Region: "us", ProductID: "PROVIDER-COMPAT", Status: model.TaskStatusPending.Int16(), Priority: 3, CreateTime: now, UpdateTime: now,
	}).Error; err != nil {
		t.Fatalf("seed import task: %v", err)
	}
	provider := NewLocalDataProviderFromResources(NewRuntimeResources(db, nil))
	t.Cleanup(func() { _ = provider.Close() })

	task, found, err := provider.GetImportTaskByID(704)
	if err != nil || !found || task == nil || task.ID != 704 || task.Status != model.TaskStatusPending.Int16() {
		t.Fatalf("GetImportTaskByID() = %#v, %t, %v; want found pending task", task, found, err)
	}
	tasks, found, err := provider.GetPendingAndRetryTasks(10, 246, []int64{986})
	if err != nil || !found || len(tasks) != 1 || tasks[0].ID != 704 {
		t.Fatalf("GetPendingAndRetryTasks() = %#v, %t, %v; want one pending task", tasks, found, err)
	}
	expected := model.TaskStatusPending.Int16()
	updated, err := provider.UpdateImportTaskStatus(&listingadmin.ImportTaskStatusUpdate{ID: 704, Status: model.TaskStatusProcessing.Int16(), ExpectedCurrentStatus: &expected})
	if err != nil || !updated {
		t.Fatalf("UpdateImportTaskStatus() = %t, %v; want conditional update", updated, err)
	}
	task, found, err = provider.GetImportTaskByID(704)
	if err != nil || !found || task == nil || task.Status != model.TaskStatusProcessing.Int16() {
		t.Fatalf("updated GetImportTaskByID() = %#v, %t, %v; want processing task", task, found, err)
	}
}

func newLocalRuntimeImportTaskTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Table("listing_product_import_task").AutoMigrate(&localImportTaskRow{}); err != nil {
		t.Fatalf("migrate import task row: %v", err)
	}
	if err := listingadmin.AutoMigrateImportTaskRepository(db); err != nil {
		t.Fatalf("migrate import task repository: %v", err)
	}
	return db
}
