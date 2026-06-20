package sync

import (
	"testing"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pkg/types"
)

func TestTemuProductSnapshotToBatchSaveReq(t *testing.T) {
	snapshot := &TemuProductSnapshot{
		TenantID:          17,
		StoreID:           11,
		Platform:          "TEMU",
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
		PlatformProductID: "goods-1",
		PlatformStatus:    `{"shelf_status":"ON_SHELF"}`,
		ShelfStatus:       managementapi.ShelfStatusOnShelf,
	}

	item := snapshot.toProductDataItemDTO()
	if item.PlatformProductID != snapshot.PlatformProductID {
		t.Fatalf("item.PlatformProductID = %q, want %q", item.PlatformProductID, snapshot.PlatformProductID)
	}
	if item.ProductPrice != snapshot.OriginalPrice || item.ProductStock != snapshot.Stock {
		t.Fatalf("item price/stock mismatch: %#v", item)
	}

	req := snapshot.toBatchSaveReq([]managementapi.ProductDataItemDTO{item})
	if req.Platform != "TEMU" || req.TenantID != snapshot.TenantID || req.StoreID != snapshot.StoreID || req.Region != snapshot.Region {
		t.Fatalf("batch save request metadata mismatch: %#v", req)
	}
	if len(req.Products) != 1 || req.Products[0].PlatformProductID != snapshot.PlatformProductID {
		t.Fatalf("batch save request products mismatch: %#v", req.Products)
	}
}
