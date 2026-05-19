package listingkit

import (
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildSheinFinalReviewSKU(t *testing.T) {
	t.Parallel()

	sku := SheinSKUDraft{
		SupplierSKU: "SKU-1",
		BasePrice:   "12.50",
		Currency:    "USD",
		StockCount:  8,
		Weight:      0.3,
		SaleAttributes: []SheinResolvedSaleAttribute{
			{Name: "颜色", Value: "Black"},
			{Name: "尺码", Value: "One Size"},
		},
	}

	item := buildSheinFinalReviewSKU("SKC-1", sku)
	if item.SupplierCode != "SKC-1" || item.SupplierSKU != "SKU-1" {
		t.Fatalf("item = %+v", item)
	}
	if item.Color != "Black" || item.Size != "One Size" {
		t.Fatalf("item attrs = %+v", item)
	}
}

func TestResolveSheinFinalReviewImageRole(t *testing.T) {
	t.Parallel()

	role, main := resolveSheinFinalReviewImageRole(
		"https://cdn.example.com/size.jpg",
		"gallery",
		false,
		&sheinpub.FinalDraft{
			ImageRoleOverrides: map[string]string{
				"https://cdn.example.com/skc.jpg": "swatch",
			},
		},
		map[string]struct{}{"https://cdn.example.com/size.jpg": {}},
	)
	if role != "size_map" || main {
		t.Fatalf("role=%q main=%v", role, main)
	}
}

func TestMergeSheinFinalReviewImage(t *testing.T) {
	t.Parallel()

	image := &SheinFinalReviewImage{Role: "gallery"}
	mergeSheinFinalReviewImage(image, "main", true)
	if image.Role != "main" || !image.Main || image.Swatch || image.SizeMap {
		t.Fatalf("image = %+v", image)
	}
}
