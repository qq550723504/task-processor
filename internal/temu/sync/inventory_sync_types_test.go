package sync

import (
	"testing"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pkg/types"
)

func TestTemuInventoryProductSnapshotFromDTOAndBatchSaveReq(t *testing.T) {
	dto := &managementapi.ProductDataDTO{
		ID:                7,
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
		TenantID:          17,
	}

	snapshot := temuInventoryProductSnapshotFromDTO(dto)
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	if snapshot.ProductID != dto.ProductID || snapshot.PlatformProductID != dto.PlatformProductID {
		t.Fatalf("snapshot fields not copied: %#v", snapshot)
	}

	item := snapshot.toProductDataItemDTO()
	if item.PlatformProductID != dto.PlatformProductID {
		t.Fatalf("item.PlatformProductID = %q, want %q", item.PlatformProductID, dto.PlatformProductID)
	}
	if item.ProductPrice != dto.OriginalPrice || item.ProductStock != dto.Stock {
		t.Fatalf("item price/stock mismatch: %#v", item)
	}

	req := snapshot.toBatchSaveReq()
	if req.Platform != "TEMU" || req.TenantID != dto.TenantID || req.StoreID != dto.StoreID || req.Region != dto.Region {
		t.Fatalf("batch save request metadata mismatch: %#v", req)
	}
	if len(req.Products) != 1 || req.Products[0].PlatformProductID != dto.PlatformProductID {
		t.Fatalf("batch save request products mismatch: %#v", req.Products)
	}
}
