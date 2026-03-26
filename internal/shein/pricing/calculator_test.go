package pricing_test

import (
	"testing"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/shein/pricing"
)

func float64Ptr(v float64) *float64 { return &v }

func makeRule(ruleType string, ruleValue, fixedValue, priceMin, priceMax *float64) managementapi.PricingRuleRespDTO {
	return managementapi.PricingRuleRespDTO{
		RuleType:   ruleType,
		RuleValue:  ruleValue,
		FixedValue: fixedValue,
		PriceMin:   priceMin,
		PriceMax:   priceMax,
	}
}

func TestAutoPricingCalculator_GetAutoPrice_MatchRule(t *testing.T) {
	calc := pricing.NewAutoPricingCalculator()
	rules := []managementapi.PricingRuleRespDTO{
		makeRule("fixed", float64Ptr(5), nil, float64Ptr(10), float64Ptr(50)),
	}
	got := calc.GetAutoPrice(20, rules)
	want := 25.0
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAutoPricingCalculator_GetAutoPrice_NoMatchRule(t *testing.T) {
	calc := pricing.NewAutoPricingCalculator()
	rules := []managementapi.PricingRuleRespDTO{
		makeRule("fixed", float64Ptr(5), nil, float64Ptr(10), float64Ptr(50)),
	}
	// 价格超出规则范围
	got := calc.GetAutoPrice(100, rules)
	if got != 100 {
		t.Errorf("got %v, want 100", got)
	}
}

func TestAutoPricingCalculator_GetAutoPrice_EmptyRules(t *testing.T) {
	calc := pricing.NewAutoPricingCalculator()
	got := calc.GetAutoPrice(30, nil)
	if got != 30 {
		t.Errorf("got %v, want 30", got)
	}
}

func TestAutoPricingCalculator_ApplyRule(t *testing.T) {
	calc := pricing.NewAutoPricingCalculator()

	tests := []struct {
		name       string
		ruleType   string
		ruleValue  *float64
		fixedValue *float64
		origin     float64
		want       float64
	}{
		{
			name:       "multiple_fixed_with_fixed_value",
			ruleType:   "multiple_fixed",
			ruleValue:  float64Ptr(2),
			fixedValue: float64Ptr(3),
			origin:     10,
			want:       23, // 10*2 + 3
		},
		{
			name:       "multiple_fixed_nil_fixed_value",
			ruleType:   "multiple_fixed",
			ruleValue:  float64Ptr(2),
			fixedValue: nil,
			origin:     10,
			want:       20, // 10*2 + 0
		},
		{
			name:      "fixed",
			ruleType:  "fixed",
			ruleValue: float64Ptr(5),
			origin:    10,
			want:      15,
		},
		{
			name:      "percent",
			ruleType:  "percent",
			ruleValue: float64Ptr(0.2),
			origin:    100,
			want:      120, // 100 * (1 + 0.2)
		},
		{
			name:      "multiple",
			ruleType:  "multiple",
			ruleValue: float64Ptr(3),
			origin:    10,
			want:      30,
		},
		{
			name:      "discount",
			ruleType:  "discount",
			ruleValue: float64Ptr(0.1),
			origin:    100,
			want:      90, // 100 * (1 - 0.1)
		},
		{
			name:      "fixed_price",
			ruleType:  "fixed_price",
			ruleValue: float64Ptr(99),
			origin:    10,
			want:      99,
		},
		{
			name:      "unknown_rule_type",
			ruleType:  "unknown",
			ruleValue: float64Ptr(5),
			origin:    10,
			want:      10, // 原价返回
		},
		{
			name:     "nil_rule_value",
			ruleType: "fixed",
			origin:   10,
			want:     10, // nil RuleValue 返回原价
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rule := managementapi.PricingRuleRespDTO{
				RuleType:   tc.ruleType,
				RuleValue:  tc.ruleValue,
				FixedValue: tc.fixedValue,
			}
			got := calc.ApplyRule(tc.origin, rule)
			if got != tc.want {
				t.Errorf("ApplyRule(%v, %q) = %v, want %v", tc.origin, tc.ruleType, got, tc.want)
			}
		})
	}
}
