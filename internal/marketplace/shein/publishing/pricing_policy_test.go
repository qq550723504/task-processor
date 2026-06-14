package publishing

import (
	"testing"

	common "task-processor/internal/publishing/common"
)

func TestPricingPolicyDisabledPreservesSourcePrice(t *testing.T) {
	source := &common.Price{Currency: "usd", Amount: 12.8, CostPrice: 9.5}

	price := (PricingPolicy{}).Apply(source)

	if price == nil {
		t.Fatal("expected price")
	}
	if price.Amount != 12.8 {
		t.Fatalf("amount = %v, want 12.8", price.Amount)
	}
	if price.CostPrice != 9.5 {
		t.Fatalf("cost = %v, want 9.5", price.CostPrice)
	}
	if price.Currency != "USD" {
		t.Fatalf("currency = %q, want USD", price.Currency)
	}
}

func TestPricingPolicyEnabledAppliesListingFormula(t *testing.T) {
	source := &common.Price{Currency: "USD", Amount: 10, CostPrice: 8}
	policy := PricingPolicy{
		Enabled:        true,
		MarkupRate:     0.25,
		FixedMarkup:    1,
		ShippingCost:   2,
		CommissionRate: 0.1,
		RoundTo:        0.01,
	}

	price := policy.Apply(source)

	if price == nil {
		t.Fatal("expected price")
	}
	if price.CostPrice != 8 {
		t.Fatalf("cost = %v, want 8", price.CostPrice)
	}
	if price.Amount != 15.28 {
		t.Fatalf("amount = %v, want 15.28", price.Amount)
	}
}

func TestPricingPolicyAppliesMinimumPrice(t *testing.T) {
	source := &common.Price{Currency: "USD", Amount: 2}

	price := (PricingPolicy{Enabled: true, MinimumPrice: 9.99}).Apply(source)

	if price.Amount != 9.99 {
		t.Fatalf("amount = %v, want 9.99", price.Amount)
	}
}
