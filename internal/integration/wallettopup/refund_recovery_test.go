package wallettopup

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"task-processor/internal/commercial/billing"
)

func TestAlipayRefundReplayRequiresVerifiedUnconfirmedOriginalQuery(t *testing.T) {
	key, private, public := fixtureKeys(t)
	merchant := billing.TopUpMerchant{Provider: billing.PaymentAlipay, Environment: "PRODUCTION", ProfileVersion: "v1", MerchantID: "merchant-1", AppID: "app-1", Product: "PAGE_PAY"}
	p, err := NewAlipay(AlipayConfig{Merchant: merchant, PrivateKey: private, PublicKey: public, NotifyURL: "https://merchant.example/notify", ReturnURL: "https://merchant.example/orders"})
	if err != nil {
		t.Fatal(err)
	}
	r := billing.TopUpRefundIntent{Merchant: merchant, MerchantOrderID: "order-1", TradeID: "trade-1", ProviderRequestID: "refund-1", AmountMinor: 6000, TotalMinor: 10000, Dispatched: true}
	for _, scenario := range []string{"empty", "bound", "wrong_order", "wrong_trade", "wrong_request", "wrong_amount", "wrong_total", "pending", "tampered"} {
		t.Run(scenario, func(t *testing.T) {
			body := map[string]string{"code": "10000"}
			if scenario != "empty" {
				body["out_trade_no"], body["trade_no"], body["out_request_no"] = "order-1", "trade-1", "refund-1"
				body["refund_amount"], body["total_amount"] = "60.00", "100.00"
			}
			switch scenario {
			case "wrong_order":
				body["out_trade_no"] = "other"
			case "wrong_trade":
				body["trade_no"] = "other"
			case "wrong_request":
				body["out_request_no"] = "other"
			case "wrong_amount":
				body["refund_amount"] = "61.00"
			case "wrong_total":
				body["total_amount"] = "101.00"
			case "pending":
				body["refund_status"] = "PROCESSING"
			}
			raw, _ := json.Marshal(body)
			sig := signFixture(t, key, string(raw))
			if scenario == "tampered" {
				sig = "invalid"
			}
			response, _ := json.Marshal(map[string]any{"alipay_trade_fastpay_refund_query_response": json.RawMessage(raw), "sign": sig})
			c := providerHTTP()
			c.HttpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if err := req.ParseForm(); err != nil {
					t.Fatal(err)
				}
				var query map[string]any
				if json.Unmarshal([]byte(req.Form.Get("biz_content")), &query) != nil || query["out_request_no"] != r.ProviderRequestID || query["trade_no"] != r.TradeID || query["out_trade_no"] != r.MerchantOrderID {
					t.Fatal("query lost original identity")
				}
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(response))), Request: req}, nil
			})
			p.client.SetHttpClient(c)
			_, err := p.QueryRefund(context.Background(), r)
			want := scenario == "empty" || scenario == "bound"
			if errors.Is(err, billing.ErrRefundReplayAllowed) != want {
				t.Fatalf("unsafe replay classification: %v", err)
			}
		})
	}
}

func TestWeChatRefundReplayRequiresVerifiedOriginalNotFound(t *testing.T) {
	key, private, public := fixtureKeys(t)
	merchant := billing.TopUpMerchant{Provider: billing.PaymentWeChat, Environment: "PRODUCTION", ProfileVersion: "v1", MerchantID: "merchant-1", AppID: "app-1", Product: "NATIVE"}
	p, err := NewWeChat(WeChatConfig{Merchant: merchant, PrivateKey: private, PublicKey: public, PublicKeyID: "PUB_KEY_ID_test", SerialNumber: "serial-1", APIv3Key: strings.Repeat("a", 32), NotifyURL: "https://merchant.example/notify"})
	if err != nil {
		t.Fatal(err)
	}
	r := billing.TopUpRefundIntent{Merchant: merchant, MerchantOrderID: "order-1", TradeID: "trade-1", ProviderRequestID: "refund-1", AmountMinor: 6000, TotalMinor: 10000, Dispatched: true}
	for _, scenario := range []string{"absent", "wrong_code", "tampered", "stale", "wrong_key", "PROCESSING", "ABNORMAL", "CLOSED"} {
		t.Run(scenario, func(t *testing.T) {
			body := `{"code":"RESOURCE_NOT_EXISTS","message":"not found"}`
			status := http.StatusNotFound
			if scenario == "wrong_code" {
				body = `{"code":"ORDER_NOT_EXIST","message":"not found"}`
			}
			if scenario == "PROCESSING" || scenario == "ABNORMAL" || scenario == "CLOSED" {
				status = http.StatusOK
				body = `{"out_trade_no":"order-1","transaction_id":"trade-1","out_refund_no":"refund-1","refund_id":"native-1","status":"` + scenario + `","amount":{"total":10000,"refund":6000,"currency":"CNY"}}`
			}
			c := providerHTTP()
			c.HttpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/v3/refund/domestic/refunds/refund-1" || req.Header.Get("Authorization") == "" {
					t.Fatal("query lost original identity")
				}
				stamp := time.Now()
				if scenario == "stale" {
					stamp = stamp.Add(-10 * time.Minute)
				}
				ts := strconv.FormatInt(stamp.Unix(), 10)
				sig := signFixture(t, key, ts+"\nnonce\n"+body+"\n")
				if scenario == "tampered" {
					sig = "invalid"
				}
				serial := "PUB_KEY_ID_test"
				if scenario == "wrong_key" {
					serial = "PUB_KEY_ID_other"
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Wechatpay-Timestamp": {ts}, "Wechatpay-Nonce": {"nonce"}, "Wechatpay-Serial": {serial}, "Wechatpay-Signature": {sig}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
			})
			p.client.SetHttpClient(c)
			_, err := p.QueryRefund(context.Background(), r)
			if errors.Is(err, billing.ErrRefundReplayAllowed) != (scenario == "absent") {
				t.Fatalf("unsafe replay classification: %v", err)
			}
		})
	}
}
