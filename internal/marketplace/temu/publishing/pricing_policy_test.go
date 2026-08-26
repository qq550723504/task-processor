package publishing

import (
	"testing"

	"task-processor/internal/listingadmin"
)

func TestCalculateMinAcceptablePriceUsesRuleFormula(t *testing.T) {
	ruleValue := 1.2
	fixedValue := 2.5
	tests := []struct {
		name string
		rule listingadmin.PricingRuleRespDTO
		want float64
	}{
		{name: "multiple fixed", rule: listingadmin.PricingRuleRespDTO{RuleType: "multiple_fixed", RuleValue: &ruleValue, FixedValue: &fixedValue}, want: 14.5},
		{name: "multiple", rule: listingadmin.PricingRuleRespDTO{RuleType: "multiple", RuleValue: &ruleValue}, want: 12},
		{name: "fixed", rule: listingadmin.PricingRuleRespDTO{RuleType: "fixed", RuleValue: &fixedValue}, want: 12.5},
		{name: "fixed price", rule: listingadmin.PricingRuleRespDTO{RuleType: "fixed_price", RuleValue: &fixedValue}, want: 2.5},
		{name: "unknown uses multiple", rule: listingadmin.PricingRuleRespDTO{RuleType: "unexpected", RuleValue: &ruleValue}, want: 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateMinAcceptablePrice(10, &tt.rule); got != tt.want {
				t.Fatalf("CalculateMinAcceptablePrice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateMinAcceptablePriceUsesDefaultForInvalidInput(t *testing.T) {
	ruleValue := 2.0
	tests := []struct {
		name string
		cost float64
		rule *listingadmin.PricingRuleRespDTO
		want float64
	}{
		{name: "non-positive cost", cost: 0, rule: &listingadmin.PricingRuleRespDTO{RuleType: "multiple", RuleValue: &ruleValue}, want: 0},
		{name: "missing rule", cost: 10, want: 15},
		{name: "missing rule value", cost: 10, rule: &listingadmin.PricingRuleRespDTO{RuleType: "multiple"}, want: 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateMinAcceptablePrice(tt.cost, tt.rule); got != tt.want {
				t.Fatalf("CalculateMinAcceptablePrice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectApplicablePricingRuleUsesExclusiveBounds(t *testing.T) {
	min := 10.0
	max := 20.0
	fallbackMin := 0.0
	fallbackMax := 100.0
	rules := []listingadmin.PricingRuleRespDTO{
		{Name: "first", PriceMin: &min, PriceMax: &max},
		{Name: "fallback", PriceMin: &fallbackMin, PriceMax: &fallbackMax},
	}

	if got := SelectApplicablePricingRule(15, rules); got == nil || got.Name != "first" {
		t.Fatalf("SelectApplicablePricingRule() = %+v, want first rule", got)
	}
	if got := SelectApplicablePricingRule(10, rules); got == nil || got.Name != "fallback" {
		t.Fatalf("SelectApplicablePricingRule(lower bound) = %+v, want fallback rule", got)
	}
	if got := SelectApplicablePricingRule(20, rules); got == nil || got.Name != "fallback" {
		t.Fatalf("SelectApplicablePricingRule(upper bound) = %+v, want fallback rule", got)
	}
}
