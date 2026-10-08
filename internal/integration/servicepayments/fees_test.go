package servicepayments

import (
	"testing"
	"time"
)

func TestOriginalPlatformFeeBillUsesActualFlowAndIntegerYuan(t *testing.T) {
	p, o, _ := wechatServiceFixture()
	o.Source.RequestID = "original-request"
	o.Source.BuyerOrganizationID = "buyer"
	o.Source.ProviderOrganizationID = "provider"
	o.Source.PolicyVersion = "policy"
	o.Payment.OccurredAt = time.Now().UTC()
	bill := "记账时间,微信支付业务单号,资金流水单号,业务名称,业务类型,收支类型,收支金额(元),账户结余(元),资金变更提交申请人,备注,业务凭证号\n" +
		"`2026-10-08 10:00:00,`transaction,`flow-original,`交易,`扣除交易手续费,`支出,`0.03,`100.00,`actor,`memo,`voucher\n" +
		"`2026-10-08 10:01:00,`foreign,`flow-foreign,`交易,`扣除交易手续费,`支出,`9.99,`100.00,`actor,`memo,`voucher\n" +
		"资金流水总笔数,收入笔数,收入金额,支出笔数,支出金额\n`2,`0,`0,`2,`10.02\n"
	facts, err := p.parseFeeBill(o, "2026-10-08", []byte(bill), "verified-bill-hash")
	if err != nil || len(facts) != 1 || facts[0].AmountMinor != 3 || facts[0].FlowID != "flow-original" || facts[0].Returned {
		t.Fatalf("original actual fee lost: %+v %v", facts, err)
	}
	if _, err = p.parseFeeBill(o, "2026-10-09", []byte(bill), "proof"); err == nil {
		t.Fatal("wrong bill date accepted")
	}
	for _, value := range []string{"0.001", "1e2", "-0.03", "9223372036854775807.01"} {
		if _, err = feeMinor(value); err == nil {
			t.Fatalf("ambiguous minor amount accepted %s", value)
		}
	}
}
