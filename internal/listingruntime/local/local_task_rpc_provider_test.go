package local

import (
	"testing"

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
