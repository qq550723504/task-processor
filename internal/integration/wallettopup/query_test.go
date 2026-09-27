package wallettopup

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"task-processor/internal/commercial/billing"

	wechat "github.com/go-pay/gopay/wechat/v3"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWeChatUnpaidQueryAllowsAbsentPaidOnlyFields(t *testing.T) {
	p := &WeChat{config: WeChatConfig{Merchant: billing.TopUpMerchant{Provider: billing.PaymentWeChat, Environment: "PRODUCTION", ProfileVersion: "v1", MerchantID: "merchant-1", AppID: "app-1", Product: "NATIVE"}, PublicKeyID: "PUB_KEY_ID_test"}}
	v := wechat.QueryOrder{Mchid: "merchant-1", Appid: "app-1", OutTradeNo: "order-1", TradeState: "CLOSED"}
	o, err := p.payment(v)
	if err != nil || o.State != "CLOSED" {
		t.Fatalf("closed: %+v %v", o, err)
	}
	v.TradeState = "SUCCESS"
	if _, err = p.payment(v); err == nil {
		t.Fatal("accepted paid without amount or trade type")
	}
	v.TradeState = "NOTPAY"
	v.Mchid = "other"
	if _, err = p.payment(v); err == nil {
		t.Fatal("accepted another merchant")
	}
}

func TestWeChatQueryRequiresSignedOriginalBinding(t *testing.T) {
	key, private, public := fixtureKeys(t)
	merchant := billing.TopUpMerchant{Provider: billing.PaymentWeChat, Environment: "PRODUCTION", ProfileVersion: "v1", MerchantID: "merchant-1", AppID: "app-1", Product: "NATIVE"}
	p, err := NewWeChat(WeChatConfig{Merchant: merchant, PrivateKey: private, PublicKey: public, PublicKeyID: "PUB_KEY_ID_test", SerialNumber: "serial-1", APIv3Key: strings.Repeat("a", 32), NotifyURL: "https://merchant.example/notify"})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"appid":"app-1","mchid":"merchant-1","out_trade_no":"order-1","trade_state":"SUCCESS","trade_type":"NATIVE","transaction_id":"trade-1","success_time":"2026-09-27T12:00:00+08:00","amount":{"total":10000,"currency":"CNY"}}`
	tamper := false
	c := providerHTTP()
	c.HttpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "api.mch.weixin.qq.com" || req.URL.Path != "/v3/pay/transactions/out-trade-no/order-1" || req.URL.Query().Get("mchid") != merchant.MerchantID || req.Header.Get("Authorization") == "" {
			t.Fatal("incorrect signed query")
		}
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		nonce := "fixture-nonce"
		sig := signFixture(t, key, ts+"\n"+nonce+"\n"+body+"\n")
		raw := body
		if tamper {
			raw = strings.Replace(raw, "10000", "20000", 1)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Wechatpay-Timestamp": {ts}, "Wechatpay-Nonce": {nonce}, "Wechatpay-Serial": {"PUB_KEY_ID_test"}, "Wechatpay-Signature": {sig}}, Body: io.NopCloser(strings.NewReader(raw)), Request: req}, nil
	})
	p.client.SetHttpClient(c)
	a := billing.TopUpPaymentAttempt{Merchant: merchant, MerchantOrderID: "order-1", Currency: "CNY", AmountMinor: 10000}
	o, err := p.QueryPayment(context.Background(), a)
	if err != nil || o.State != "PAID" || o.AmountMinor != 10000 {
		t.Fatalf("query %+v %v", o, err)
	}
	tamper = true
	if _, err = p.QueryPayment(context.Background(), a); err == nil {
		t.Fatal("accepted altered signed response")
	}
}
