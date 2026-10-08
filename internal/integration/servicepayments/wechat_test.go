package servicepayments

import (
	wechat "github.com/go-pay/gopay/wechat/v3"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/ledger/money"
	"testing"
	"time"
)

func wechatServiceFixture() (WeChat, billing.ServicePurchaseOrder, billing.ServiceFinancialOperation) {
	profile := billing.ServiceMerchantProfile{Version: "profile", Environment: "PRODUCTION", PlatformMerchantID: "platform", AppID: "app", FreezeDays: 180}
	p := WeChat{config: WeChatConfig{Profile: profile, PublicKeyID: "PUB_KEY_ID_fixture"}, now: time.Now}
	o := billing.ServicePurchaseOrder{Source: billing.ServicePurchaseCommand{OrderID: "service-order", ProviderMerchantID: "sub", AmountMinor: 100}, Profile: profile, TradeNo: "original-trade", Payment: &billing.ServicePaymentObservation{TransactionID: "transaction"}}
	op := billing.ServiceFinancialOperation{Reservation: money.ServiceOperation{OrderID: "service-order", OperationID: "share-op", Kind: money.ServiceShare, AmountMinor: 10}, ProviderRequestID: "share-request"}
	return p, o, op
}
func TestWeChatFinishedShareRequiresExactSuccessfulReceiver(t *testing.T) {
	p, o, op := wechatServiceFixture()
	at := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	v := wechat.EcommerceProfitShareQuery{SubMchid: "sub", TransactionId: "transaction", OutOrderNo: "share-request", OrderId: "channel-share", Status: "FINISHED", Receivers: []*wechat.EcommerceReceiver{{Type: "MERCHANT_ID", ReceiverAccount: "platform", Amount: 10, Result: "CLOSED"}}}
	got, err := p.shareResult(o, op, v, at)
	if err != nil || got.State != "FAILED" {
		t.Fatalf("closed receiver must be known failure: %+v %v", got, err)
	}
	v.Receivers[0].Result = "SUCCESS"
	v.Receivers[0].DetailId = "detail"
	v.Receivers[0].FinishTime = at.Format(time.RFC3339)
	got, err = p.shareResult(o, op, v, at)
	if err != nil || got.State != "SUCCESS" {
		t.Fatalf("exact success %+v %v", got, err)
	}
	v.Receivers[0].ReceiverAccount = "foreign"
	if _, err = p.shareResult(o, op, v, at); err == nil {
		t.Fatal("wrong recipient accepted")
	}
	v.Receivers[0].ReceiverAccount = "platform"
	v.Receivers[0].Amount = 100
	if _, err = p.shareResult(o, op, v, at); err == nil {
		t.Fatal("wrong amount accepted")
	}
}
func TestWeChatFinishAndRefundCannotBorrowAnotherEffect(t *testing.T) {
	p, o, op := wechatServiceFixture()
	op.Reservation.Kind = money.ServiceFinish
	op.Reservation.AmountMinor = 90
	op.ProviderRequestID = "finish-request"
	v := wechat.EcommerceProfitShareQuery{SubMchid: "sub", TransactionId: "transaction", OutOrderNo: "finish-request", OrderId: "channel-finish", Status: "FINISHED", FinishAmount: 90}
	got, err := p.shareResult(o, op, v, time.Now())
	if err != nil || got.State != "SUCCESS" {
		t.Fatalf("finish amount not proved %+v %v", got, err)
	}
	v.OutOrderNo = "share-request"
	if _, err = p.shareResult(o, op, v, time.Now()); err == nil {
		t.Fatal("share identity used as finish")
	}
	op.Reservation.Kind = money.ServiceRefund
	op.Reservation.AmountMinor = 20
	op.ProviderRequestID = "refund-request"
	refund := wechat.EcommerceRefundQuery{RefundId: "channel-refund", OutRefundNo: "refund-request", TransactionId: "transaction", OutTradeNo: "original-trade", Status: "SUCCESS", RefundAccount: "REFUND_SOURCE_SUB_MERCHANT", SuccessTime: time.Now().Format(time.RFC3339), Amount: &wechat.EcommerceRefundAmount{Refund: 20, Currency: "CNY", PayerRefund: 20}}
	got, err = p.refundResult(o, op, refund)
	if err != nil || got.State != "SUCCESS" {
		t.Fatalf("original refund %+v %v", got, err)
	}
	refund.RefundAccount = "REFUND_SOURCE_PARTNER_ADVANCE"
	if _, err = p.refundResult(o, op, refund); err == nil {
		t.Fatal("unauthorized platform advance accepted")
	}
}
