package listingadmin

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestFindProductDataRowsUsesRequestOwnerScope(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductData{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []listingProductData{
		{TenantID: 101, OwnerUserID: "user-a", ProductID: "A-1", Title: "kept", Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-b", ProductID: "B-1", Title: "other-owner", Status: 1, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", ProductID: "A-2", Title: "deleted", Status: 1, Deleted: 1},
	} {
		if err := db.Table("listing_product_data").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, total, page, pageSize, err := findProductDataRows(ctx, db.Table("listing_product_data"), ProductDataQuery{
		TenantID: 101,
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("findProductDataRows: %v", err)
	}
	if total != 1 || page != 1 || pageSize != 20 {
		t.Fatalf("result meta = total:%d page:%d pageSize:%d, want 1/1/20", total, page, pageSize)
	}
	if len(rows) != 1 || rows[0].ProductID != "A-1" {
		t.Fatalf("rows = %+v, want only active owner-scoped row", rows)
	}
}

func TestFindProductDataRowsAppliesResourceFilters(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductData{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	shelfStatus := 2
	status := int16(3)
	for _, row := range []listingProductData{
		{TenantID: 101, OwnerUserID: "user-a", ProductID: "A-1", Title: "alpha phone", Brand: "BrandA", Platform: "SHEIN", ShelfStatus: 2, Status: 3, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", ProductID: "A-2", Title: "alpha phone", Brand: "BrandB", Platform: "SHEIN", ShelfStatus: 2, Status: 3, Deleted: 0},
		{TenantID: 101, OwnerUserID: "user-a", ProductID: "A-3", Title: "beta phone", Brand: "BrandA", Platform: "AMZ", ShelfStatus: 2, Status: 3, Deleted: 0},
	} {
		if err := db.Table("listing_product_data").Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	t.Cleanup(SetOwnerScopeRequiredForTesting(true))
	ctx := withRequestIdentity(context.TODO(), "user-a", nil)

	rows, total, _, _, err := findProductDataRows(ctx, db.Table("listing_product_data"), ProductDataQuery{
		TenantID:    101,
		Title:       "alpha",
		Brand:       "BrandA",
		Platform:    "SHEIN",
		ShelfStatus: &shelfStatus,
		Status:      &status,
	})
	if err != nil {
		t.Fatalf("findProductDataRows: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].ProductID != "A-1" {
		t.Fatalf("rows = %+v total=%d, want only BrandA SHEIN alpha row", rows, total)
	}
}

func TestGormProductDataRepositoryBatchOperations(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductData{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewGormProductDataRepository(db)
	storeID := int64(11)
	count, err := repo.UpsertProductDataBatch(context.Background(), []ProductData{
		{
			TenantID:          101,
			StoreID:           &storeID,
			Platform:          "SHEIN",
			Region:            "US",
			ProductID:         "SKU-1",
			Title:             "Alpha",
			PlatformProductID: "SP-1",
			Attributes:        []byte(`{"size":"M"}`),
		},
	})
	if err != nil {
		t.Fatalf("UpsertProductDataBatch() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("UpsertProductDataBatch() count = %d, want 1", count)
	}

	count, err = repo.UpsertProductDataBatch(context.Background(), []ProductData{
		{
			TenantID:          101,
			StoreID:           &storeID,
			Platform:          "SHEIN",
			Region:            "US",
			ProductID:         "SKU-1",
			Title:             "Alpha Updated",
			PlatformProductID: "SP-1",
			Attributes:        []byte(`{"size":"L"}`),
		},
	})
	if err != nil {
		t.Fatalf("UpsertProductDataBatch(update) error = %v", err)
	}
	if count != 1 {
		t.Fatalf("UpsertProductDataBatch(update) count = %d, want 1", count)
	}

	updated, err := repo.BatchUpdateAttributesByPlatformProductID(context.Background(), []ProductData{
		{
			TenantID:          101,
			StoreID:           &storeID,
			Platform:          "SHEIN",
			PlatformProductID: "SP-1",
			Attributes:        []byte(`{"size":"XL"}`),
		},
	})
	if err != nil {
		t.Fatalf("BatchUpdateAttributesByPlatformProductID() error = %v", err)
	}
	if updated != 1 {
		t.Fatalf("BatchUpdateAttributesByPlatformProductID() updated = %d, want 1", updated)
	}
}
