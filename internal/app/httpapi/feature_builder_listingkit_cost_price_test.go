package httpapi

import (
	"testing"

	"task-processor/internal/listingkit"
)

func TestMarketplaceSheinCostPriceComposition(t *testing.T) {
	input := listingkit.SheinCostPriceInput{
		ExchangeRate: 8, MarkupMultiplier: 2, MinimumPrice: 9,
		RoundTo: 0.5, PriceEnding: 0.49,
	}
	if got := marketplaceSheinCostPrice(80, input); got != 20.5 {
		t.Fatalf("cost price = %v, want 20.5", got)
	}
	if got := marketplaceSheinCostPrice(0, input); got != 0 {
		t.Fatalf("missing cost price = %v, want 0", got)
	}
}
