package extractor

import (
	"testing"

	"task-processor/internal/crawler/alibaba1688/model"
)

func TestValidateAndFixPriceRangesSortsAndFiltersRanges(t *testing.T) {
	extractor := NewPriceExtractor()
	product := &model.Product1688{
		PriceRanges: []model.PriceRange{
			{MinQuantity: 10, MaxQuantity: 0, Price: 9.9},
			{MinQuantity: 1, MaxQuantity: 9, Price: 12.5},
			{MinQuantity: 0, MaxQuantity: 5, Price: 8.8},
			{MinQuantity: 50, MaxQuantity: 0, Price: 7.5},
		},
	}

	extractor.validateAndFixPriceRanges(product)

	if len(product.PriceRanges) != 3 {
		t.Fatalf("expected 3 valid price ranges, got %d", len(product.PriceRanges))
	}

	if product.PriceRanges[0].MinQuantity != 1 {
		t.Fatalf("expected first range min quantity to be 1, got %d", product.PriceRanges[0].MinQuantity)
	}
	if product.PriceRanges[1].MinQuantity != 10 {
		t.Fatalf("expected second range min quantity to be 10, got %d", product.PriceRanges[1].MinQuantity)
	}
	if product.PriceRanges[2].MinQuantity != 50 {
		t.Fatalf("expected third range min quantity to be 50, got %d", product.PriceRanges[2].MinQuantity)
	}

	if product.MinPrice != 7.5 {
		t.Fatalf("expected min price to be 7.5, got %v", product.MinPrice)
	}
	if product.MaxPrice != 12.5 {
		t.Fatalf("expected max price to be 12.5, got %v", product.MaxPrice)
	}
}

func TestValidateAndFixPriceRangesCollapsesDuplicateMinQuantity(t *testing.T) {
	extractor := NewPriceExtractor()
	product := &model.Product1688{
		PriceRanges: []model.PriceRange{
			{MinQuantity: 300, MaxQuantity: 0, Price: 699},
			{MinQuantity: 300, MaxQuantity: 0, Price: 109},
		},
	}

	extractor.validateAndFixPriceRanges(product)

	if len(product.PriceRanges) != 1 {
		t.Fatalf("expected duplicate min quantity ranges to collapse to 1, got %d", len(product.PriceRanges))
	}
	if product.PriceRanges[0].MinQuantity != 300 {
		t.Fatalf("expected collapsed range min quantity to be 300, got %d", product.PriceRanges[0].MinQuantity)
	}
	if product.PriceRanges[0].Price != 109 {
		t.Fatalf("expected collapsed range to keep lowest price 109, got %v", product.PriceRanges[0].Price)
	}
	if product.MinPrice != 109 {
		t.Fatalf("expected min price to be 109, got %v", product.MinPrice)
	}
	if product.MaxPrice != 699 {
		t.Fatalf("expected max price to remain 699, got %v", product.MaxPrice)
	}
}
