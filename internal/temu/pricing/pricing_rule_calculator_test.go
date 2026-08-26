package pricing

import (
	"bytes"
	"strings"
	"testing"

	"task-processor/internal/listingadmin"

	"github.com/sirupsen/logrus"
)

func TestPricingRuleCalculatorPreservesCalculationInfoLogs(t *testing.T) {
	ruleValue := 1.2
	fixedValue := 2.5
	tests := []struct {
		name    string
		rule    *listingadmin.PricingRuleRespDTO
		wantLog string
	}{
		{
			name:    "default rate",
			wantLog: "使用默认利润率 1.50: 10.00 * 1.50 = 15.00",
		},
		{
			name:    "multiple fixed",
			rule:    &listingadmin.PricingRuleRespDTO{Name: "tiered", RuleType: "multiple_fixed", RuleValue: &ruleValue, FixedValue: &fixedValue},
			wantLog: "使用核价规则 tiered (倍率加固定值): 10.00 * 1.20 + 2.50 = 14.50",
		},
		{
			name:    "multiple",
			rule:    &listingadmin.PricingRuleRespDTO{Name: "multiplier", RuleType: "multiple", RuleValue: &ruleValue},
			wantLog: "使用核价规则 multiplier (倍率): 10.00 * 1.20 = 12.00",
		},
		{
			name:    "fixed",
			rule:    &listingadmin.PricingRuleRespDTO{Name: "fixed", RuleType: "fixed", RuleValue: &fixedValue},
			wantLog: "使用核价规则 fixed (固定加价): 10.00 + 2.50 = 12.50",
		},
		{
			name:    "fixed price",
			rule:    &listingadmin.PricingRuleRespDTO{Name: "fixed-price", RuleType: "fixed_price", RuleValue: &fixedValue},
			wantLog: "使用核价规则 fixed-price (固定价格): 2.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := logrus.New()
			logger.SetOutput(&output)
			calculator := &PricingRuleCalculator{logger: logrus.NewEntry(logger)}

			calculator.CalculateMinAcceptablePrice(10, tt.rule)

			if !strings.Contains(output.String(), tt.wantLog) {
				t.Fatalf("log output = %q, want substring %q", output.String(), tt.wantLog)
			}
		})
	}
}
