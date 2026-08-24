package local

import (
	"errors"
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/model"
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
	seedLocalTaskStore(t, db, 246, 986, "store-owner")
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

func TestLocalTaskRPCSubmitTaskDerivesOwnerFromStoreAndUsesRepository(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Table("listing_store").AutoMigrate(&localListingStore{}); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	if err := db.Table("listing_store").Create(&localListingStore{
		ID:          986,
		TenantID:    246,
		OwnerUserID: "store-owner",
	}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
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
		Platform:       "Amazon",
		SourcePlatform: "Amazon",
		TargetPlatform: "SHEIN",
		Region:         "US",
		ProductID:      "OWNER-DERIVED",
	}, false); err != nil || !handled {
		t.Fatalf("SubmitTask() = handled:%v error:%v, want handled without error", handled, err)
	}

	var owner string
	if err := db.Table("listing_product_import_task").Where("product_id = ?", "OWNER-DERIVED").Pluck("owner_user_id", &owner).Error; err != nil {
		t.Fatalf("load submitted task owner: %v", err)
	}
	if owner != "store-owner" {
		t.Fatalf("persisted owner = %q, want store-owner", owner)
	}
}

func TestLocalTaskRPCSubmitTaskPreservesCompletedDuplicateClassification(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	seedLocalTaskStore(t, db, 246, 986, "store-owner")
	if err := db.Table("listing_product_import_task").AutoMigrate(&localImportTaskRow{}); err != nil {
		t.Fatalf("migrate local task row: %v", err)
	}
	if err := listingadmin.AutoMigrateImportTaskRepository(db); err != nil {
		t.Fatalf("migrate import task repository: %v", err)
	}
	if err := db.Table("listing_product_import_task").Create(&localImportTaskRow{
		ID:             991,
		TenantID:       246,
		StoreID:        986,
		Platform:       "amazon",
		SourcePlatform: "amazon",
		TargetPlatform: "shein",
		Region:         "US",
		ProductID:      "ALREADY-PUBLISHED",
		Status:         model.TaskStatusPublished.Int16(),
		Deleted:        0,
	}).Error; err != nil {
		t.Fatalf("seed completed import task: %v", err)
	}

	provider := &LocalTaskRPCProvider{db: db}
	if _, handled, err := provider.SubmitTask(&api.TaskSubmitReqDTO{
		TenantID:       246,
		StoreID:        986,
		Platform:       "Amazon",
		SourcePlatform: "Amazon",
		TargetPlatform: "SHEIN",
		Region:         "US",
		ProductID:      "ALREADY-PUBLISHED",
	}, false); !handled || !errors.Is(err, listingadmin.ErrImportTaskAlreadyExists) {
		t.Fatalf("SubmitTask() = handled:%v error:%v, want completed-duplicate classification", handled, err)
	}
}

func TestLocalTaskRPCSubmitTaskRejectsStoreWithoutOwner(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Table("listing_store").AutoMigrate(&localListingStore{}); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	if err := db.Table("listing_store").Create(&localListingStore{ID: 986, TenantID: 246}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}
	if err := db.Table("listing_product_import_task").AutoMigrate(&localImportTaskRow{}); err != nil {
		t.Fatalf("migrate local task row: %v", err)
	}
	if err := listingadmin.AutoMigrateImportTaskRepository(db); err != nil {
		t.Fatalf("migrate import task repository: %v", err)
	}

	provider := &LocalTaskRPCProvider{db: db}
	if _, handled, err := provider.SubmitTask(&api.TaskSubmitReqDTO{
		TenantID:  246,
		StoreID:   986,
		Platform:  "Amazon",
		Region:    "US",
		ProductID: "OWNER-MISSING",
	}, false); !handled || !errors.Is(err, listingadmin.ErrOwnerUserIDRequired) {
		t.Fatalf("SubmitTask() = handled:%v error:%v, want owner-required rejection", handled, err)
	}
	var count int64
	if err := db.Table("listing_product_import_task").Count(&count).Error; err != nil {
		t.Fatalf("count submitted tasks: %v", err)
	}
	if count != 0 {
		t.Fatalf("submitted task rows = %d, want 0", count)
	}
}

func TestLocalTaskRPCSubmitTaskRejectsWritesWhenImportTaskUniquenessIsUnavailable(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	seedLocalTaskStore(t, db, 246, 986, "store-owner")
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

func TestLocalTaskRPCSubmitBatchPropagatesIntegrityUnavailable(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	seedLocalTaskStore(t, db, 246, 986, "store-owner")
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
			ProductID:      "LEGACY-BATCH-DUPLICATE",
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
	resp, handled, err := provider.SubmitBatchTasks(&api.TaskBatchSubmitReqDTO{Tasks: []api.TaskSubmitReqDTO{{
		TenantID:       246,
		StoreID:        986,
		Platform:       "Amazon",
		SourcePlatform: "Amazon",
		TargetPlatform: "SHEIN",
		Region:         "US",
		ProductID:      "NEW-BATCH-TASK",
	}}})
	if !handled || !errors.Is(err, listingadmin.ErrImportTaskIntegrityUnavailable) || resp != nil {
		t.Fatalf("SubmitBatchTasks() = response:%+v handled:%v error:%v, want propagated integrity-unavailable error", resp, handled, err)
	}
}

func seedLocalTaskStore(t *testing.T, db *gorm.DB, tenantID, storeID int64, owner string) {
	t.Helper()
	if err := db.Table("listing_store").AutoMigrate(&localListingStore{}); err != nil {
		t.Fatalf("migrate local store: %v", err)
	}
	if err := db.Table("listing_store").Create(&localListingStore{
		ID:          storeID,
		TenantID:    tenantID,
		OwnerUserID: owner,
	}).Error; err != nil {
		t.Fatalf("seed local store: %v", err)
	}
}
