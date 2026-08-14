package local

import (
	"errors"
	"testing"

	"task-processor/internal/listingadmin"
	api "task-processor/internal/taskrpcapi"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestLocalTaskRPCSubmitTaskPersistsSourceAndTargetPlatforms(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Table("listing_product_import_task").AutoMigrate(&localImportTaskRow{}); err != nil {
		t.Fatalf("migrate local task row: %v", err)
	}
	if err := listingadmin.AutoMigrateImportTaskRepository(db); err != nil {
		t.Fatalf("migrate import task repository: %v", err)
	}

	provider := &LocalTaskRPCProvider{db: db}
	if _, handled, err := provider.SubmitTask(&api.TaskSubmitReqDTO{
		TenantID:       246,
		StoreID:        986,
		Platform:       "legacy",
		SourcePlatform: "Amazon",
		TargetPlatform: "SHEIN",
		Region:         "US",
		ProductID:      "B0ROUTE",
	}, false); err != nil || !handled {
		t.Fatalf("SubmitTask() = handled:%v error:%v, want handled without error", handled, err)
	}

	var row localImportTaskRow
	if err := db.Table("listing_product_import_task").Where("product_id = ?", "B0ROUTE").Take(&row).Error; err != nil {
		t.Fatalf("load submitted task: %v", err)
	}
	if row.Platform != "legacy" || row.SourcePlatform != "amazon" || row.TargetPlatform != "shein" {
		t.Fatalf("persisted platforms = %q/%q/%q, want legacy/amazon/shein", row.Platform, row.SourcePlatform, row.TargetPlatform)
	}
}

func TestLocalTaskRPCSubmitTaskRejectsWritesWhenImportTaskUniquenessIsUnavailable(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Table("listing_product_import_task").AutoMigrate(&localImportTaskRow{}); err != nil {
		t.Fatalf("migrate local task row: %v", err)
	}
	for i, target := range []string{"SHEIN", "shein"} {
		if err := db.Table("listing_product_import_task").Create(&localImportTaskRow{
			ID:             int64(i + 1),
			TenantID:       246,
			StoreID:        986,
			Platform:       "amazon",
			SourcePlatform: "amazon",
			TargetPlatform: target,
			Region:         "US",
			ProductID:      "LEGACY-DUPLICATE",
			Status:         0,
			Deleted:        0,
		}).Error; err != nil {
			t.Fatalf("seed legacy duplicate %q: %v", target, err)
		}
	}
	if err := listingadmin.AutoMigrateImportTaskRepository(db); err != nil {
		t.Fatalf("migrate import task repository with duplicates: %v", err)
	}

	provider := &LocalTaskRPCProvider{db: db}
	if _, handled, err := provider.SubmitTask(&api.TaskSubmitReqDTO{
		TenantID:       246,
		StoreID:        986,
		Platform:       "Amazon",
		SourcePlatform: "Amazon",
		TargetPlatform: "SHEIN",
		Region:         "US",
		ProductID:      "NEW-TASK",
	}, false); !handled || !errors.Is(err, listingadmin.ErrImportTaskIntegrityUnavailable) {
		t.Fatalf("SubmitTask() = handled:%v error:%v, want integrity-unavailable rejection", handled, err)
	}
}
