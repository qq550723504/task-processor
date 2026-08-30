package listingadmin

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestGormProductDataAPIListByStoreKeepsExplicitZeroStoreScope(t *testing.T) {
	api := newGormProductDataAPIZeroStoreScopeTestAPI(t)

	items, err := api.ListByStore("temu", 7, 0, nil)
	if err != nil || len(items) != 1 || items[0].ID != 1 {
		t.Fatalf("ListByStore() = %#v, %v; want only zero-store product ID 1", items, err)
	}
}

func TestGormProductDataAPIPageByStoreKeepsExplicitZeroStoreScope(t *testing.T) {
	api := newGormProductDataAPIZeroStoreScopeTestAPI(t)

	page, err := api.PageProductDataByStore(&ProductDataListByStorePageReqDTO{
		TenantID: 7,
		StoreID:  0,
		Platform: "temu",
		PageNo:   1,
		PageSize: 20,
	})
	if err != nil || page == nil || len(page.List) != 1 || page.List[0].ID != 1 {
		t.Fatalf("PageProductDataByStore() = %#v, %v; want only zero-store product ID 1", page, err)
	}
}

func newGormProductDataAPIZeroStoreScopeTestAPI(t *testing.T) ProductDataAPI {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingProductData{}); err != nil {
		t.Fatalf("migrate product data: %v", err)
	}
	for _, product := range []listingProductData{
		{ID: 1, TenantID: 7, StoreID: 0, Platform: "temu", ProductID: "ZERO-STORE", Deleted: 0},
		{ID: 2, TenantID: 7, StoreID: 88, Platform: "temu", ProductID: "OTHER-STORE", Deleted: 0},
	} {
		if err := db.Table("listing_product_data").Create(&product).Error; err != nil {
			t.Fatalf("seed product %#v: %v", product, err)
		}
	}
	return NewGormProductDataAPI(NewGormProductDataRepository(db), 0)
}
