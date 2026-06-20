package sync

import (
	"testing"
	"time"

	"task-processor/internal/pkg/types"
)

func TestTemuProductDataFromSnapshot(t *testing.T) {
	now := time.Unix(1700000000, 0)
	snapshot := &TemuProductSnapshot{
		TenantID:          1,
		StoreID:           2,
		Platform:          "TEMU",
		Region:            "US",
		ParentProductID:   "parent",
		ProductID:         "product",
		Title:             "title",
		Description:       "desc",
		OriginalPrice:     "12.34",
		SpecialPrice:      "10.01",
		PriceCurrency:     "USD",
		Stock:             "8",
		Brand:             "brand",
		Category:          "cat",
		MainImageURL:      "img",
		ImageURLs:         `["a"]`,
		Attributes:        `{"k":"v"}`,
		PlatformStatus:    `{"s":"on"}`,
		PlatformData:      `{"p":1}`,
		PlatformProductID: "ppid",
		ShelfStatus:       2,
		CategoryID:        3,
		PublishTime:       types.ToFlexibleTime(&now),
	}

	item := temuProductDataFromSnapshot(snapshot)
	if item.TenantID != 1 || item.PlatformProductID != "ppid" {
		t.Fatalf("unexpected item: %#v", item)
	}
	if item.OriginalPrice != 12.34 || item.SpecialPrice != 10.01 {
		t.Fatalf("unexpected prices: %#v", item)
	}
	if item.PublishTime == nil || !item.PublishTime.Equal(now) {
		t.Fatalf("unexpected publish time: %#v", item.PublishTime)
	}
}
