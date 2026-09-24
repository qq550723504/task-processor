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
			heading: "### 4.2 身份链",
			required: []string{
				"TopUpReversalKey", "Kind WalletReversalKind", "REFUND / CHARGEBACK",
				"UNIQUE(payment_id, reversal_kind, reversal_id) on top-up reversal receipts",
				"UNIQUE(payment_id, reversal_kind, reversal_id) on excess reconciliation records",
				"同类 ID 不能改绑另一 payment", "topup-reversal:v1:",
				"不能直接使用原始 ReversalID 作为跨类型主键",
			},
		},
		{
			heading: "### 5.2 money 侧：复用事实，补足精确结果",
			required: []string{
				"payment_id、reversal_kind、reversal_id", "request/result fingerprint",
				"差额记录与 receipt 的关联也使用完整 typed key",
			},
		},
		{
			heading: "## 10. 入账结果与 readback 契约",
			required: []string{
				"ReadTopUpReversal", "TopUpReversalKey{PaymentID, Kind, ReversalID}",
				"缺失或未知 Kind 返回 ErrInvalid", "完整 key 未找到返回 ErrNotFound",
				"不允许省略 kind、跨类型查找或返回第一条匹配",
			},
		},
		{
			heading: "### 11.2 外部退款/拒付",
			required: []string{
				"TopUpReversalKey", "只有 REFUND", "不能消费同名退款 hold",
			},
		},
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
				"R + H + x <= B", "R' = R + x", "H' = H - x",
				"W = 累计已作用于钱包的本金冲正", "E = 累计渠道超本金差额",
				"d = min(x, C - W)", "e = x - d", "W' = W + d", "E' = E + e",
				"R' = W' + E'", "0 <= W' <= C", "即使 R + x > B 也必须记录完整事实",
				"不得把 R + x <= B 作为已确认事实的接收或 hold 结清门槛",
				"reserved 减少 x", "只将 d 计为本次本金冲正",
				"x - d 按释放规则先偿还既有 debt", "差额不是钱包债务",
				"CONFIRMED", "事实及 hold 已结清、差额待核对",
				"以 `(payment_id, reversal_kind, reversal_id)` 为唯一身份的差额核对记录", "包括 d=0/e>0",
				"不得因字符串 ID 相同而互相覆盖",
				"d=0 时不创建零金额 wallet entry", "不改写旧 receipt",
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
				"NC_INFLIGHT_SUCCESS_AFTER_REVERSAL", "NC_REVERSED_DELIVERY_ORDER", "NC_PRINCIPAL_EXHAUSTED",
				"NC_CROSS_KIND_ID_COLLISION", "NC_CROSS_KIND_WALLET_AND_HOLD",
				`RefundID="r1"`, `ChargebackID="r1"`, "R=10002、W=10000、E=2",
				"两份不同 receipt", "两条各为 1 的 OPEN 差额记录",
				"d=5000、e=1000", "R=11000、W=10000、E=1000",
				"H=0、reserved=0、available=0、debt=0", "CONFIRMED",
				"完整 RefundSettlement=6000", "差额核对记录=1000",
				"重复通知和事务提交响应丢失后重启",
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

// Both channels are first-release requirements; this is a document guard only.
func TestWalletTopUpDesignDualChannelContract(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "alipay-wallet-topup-design.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	checks := []struct {
		heading string
		terms   []string
	}{
		{"## 1. 设计决定与用户结果", []string{"首版同时支持微信支付和支付宝", "WECHAT_PAY", "ALIPAY"}},
		{"## 3. 责任与代码依赖", []string{"internal/integration/payments/wechat", "internal/integration/payments/alipay", "CreateOrReadCheckout", "REDIRECT", "QR_CODE"}},
		{"### 4.4 双渠道绑定与切换", []string{"同一幂等键换渠道必须冲突", "不把 provider 加入创建幂等键", "拒绝自动切换渠道", "原渠道", "新订单"}},
		{"### 7.2 通知验签与接收", []string{"Wechatpay-Serial", "原始 body", "AEAD_AES_256_GCM", "HTTP 204", "success", "先可靠落盘再 ACK", "错误渠道"}},
		{"### 11.4 双渠道退款映射", []string{"原支付的 provider", "out_request_no", "out_refund_no", "PROCESSING", "ABNORMAL", "不释放 hold", "CLOSED"}},
		{"## 14. GoPay 接入约束", []string{"V3TransactionNative", "V3TransactionQueryOrder", "V3RefundQuery", "TradePagePay", "微信 APIv3 不虚构沙箱开关"}},
		{"### 15.2 双渠道验收", []string{"DUAL_HAPPY_PATH", "DUAL_SAME_KEY_DIFFERENT_CHANNEL", "DUAL_CALLBACK_CONFUSION", "DUAL_REFUND_ORIGIN", "DUAL_NATIVE_RESPONSE_LOSS", "未验证一方不能标为双渠道完成"}},
	}
	for _, check := range checks {
		t.Run(check.heading, func(t *testing.T) {
			start := strings.Index(text, check.heading+"\n")
			if start < 0 {
				t.Fatalf("missing section %q", check.heading)
			}
			section := text[start+len(check.heading)+1:]
			if end := strings.Index(section, "\n#"); end >= 0 {
				section = section[:end]
			}
			section = strings.Join(strings.Fields(section), " ")
			for _, term := range check.terms {
				if !strings.Contains(section, term) {
					t.Errorf("section %q must retain %q", check.heading, term)
				}
			}
		})
	}
	for _, superseded := range []string{
		"支付宝是本设计的首发渠道。微信、Stripe、通用渠道路由不进入首版",
		"首版限定 CNY、单渠道、单个收款商户配置",
	} {
		if strings.Contains(text, superseded) {
			t.Errorf("superseded single-channel restriction remains: %q", superseded)
		}
	}
}
