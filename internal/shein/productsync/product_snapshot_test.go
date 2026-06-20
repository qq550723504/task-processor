package productsync

import (
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/pkg/types"
)

func TestProductSnapshotToListingProductData(t *testing.T) {
	snapshot := &ProductSnapshot{
		TenantID:          17,
		StoreID:           11,
		Platform:          "SHEIN",
		CategoryID:        13,
		Region:            "US",
		ParentProductID:   "parent-1",
		ProductID:         "product-1",
		Title:             "Test Product",
		Description:       "desc",
		OriginalPrice:     types.FlexibleString("19.99"),
		SpecialPrice:      types.FlexibleString("15.99"),
		PriceCurrency:     "USD",
		Stock:             types.FlexibleString("8"),
		Brand:             "brand",
		Category:          "cat",
		MainImageURL:      "main",
		ImageURLs:         "img1,img2",
		Attributes:        `{"a":1}`,
		PlatformProductID: "spu-1",
		PlatformStatus:    `{"shelf_status":"ON_SHELF"}`,
		ShelfStatus:       2,
	}

	item := sheinProductDataFromSnapshot(snapshot)
	if item.PlatformProductID != snapshot.PlatformProductID {
		t.Fatalf("item.PlatformProductID = %q, want %q", item.PlatformProductID, snapshot.PlatformProductID)
	}
	if item.OriginalPrice != 19.99 || item.Stock != "8" {
		t.Fatalf("item price/stock mismatch: %#v", item)
	}
	if item.StoreID == nil || *item.StoreID != snapshot.StoreID {
		t.Fatalf("item.StoreID = %#v, want %d", item.StoreID, snapshot.StoreID)
	}
	if item.ShelfStatus == nil || *item.ShelfStatus != 2 {
		t.Fatalf("item.ShelfStatus = %#v, want 2", item.ShelfStatus)
	}
	var _ listingadmin.ProductData = item
}
