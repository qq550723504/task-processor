package servicepayments

import (
	wechat "github.com/go-pay/gopay/wechat/v3"
	"task-processor/internal/ledger/money"
	"testing"
	"time"
)

func TestSignedChannelPaymentVouchersCanReachOriginalOrder(t *testing.T) {
	for _, kind := range []string{"CASH", "NOCASH"} {
		t.Run(kind, func(t *testing.T) {
			p, o, _ := wechatServiceFixture()
			o.Source.Allocation.Basis = "CHANNEL_SETTLEMENT_NET_FLOOR_V2"
			o.Source.AmountMinor = 10000
			v := wechat.PartnerQueryOrder{SpAppid: o.Profile.AppID, SpMchid: o.Profile.PlatformMerchantID, SubMchid: o.Source.ProviderMerchantID, OutTradeNo: o.TradeNo, TradeType: "NATIVE", TradeState: "SUCCESS", TransactionId: "original-payment", SuccessTime: time.Now().Format(time.RFC3339), Amount: &wechat.Amount{Total: 10000, PayerTotal: 8000, Currency: "CNY", PayerCurrency: "CNY"}, PromotionDetail: []*wechat.PromotionDetail{{CouponId: "original-voucher", Type: kind, Amount: 2000, Currency: "CNY"}}}
			result, err := p.paymentResult(o, v)
			if err != nil || result.State != "PAID" || result.AmountMinor != 10000 {
				t.Fatalf("valid original %s voucher rejected %+v %v", kind, result, err)
			}
		})
	}
}
func TestOriginalCouponRefundCanBeVerifiedWithoutInventingCash(t *testing.T) {
	p, o, op := wechatServiceFixture()
	o.Source.Allocation.Basis = "CHANNEL_SETTLEMENT_NET_FLOOR_V2"
	o.Payment.ChannelAmounts = &money.ServicePaymentAmounts{PayerMinor: 80, Vouchers: []money.ServiceVoucher{{ID: "original-voucher", FundingType: "NOCASH", AmountMinor: 20}}}
	op.Reservation.Kind = money.ServiceRefund
	op.Reservation.AmountMinor = 20
	op.ProviderRequestID = "original-refund"
	v := wechat.EcommerceRefundQuery{RefundId: "channel-refund", OutRefundNo: op.ProviderRequestID, TransactionId: o.Payment.TransactionID, OutTradeNo: o.TradeNo, RefundAccount: "REFUND_SOURCE_SUB_MERCHANT", Status: "SUCCESS", SuccessTime: time.Now().Format(time.RFC3339), Amount: &wechat.EcommerceRefundAmount{Refund: 20, PayerRefund: 16, DiscountRefund: 4, Currency: "CNY"}, PromotionDetail: []*wechat.PromotionDetailItem{{PromotionId: "original-voucher", Type: "DISCOUNT", Amount: 20, RefundAmount: 4}}}
	result, err := p.refundResult(o, op, v)
	if err != nil || result.State != "SUCCESS" {
		t.Fatalf("actual original coupon refund rejected %+v %v", result, err)
	}
}

func TestV1CashPaymentKeepsOriginalObservationIdentity(t *testing.T) {
	p, o, _ := wechatServiceFixture()
	at := time.Now().UTC().Truncate(time.Second)
	v := wechat.PartnerQueryOrder{SpAppid: o.Profile.AppID, SpMchid: o.Profile.PlatformMerchantID, SubMchid: o.Source.ProviderMerchantID, OutTradeNo: o.TradeNo, TradeType: "NATIVE", TradeState: "SUCCESS", TransactionId: o.Payment.TransactionID, SuccessTime: at.Format(time.RFC3339), Amount: &wechat.Amount{Total: 100, PayerTotal: 100, Currency: "CNY"}}
	observation, err := p.paymentResult(o, v)
	original := p.paymentObservation(o, "PAID", o.Payment.TransactionID, 100, at)
	if err != nil || observation.ChannelAmounts != nil || observation.EventID != original.EventID {
		t.Fatalf("original V1 cash observation changed %+v %v", observation, err)
	}
}
