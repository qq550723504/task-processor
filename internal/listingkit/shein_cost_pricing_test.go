package listingkit

import (
	"reflect"
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestSheinPricingReviewsPreserveCostAndOverridePrecedence(t *testing.T) {
	t.Parallel()

	rule := sheinpub.PricingRule{
		SourceCurrency: "CNY", TargetCurrency: "USD",
		ExchangeRate: 8, MarkupMultiplier: 2, MinimumPrice: 9,
		RoundTo: 0.5, PriceEnding: 0.49,
	}
	builders := []struct {
		name  string
		build func(*sheinpub.Package, sheinpub.PricingRule, map[string]float64, SheinCostPriceCalculator) *sheinpub.PricingReview
		draft bool
	}{
		{name: "calculated review", build: buildSheinPricingReview},
		{name: "draft backed review", build: buildSheinDraftBackedPricingReview, draft: true},
	}
	tests := []struct {
		name           string
		cost           string
		base           string
		site           string
		override       float64
		wantCalculated float64
		wantDraft      float64
		manual         bool
	}{
		{name: "cost calculation", cost: "80", wantCalculated: 20.5, wantDraft: 20.5},
		{name: "minimum then ending then increment", cost: "1", wantCalculated: 9.5, wantDraft: 9.5},
		{name: "base price precedes site and cost", cost: "80", base: "17.25", site: "18.25", wantCalculated: 20.5, wantDraft: 17.25},
		{name: "site price precedes cost", cost: "80", site: "18.25", wantCalculated: 20.5, wantDraft: 18.25},
		{name: "invalid draft prices use cost", cost: "80", base: "invalid", site: "-1", wantCalculated: 20.5, wantDraft: 20.5},
		{name: "positive manual override", cost: "80", base: "17.25", override: 31.75, wantCalculated: 20.5, wantDraft: 17.25, manual: true},
		{name: "negative manual override ignored", cost: "80", override: -1, wantCalculated: 20.5, wantDraft: 20.5},
		{name: "missing price stays not ready", cost: "invalid"},
		{name: "manual override supplies missing price", cost: "invalid", override: 31.75, manual: true},
	}
	for _, builder := range builders {
		t.Run(builder.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					sku := sheinpub.SKUDraft{SupplierSKU: "sku-1", CostPrice: tt.cost, BasePrice: tt.base, Currency: "USD"}
					if tt.site != "" {
						sku.SitePriceList = []sheinpub.SitePrice{{SubSite: "US", BasePrice: tt.site, Currency: "USD"}}
					}
					pkg := &sheinpub.Package{DraftPayload: &sheinpub.RequestDraft{
						SKCList: []sheinpub.SKCRequestDraft{{SupplierCode: "skc-1", SKUList: []sheinpub.SKUDraft{sku}}},
					}}
					overrides := map[string]float64{"sku-1": tt.override}
					review := builder.build(pkg, rule, overrides, testSheinCostPrice)
					if review == nil || len(review.SKUPrices) != 1 {
						t.Fatalf("expected one SKU price, got %+v", review)
					}
					want := tt.wantCalculated
					if builder.draft {
						want = tt.wantDraft
					}
					wantFinal := want
					if tt.manual {
						wantFinal = tt.override
					}
					got := review.SKUPrices[0]
					if got.CalculatedPrice != want || got.FinalPrice != wantFinal || got.Manual != tt.manual {
						t.Fatalf("price = %+v; want calculated=%v final=%v manual=%v", got, want, wantFinal, tt.manual)
					}
					if got.SupplierSKU != "sku-1" || got.SupplierCode != "skc-1" || got.Currency != "USD" {
						t.Fatalf("price identity/currency changed: %+v", got)
					}
					if review.Ready != (wantFinal > 0) {
						t.Fatalf("ready = %v for final price %v", review.Ready, wantFinal)
					}
					var wantMissing []string
					if wantFinal <= 0 {
						wantMissing = []string{"sku-1"}
					}
					if !reflect.DeepEqual(review.MissingPriceSKUs, wantMissing) {
						t.Fatalf("missing prices = %v, want %v", review.MissingPriceSKUs, wantMissing)
					}
					if review.RuleSnapshot == nil || *review.RuleSnapshot != rule || review.UpdatedAt == nil {
						t.Fatalf("rule snapshot/timestamp missing or changed: %+v", review)
					}
				})
			}
		})
	}
}
