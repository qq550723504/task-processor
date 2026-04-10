package product

import (
	"testing"

	"task-processor/internal/model"
)

func TestGetProductPriceOriginalUsesListPriceFirst(t *testing.T) {
	listPrice := 29.99
	product := &model.Product{
		InitialPrice: 19.99,
		FinalPrice:   9.99,
		PricesBreakdown: model.PriceBreakdown{
			ListPrice: &listPrice,
		},
	}

	price := GetProductPrice(product, "original")
	if price != 29.99 {
		t.Fatalf("GetProductPrice(original) = %v, want 29.99", price)
	}
}

func TestGetProductPriceOriginalFallsBackToInitialPrice(t *testing.T) {
	product := &model.Product{
		InitialPrice: 19.99,
		FinalPrice:   9.99,
	}

	price := GetProductPrice(product, "original")
	if price != 19.99 {
		t.Fatalf("GetProductPrice(original) = %v, want 19.99", price)
	}
}

func TestGetProductPriceOriginalFallsBackToFinalPriceWhenOriginalMissing(t *testing.T) {
	product := &model.Product{
		InitialPrice: 0,
		FinalPrice:   6.19,
	}

	price := GetProductPrice(product, "original")
	if price != 6.19 {
		t.Fatalf("GetProductPrice(original) = %v, want 6.19", price)
	}
}
