package local

import (
	"testing"

	"task-processor/internal/listingadmin"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestLocalDataProviderBatchProductDataDerivesOwnerFromStore(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Table("listing_store").AutoMigrate(&localListingStore{}); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	if err := db.Table("listing_store").Create(&localListingStore{ID: 986, TenantID: 246, OwnerUserID: "store-owner"}).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}
	if err := db.Table("listing_product_data").AutoMigrate(&listingadmin.ProductData{}); err != nil {
		t.Fatalf("create product data table: %v", err)
	}
	for _, column := range []string{"creator", "updater", "deleted"} {
		if err := db.Exec("ALTER TABLE listing_product_data ADD COLUMN " + column + " varchar(128)").Error; err != nil {
			t.Fatalf("add product audit column %s: %v", column, err)
		}
	}
	if err := listingadmin.AutoMigrateProductDataRepository(db); err != nil {
		t.Fatalf("migrate product data: %v", err)
	}

	provider := &LocalDataProvider{db: db}
	count, err := provider.BatchCreateOrUpdateProductData(&listingadmin.ProductDataBatchSaveReqDTO{
		Platform: "SHEIN",
		TenantID: 246,
		Region:   "US",
		StoreID:  986,
		Products: []listingadmin.ProductDataItemDTO{{
			PlatformProductID: "SP-1",
			ProductName:       "Product",
			ProductPrice:      "1.00",
			ProductStock:      "1",
		}},
	})
	if err != nil || count != 1 {
		t.Fatalf("BatchCreateOrUpdateProductData() = count:%d error:%v, want one persisted row", count, err)
	}
	var owner string
	if err := db.Table("listing_product_data").Pluck("owner_user_id", &owner).Error; err != nil {
		t.Fatalf("load product owner: %v", err)
	}
	if owner != "store-owner" {
		t.Fatalf("persisted owner = %q, want store-owner", owner)
	}
}
