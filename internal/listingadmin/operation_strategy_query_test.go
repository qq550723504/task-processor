package listingadmin

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestFindOperationStrategyRowsUsesRequestOwnerScope(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingOperationStrategy{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []listingOperationStrategy{
		{TenantID: 101, OwnerUserID: "user-a", StoreID: 11, Name: "keep", Platform: "SHEIN", Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-b", StoreID: 11, Name: "other-owner", Platform: "SHEIN", Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", StoreID: 11, Name: "deleted", Platform: "SHEIN", Status: 1, Deleted: 1},
	} {
		if err := db.Table("listing_operation_strategy").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, total, page, pageSize, err := findOperationStrategyRows(ctx, db.Table("listing_operation_strategy"), OperationStrategyQuery{
		TenantID: 101,
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("findOperationStrategyRows: %v", err)
	}
	if total != 1 || page != 1 || pageSize != 20 {
		t.Fatalf("result meta = total:%d page:%d pageSize:%d, want 1/1/20", total, page, pageSize)
	}
	if len(rows) != 1 || rows[0].Name != "keep" {
		t.Fatalf("rows = %+v, want only active owner-scoped row", rows)
	}
}

func TestFindOperationStrategyRowsAppliesResourceFilters(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingOperationStrategy{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	storeID := int64(21)
	status := int16(2)
	for _, row := range []listingOperationStrategy{
		{TenantID: 101, OwnerUserID: "user-a", StoreID: 21, Name: "Alpha Daily", Platform: "SHEIN", Status: 2, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", StoreID: 21, Name: "Alpha Daily", Platform: "AMZ", Status: 2, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", StoreID: 22, Name: "Beta Daily", Platform: "SHEIN", Status: 2, Deleted: 0},
	} {
		if err := db.Table("listing_operation_strategy").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, total, _, _, err := findOperationStrategyRows(ctx, db.Table("listing_operation_strategy"), OperationStrategyQuery{
		TenantID: 101,
		Name:     "Alpha",
		StoreID:  &storeID,
		Platform: "SHEIN",
		Status:   &status,
	})
	if err != nil {
		t.Fatalf("findOperationStrategyRows: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].Platform != "SHEIN" {
		t.Fatalf("rows = %+v total=%d, want only fully matched SHEIN row", rows, total)
	}
}

func TestGormOperationStrategyRepositoryGetLatestByStoreIDReturnsNewestRow(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingOperationStrategy{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []listingOperationStrategy{
		{TenantID: 101, StoreID: 21, Name: "old", Platform: "SHEIN", Status: 1, Deleted: 0},
		{TenantID: 101, StoreID: 21, Name: "new", Platform: "SHEIN", Status: 0, Deleted: 0, ActivityEnabled: 1, RestoreStockAmount: 5},
	} {
		if err := db.Table("listing_operation_strategy").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	repo := NewGormOperationStrategyRepository(db)
	got, err := repo.GetLatestByStoreID(context.Background(), 21)
	if err != nil {
		t.Fatalf("GetLatestByStoreID() error = %v", err)
	}
	if got == nil || got.Name != "new" || !got.ActivityEnabled || got.RestoreStockAmount == nil || *got.RestoreStockAmount != 5 {
		t.Fatalf("GetLatestByStoreID() = %+v, want newest mapped row", got)
	}
}

func TestGormOperationStrategyRepositoryGetActiveActivityStrategyFiltersEnrollmentScope(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingOperationStrategy{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []listingOperationStrategy{
		{TenantID: 101, StoreID: 21, Name: "correct", Platform: "SHEIN", Status: 0, Deleted: 0, ActivityEnabled: 1, ActivityType: "PROMOTION", ActivityDiscountRate: 0.2},
		{TenantID: 101, StoreID: 21, Name: "wrong-type-newer", Platform: "SHEIN", Status: 0, Deleted: 0, ActivityEnabled: 1, ActivityType: "FLASH_SALE", ActivityDiscountRate: 0.3},
		{TenantID: 101, StoreID: 21, Name: "disabled-newer", Platform: "SHEIN", Status: 1, Deleted: 0, ActivityEnabled: 1, ActivityType: "PROMOTION", ActivityDiscountRate: 0.4},
		{TenantID: 101, StoreID: 21, Name: "not-enabled-newer", Platform: "SHEIN", Status: 0, Deleted: 0, ActivityEnabled: 0, ActivityType: "PROMOTION", ActivityDiscountRate: 0.5},
		{TenantID: 101, StoreID: 21, Name: "wrong-platform-newer", Platform: "TEMU", Status: 0, Deleted: 0, ActivityEnabled: 1, ActivityType: "PROMOTION", ActivityDiscountRate: 0.6},
		{TenantID: 202, StoreID: 21, Name: "wrong-tenant-newer", Platform: "SHEIN", Status: 0, Deleted: 0, ActivityEnabled: 1, ActivityType: "PROMOTION", ActivityDiscountRate: 0.7},
	} {
		if err := db.Table("listing_operation_strategy").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	repo := NewGormOperationStrategyRepository(db)
	got, err := repo.GetActiveActivityStrategy(context.Background(), 101, 21, "SHEIN", "PROMOTION")
	if err != nil {
		t.Fatalf("GetActiveActivityStrategy() error = %v", err)
	}
	if got == nil || got.Name != "correct" || got.ActivityDiscountRate == nil || *got.ActivityDiscountRate != 0.2 {
		t.Fatalf("GetActiveActivityStrategy() = %+v, want correct scoped strategy", got)
	}
}
