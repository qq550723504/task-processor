package amazon

import (
	"errors"
	"testing"

	"task-processor/internal/model"
)

func TestProductResultValidatorValidateAcceptsCompleteProduct(t *testing.T) {
	validator := NewProductResultValidator()

	product := &model.Product{
		Asin:         "B001234567",
		Title:        "Demo Product",
		ImageURL:     "https://example.com/1.jpg",
		FinalPrice:   19.99,
		IsAvailable:  true,
		Availability: "In Stock",
	}

	if err := validator.Validate(product); err != nil {
		t.Fatalf("expected valid product, got error: %v", err)
	}
}

func TestProductResultValidatorValidateRejectsMissingCriticalFields(t *testing.T) {
	validator := NewProductResultValidator()

	product := &model.Product{
		Asin:         "B001234567",
		Title:        "Demo Product",
		IsAvailable:  true,
		Availability: "In Stock",
	}

	err := validator.Validate(product)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestProductResultValidatorValidateAllowsUnavailableProductWithoutPrice(t *testing.T) {
	validator := NewProductResultValidator()

	product := &model.Product{
		Asin:         "B001234567",
		Title:        "Unavailable Product",
		ImageURL:     "https://example.com/1.jpg",
		IsAvailable:  false,
		Availability: "Currently unavailable",
	}

	if err := validator.Validate(product); err != nil {
		t.Fatalf("expected unavailable product to pass validation, got error: %v", err)
	}
}

func TestProductResultValidatorValidateRejectsBrokenVariationPayload(t *testing.T) {
	validator := NewProductResultValidator()

	product := &model.Product{
		Asin:         "B001234567",
		Title:        "Variant Product",
		ImageURL:     "https://example.com/1.jpg",
		FinalPrice:   19.99,
		IsAvailable:  true,
		Availability: "In Stock",
		Variations: []model.Variation{
			{Name: "Color", Asin: "B001234568"},
		},
	}

	err := validator.Validate(product)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestClassifyFetchErrorMarksProductQualityAsRetryable(t *testing.T) {
	err := ClassifyFetchError(&ProductQualityError{Reasons: []string{"title is empty"}})
	if err == nil {
		t.Fatal("expected classified error")
	}
	if err.ErrorType() != FetchErrorTypeProductQuality {
		t.Fatalf("expected product_quality, got %s", err.ErrorType())
	}
	if !err.RetryableError() {
		t.Fatal("expected product quality error to be retryable")
	}
}

func TestIsProductQualityError(t *testing.T) {
	if !isProductQualityError(&ProductQualityError{Reasons: []string{"title is empty"}}) {
		t.Fatal("expected product quality error to be detected")
	}
	if isProductQualityError(errors.New("other")) {
		t.Fatal("did not expect generic error to be treated as product quality error")
	}
}
