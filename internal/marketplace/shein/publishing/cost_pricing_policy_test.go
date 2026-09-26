package publishing

import (
	"math"
	"testing"
)

func TestCalculateCostPrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cost float64
		rule CostPriceRule
		want float64
	}{
		{name: "conversion and multiplier", cost: 80, rule: CostPriceRule{ExchangeRate: 8, MarkupMultiplier: 2}, want: 20},
		{name: "zero cost", cost: 0, rule: CostPriceRule{ExchangeRate: 8, MarkupMultiplier: 2, MinimumPrice: 9.99}, want: 0},
		{name: "negative cost", cost: -1, rule: CostPriceRule{ExchangeRate: 8, MarkupMultiplier: 2}, want: 0},
		{name: "zero exchange rate", cost: 80, rule: CostPriceRule{MarkupMultiplier: 2, MinimumPrice: 9.99}, want: 0},
		{name: "negative exchange rate", cost: 80, rule: CostPriceRule{ExchangeRate: -8, MarkupMultiplier: 2}, want: 0},
		{name: "minimum price", cost: 8, rule: CostPriceRule{ExchangeRate: 8, MarkupMultiplier: 2, MinimumPrice: 9.99}, want: 9.99},
		{name: "ending above fraction", cost: 81, rule: CostPriceRule{ExchangeRate: 8, MarkupMultiplier: 2, PriceEnding: 0.49}, want: 20.49},
		{name: "ending below fraction", cost: 83, rule: CostPriceRule{ExchangeRate: 8, MarkupMultiplier: 2, PriceEnding: 0.49}, want: 21.49},
		{name: "ending equal fraction", cost: 82, rule: CostPriceRule{ExchangeRate: 8, MarkupMultiplier: 2, PriceEnding: 0.5}, want: 20.5},
		{name: "ending before increment", cost: 81, rule: CostPriceRule{ExchangeRate: 8, MarkupMultiplier: 2, PriceEnding: 0.49, RoundTo: 0.5}, want: 20.5},
		{name: "minimum before ending", cost: 8, rule: CostPriceRule{ExchangeRate: 8, MarkupMultiplier: 2, MinimumPrice: 9.75, PriceEnding: 0.49}, want: 10.49},
		{name: "increment ceiling", cost: 81, rule: CostPriceRule{ExchangeRate: 8, MarkupMultiplier: 2, RoundTo: 0.5}, want: 20.5},
		{name: "no increment default", cost: 1.004, rule: CostPriceRule{ExchangeRate: 1, MarkupMultiplier: 1}, want: 1},
		{name: "negative increment disabled", cost: 1.004, rule: CostPriceRule{ExchangeRate: 1, MarkupMultiplier: 1, RoundTo: -1}, want: 1},
		{name: "two decimal rounding", cost: 1.236, rule: CostPriceRule{ExchangeRate: 1, MarkupMultiplier: 1}, want: 1.24},
		{name: "ending of one disabled", cost: 81, rule: CostPriceRule{ExchangeRate: 8, MarkupMultiplier: 2, PriceEnding: 1}, want: 20.25},
		{name: "negative ending disabled", cost: 81, rule: CostPriceRule{ExchangeRate: 8, MarkupMultiplier: 2, PriceEnding: -0.49}, want: 20.25},
		{name: "zero multiplier not defaulted", cost: 80, rule: CostPriceRule{ExchangeRate: 8}, want: 0},
		{name: "zero multiplier with minimum", cost: 80, rule: CostPriceRule{ExchangeRate: 8, MinimumPrice: 9.99}, want: 9.99},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			before := tt.rule
			got := CalculateCostPrice(tt.cost, tt.rule)
			if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("CalculateCostPrice(%v, %+v) = %v, want %v", tt.cost, tt.rule, got, tt.want)
			}
			if tt.rule != before {
				t.Fatal("calculation mutated its input rule")
			}
		})
	}
}
