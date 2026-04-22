package alibaba1688

import (
	"testing"

	"task-processor/internal/crawler/alibaba1688/model"
)

func TestValidatePricingAcceptsAscendingPriceRanges(t *testing.T) {
	checker := NewProductChecker()
	product := &model.Product1688{
		MinPrice:         7.5,
		MaxPrice:         12.5,
		MinOrderQuantity: 1,
		PriceRanges: []model.PriceRange{
			{MinQuantity: 1, MaxQuantity: 9, Price: 12.5},
			{MinQuantity: 10, MaxQuantity: 49, Price: 9.9},
			{MinQuantity: 50, MaxQuantity: 0, Price: 7.5},
		},
	}

	if err := checker.validatePricing(product); err != nil {
		t.Fatalf("expected sorted price ranges to pass validation, got %v", err)
	}
}

func TestValidatePricingRejectsDuplicateMinQuantity(t *testing.T) {
	checker := NewProductChecker()
	product := &model.Product1688{
		MinPrice:         7.5,
		MaxPrice:         12.5,
		MinOrderQuantity: 1,
		PriceRanges: []model.PriceRange{
			{MinQuantity: 1, MaxQuantity: 9, Price: 12.5},
			{MinQuantity: 1, MaxQuantity: 0, Price: 9.9},
		},
	}

	if err := checker.validatePricing(product); err == nil {
		t.Fatal("expected duplicate min quantity to fail validation")
	}
}
