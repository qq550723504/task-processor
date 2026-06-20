package listingadmin

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
	"task-processor/internal/infra/clients/management/api"
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
	handled, err := repo.UpdateImportTaskStatus(context.Background(), &api.ProductImportTaskUpdateReqDTO{
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
