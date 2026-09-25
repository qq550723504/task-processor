package httpapi

import "testing"

func TestMarketplaceSheinCostPriceComposition(t *testing.T) {
	if got := marketplaceSheinCostPrice(80, 8, 2, 9, 0.5, 0.49); got != 20.5 {
		t.Fatalf("cost price = %v, want 20.5", got)
	}
	if got := marketplaceSheinCostPrice(0, 8, 2, 9, 0.5, 0.49); got != 0 {
		t.Fatalf("missing cost price = %v, want 0", got)
	}
}
