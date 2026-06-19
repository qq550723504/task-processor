package sale

import (
	"testing"

	"task-processor/internal/model"
	sheinattr "task-processor/internal/shein/product/attribute"
)

func TestCachedSaleSpecMatchesCurrentVariantsRejectsUnknownASIN(t *testing.T) {
	handler := NewSaleAttributeHandler(nil)

	currentVariants := []model.Product{
		{Asin: "A1"},
		{Asin: "A2"},
	}
	cached := sheinattr.ResultSaleAttribute{
		Variants: []sheinattr.Variant{
			{ASIN: "A1"},
			{ASIN: "A3"},
		},
	}

	if handler.cachedSaleSpecMatchesCurrentVariants(&currentVariants, cached) {
		t.Fatal("cachedSaleSpecMatchesCurrentVariants() = true, want false when cached result contains unknown ASIN")
	}
}

func TestCachedSaleSpecMatchesCurrentVariantsAcceptsSameASINSet(t *testing.T) {
	handler := NewSaleAttributeHandler(nil)

	currentVariants := []model.Product{
		{Asin: "A1"},
		{Asin: "A2"},
	}
	cached := sheinattr.ResultSaleAttribute{
		Variants: []sheinattr.Variant{
			{ASIN: "A2"},
			{ASIN: "A1"},
		},
	}

	if !handler.cachedSaleSpecMatchesCurrentVariants(&currentVariants, cached) {
		t.Fatal("cachedSaleSpecMatchesCurrentVariants() = false, want true for identical ASIN set")
	}
}
