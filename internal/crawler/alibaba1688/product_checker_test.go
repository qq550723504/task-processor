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

func TestValidateProductRejectsMissingRequiredFields(t *testing.T) {
	checker := NewProductChecker()
	product := &model.Product1688{
		URL:       "https://detail.1688.com/offer/1.html",
		MainImage: "https://img.example/product.jpg",
	}

	if err := checker.ValidateProduct(product); err == nil {
		t.Fatal("ValidateProduct() error = nil, want missing required fields error")
	}
}

func TestValidateProductAcceptsCompleteRequiredFields(t *testing.T) {
	checker := NewProductChecker()
	product := &model.Product1688{
		URL:              "https://detail.1688.com/offer/1.html",
		Title:            "Insulated lunch bag",
		MinPrice:         18.8,
		MinOrderQuantity: 1,
		Supplier:         model.SupplierInfo{Name: "Lunch Factory"},
		MainImage:        "https://img.example/product.jpg",
	}

	if err := checker.ValidateProduct(product); err != nil {
		t.Fatalf("ValidateProduct() error = %v, want complete product accepted", err)
	}
}

func TestValidateProductRejectsHostlessOptionalImageURL(t *testing.T) {
	checker := NewProductChecker()
	product := &model.Product1688{
		URL:              "https://detail.1688.com/offer/1.html",
		Title:            "Insulated lunch bag",
		MinPrice:         18.8,
		MinOrderQuantity: 1,
		Supplier:         model.SupplierInfo{Name: "Lunch Factory"},
		MainImage:        "https://img.example/product.jpg",
		Variants:         []model.Variant{{Image: "https:///.jpg"}},
	}

	if err := checker.ValidateProduct(product); err == nil {
		t.Fatal("ValidateProduct() error = nil, want hostless optional image URL rejected")
	}
}
