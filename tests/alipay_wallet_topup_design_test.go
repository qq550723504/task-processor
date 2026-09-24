package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This guards the proposed document contract, not executable payment behavior.
// Runtime money/PG acceptance remains a separate implementation obligation.
func TestAlipayWalletTopUpDesignReversalContract(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "alipay-wallet-topup-design.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	checks := []struct {
		heading  string
		required []string
	}{
		{
			heading: "## 9. 支付事实与推广收益的接缝",
			required: []string{
				"NON_COMMISSIONABLE", "CommissionableAmountMinor = 0",
				"RecordRefundSettlement", "RecordChargebackSettlement", "§11.3",
			},
		},
		{
			heading:  "### 11.1 主动原路退款：先保留资金，再调用渠道",
			required: []string{"§11.3", "已确认退款及拒付", "未决退款保留", "派发准入", "unknown 保留资金"},
		},
		{
			heading: "### 11.3 货币冲正上限与未决退款保留",
			required: []string{
				"payment_purpose = WALLET_TOP_UP", "B = payment.GrossAmountMinor",
				"C = top_up.AmountMinor", "B = C",
				"R = confirmed_refund_minor + confirmed_chargeback_minor",
				"H = outstanding_refund_hold_minor",
				"不能使用 CommissionableAmountMinor 作为货币冲正上限",
				"R + H + x <= B", "R + x <= B", "R' = R + x", "H' = H - x",
				"先核对原冲正身份及 fingerprint", "原 payment 锁",
				"不得仅因未决 hold 拒绝已发生的外部冲正",
				"R + H > B", "停止新的退款准入", "unknown hold 保留",
				"RecordRefundSettlement", "RecordChargebackSettlement", "ApplyTopUpReversal",
			},
		},
		{
			heading: "### 15.1 非佣金充值与冲正的组合用例",
			required: []string{
				"全部金额单位为分", "CommissionableAmountMinor=0",
				"NC_FULL_REFUND", "NC_MIXED_REVERSALS", "NC_HOLD_CONFIRM_REPLAY",
				"NC_EXTERNAL_DURING_HOLD", "NC_REFUND_BEFORE_POST", "TOPUP_COMMISSION_REGRESSION",
				"4000", "6000", "10000", "5000", "debt=1000",
				"不产生 earning", "实施阶段必须执行", "不是资金行为测试已通过",
			},
		},
	}

	text := string(content)
	for _, check := range checks {
		t.Run(check.heading, func(t *testing.T) {
			start := strings.Index(text, check.heading+"\n")
			if start < 0 {
				t.Fatalf("%s must contain section %q", path, check.heading)
			}
			section := text[start+len(check.heading)+1:]
			if end := strings.Index(section, "\n#"); end >= 0 {
				section = section[:end]
			}
			// Markdown line wrapping must not change a semantic phrase check.
			normalized := strings.Join(strings.Fields(section), " ")
			for _, required := range check.required {
				if !strings.Contains(normalized, required) {
					t.Errorf("%s section %q must retain %q", path, check.heading, required)
				}
			}
		})
	}
}
