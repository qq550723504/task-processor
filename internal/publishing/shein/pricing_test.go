package shein

import (
	"testing"

	sheinmarketplace "task-processor/internal/marketplace/shein/publishing"
	common "task-processor/internal/publishing/common"
)

func TestPricingPolicyBridgesMarketplacePolicy(t *testing.T) {
	t.Parallel()

	var policy sheinmarketplace.PricingPolicy = PricingPolicy{
		Enabled:        true,
		MarkupRate:     0.25,
		FixedMarkup:    1,
		ShippingCost:   2,
		CommissionRate: 0.1,
		RoundTo:        0.01,
	}

	price := policy.Apply(&common.Price{Currency: "USD", Amount: 10, CostPrice: 8})

	if price == nil {
		t.Fatal("expected price")
	}
	if price.Amount != 15.28 {
		t.Fatalf("amount = %v, want 15.28", price.Amount)
	}
}
