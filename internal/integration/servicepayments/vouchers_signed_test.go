package servicepayments

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	wechat "github.com/go-pay/gopay/wechat/v3"
	"io"
	"net/http"
	"strconv"
	"strings"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/integration/paymentsecurity"
	"task-processor/internal/ledger/money"
	"testing"
	"time"
)

func TestActualSignedSDKVoucherQueryCloseAndRefund(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	fixture, original, operation := wechatServiceFixture()
	cfg := fixture.config
	cfg.PrivateKey = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	cfg.PublicKey = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub}))
	cfg.SerialNumber = "fixture-serial"
	cfg.APIv3Key = strings.Repeat("a", 32)
	cfg.NotifyURL = "https://platform.example/notify"
	cfg.NewPayments, cfg.ProductQualified, cfg.PlatformPaysFees = true, true, true
	now := time.Now().UTC()
	for _, scenario := range []string{"CASH", "NOCASH", "late-close", "zero-cash", "missing-payer", "unknown-funding", "duplicate-voucher", "wrong-total", "refund", "missing-payer-refund", "unknown-refund-voucher", "missing-refund-amount", "V1-cash", "V1-close", "V1-coupon"} {
		t.Run(scenario, func(t *testing.T) {
			p, err := NewWeChat(cfg)
			if err != nil {
				t.Fatal(err)
			}
			p.now = func() time.Time { return now }
			o := original
			o.Source.Allocation.Basis = money.ServiceAllocationChannelNetFloorV2
			o.Source.AmountMinor = 100
			o.CancelRequested = true
			kind := "NOCASH"
			if scenario == "CASH" {
				kind = "CASH"
			}
			amount := map[string]any{"total": 100, "payer_total": 80, "currency": "CNY"}
			promotion := map[string]any{"coupon_id": "voucher", "type": kind, "amount": 20, "currency": "CNY"}
			payload := map[string]any{"sp_appid": o.Profile.AppID, "sp_mchid": o.Profile.PlatformMerchantID, "sub_mchid": o.Source.ProviderMerchantID, "out_trade_no": o.TradeNo, "trade_type": "NATIVE", "trade_state": "SUCCESS", "transaction_id": o.Payment.TransactionID, "success_time": now.Format(time.RFC3339), "amount": amount, "promotion_detail": []any{promotion}}
			if strings.HasPrefix(scenario, "V1-") {
				o.Source.Allocation.Basis = money.ServiceAllocationCumulativeNetFloorV1
				if scenario != "V1-coupon" {
					amount["payer_total"] = 100
					delete(payload, "promotion_detail")
				}
			}
			op := operation
			op.Reservation.Kind = money.ServiceRefund
			op.Reservation.AmountMinor = 20
			refund := strings.Contains(scenario, "refund")
			if refund {
				o.Payment.ChannelAmounts = &money.ServicePaymentAmounts{PayerMinor: 80, Vouchers: []money.ServiceVoucher{{ID: "voucher", FundingType: "NOCASH", AmountMinor: 20}}}
				amount = map[string]any{"refund": 20, "payer_refund": 16, "discount_refund": 4, "currency": "CNY"}
				promotion = map[string]any{"promotion_id": "voucher", "type": "DISCOUNT", "amount": 20, "refund_amount": 4}
				payload = map[string]any{"sub_mchid": o.Source.ProviderMerchantID, "out_trade_no": o.TradeNo, "transaction_id": o.Payment.TransactionID, "out_refund_no": op.ProviderRequestID, "refund_id": "original-refund", "refund_account": "REFUND_SOURCE_SUB_MERCHANT", "status": "SUCCESS", "success_time": now.Format(time.RFC3339), "amount": amount, "promotion_detail": []any{promotion}}
			}
			switch scenario {
			case "zero-cash":
				amount["payer_total"] = 0
				promotion["amount"] = 100
			case "missing-payer":
				delete(amount, "payer_total")
			case "unknown-funding":
				promotion["type"] = "UNKNOWN"
			case "duplicate-voucher":
				amount["payer_total"] = 60
				payload["promotion_detail"] = []any{promotion, promotion}
			case "wrong-total":
				amount["payer_total"] = 81
			case "missing-payer-refund":
				delete(amount, "payer_refund")
			case "unknown-refund-voucher":
				promotion["promotion_id"] = "foreign"
			case "missing-refund-amount":
				delete(promotion, "refund_amount")
			}
			body, _ := json.Marshal(payload)
			requests := 0
			client := paymentsecurity.HTTP()
			client.HttpClient.Transport = feeBillTransport(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.URL.Host != "api.mch.weixin.qq.com" || req.Header.Get("Authorization") == "" {
					t.Fatal("uncontrolled/unsigned SDK request")
				}
				responseBody := body
				status := 200
				if req.Method == http.MethodPost {
					if (scenario != "late-close" && scenario != "V1-close") || !strings.HasSuffix(req.URL.Path, "/close") {
						t.Fatal("unexpected channel mutation")
					}
					responseBody = nil
					status = 204
				} else if refund {
					if !strings.Contains(req.URL.Path, op.ProviderRequestID) || req.URL.Query().Get("sub_mchid") != o.Source.ProviderMerchantID {
						t.Fatal("changed original refund")
					}
				} else if !strings.HasSuffix(req.URL.Path, o.TradeNo) || req.URL.Query().Get("sub_mchid") != o.Source.ProviderMerchantID {
					t.Fatal("changed original query")
				}
				ts := strconv.FormatInt(now.Unix(), 10)
				digest := sha256.Sum256([]byte(ts + "\nsigned-voucher\n" + string(responseBody) + "\n"))
				sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
				if err != nil {
					t.Fatal(err)
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Wechatpay-Timestamp": {ts}, "Wechatpay-Nonce": {"signed-voucher"}, "Wechatpay-Serial": {cfg.PublicKeyID}, "Wechatpay-Signature": {base64.StdEncoding.EncodeToString(sig)}}, Body: io.NopCloser(strings.NewReader(string(responseBody))), Request: req}, nil
			})
			p.client.SetHttpClient(client)
			valid := scenario == "CASH" || scenario == "NOCASH" || scenario == "late-close" || scenario == "zero-cash" || scenario == "refund" || scenario == "V1-cash" || scenario == "V1-close"
			if refund {
				v, err := p.QueryServiceOperation(context.Background(), o, op)
				if valid {
					if err != nil || v.RefundAmounts == nil || v.RefundAmounts.PayerMinor != 16 || v.RefundAmounts.SettlementMinor() != 16 {
						t.Fatalf("signed original refund rejected %+v %v", v, err)
					}
				} else if err == nil {
					t.Fatal("missing/foreign refund fact accepted")
				}
			} else {
				v, err := p.QueryServicePayment(context.Background(), o)
				if scenario == "late-close" || scenario == "V1-close" {
					v, err = p.CloseServicePayment(context.Background(), o)
				}
				if valid {
					if err != nil || !v.Matches(o) || (o.Source.Allocation.Basis == money.ServiceAllocationChannelNetFloorV2) != (v.ChannelAmounts != nil) {
						t.Fatalf("signed original voucher rejected %+v %v", v, err)
					}
				} else if err == nil {
					t.Fatal("missing/contradictory original amounts accepted")
				}
			}
			want := 1
			if scenario == "late-close" || scenario == "V1-close" {
				want = 3
			}
			if requests != want {
				t.Fatalf("unexpected SDK requests %d", requests)
			}
		})
	}
}

type originalObservationCapture struct {
	*ingressFixture
	last billing.ServicePaymentObservation
}

func (f *originalObservationCapture) RecordServicePaymentObservation(_ context.Context, _ billing.ServicePurchaseOrder, p billing.ServicePaymentObservation) error {
	f.last = p
	f.recorded = true
	return nil
}
func TestV1SignedCashNotificationKeepsOriginalObservationStructure(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p, o, _ := wechatServiceFixture()
	p.publicKey = &key.PublicKey
	p.config.APIv3Key = "0123456789abcdef0123456789abcdef"
	now := time.Now()
	p.now = func() time.Time { return now }
	plain := `{"sp_appid":"app","sp_mchid":"platform","sub_mchid":"sub","out_trade_no":"original-trade","transaction_id":"transaction","trade_state":"SUCCESS","trade_type":"NATIVE","success_time":"` + now.Format(time.RFC3339) + `","amount":{"total":100,"payer_total":100,"currency":"CNY"}}`
	f := &originalObservationCapture{ingressFixture: &ingressFixture{order: o}}
	i := NotificationIngress{&p, f, f}
	if err := i.AcceptNotification(signedServiceCallback(t, key, p.config.APIv3Key, plain, now)); err != nil || !f.recorded || f.last.ChannelAmounts != nil {
		t.Fatalf("V1 signed notification changed original cash structure %+v %v", f.last, err)
	}
	query, err := p.paymentResult(o, wechat.PartnerQueryOrder{SpAppid: o.Profile.AppID, SpMchid: o.Profile.PlatformMerchantID, SubMchid: o.Source.ProviderMerchantID, OutTradeNo: o.TradeNo, TradeType: "NATIVE", TradeState: "SUCCESS", TransactionId: o.Payment.TransactionID, SuccessTime: now.Format(time.RFC3339), Amount: &wechat.Amount{Total: 100, PayerTotal: 100, Currency: "CNY"}})
	if err != nil || money.ServiceFingerprint(query.ChannelAmounts) != money.ServiceFingerprint(f.last.ChannelAmounts) {
		t.Fatalf("V1 query/notify observations conflict %+v %+v %v", query, f.last, err)
	}
	o.Payment.OccurredAt = f.last.OccurredAt
	originalInput := o.MoneyInput().Fingerprint()
	o.Payment = &f.last
	if o.MoneyInput().Fingerprint() != originalInput {
		t.Fatal("V1 signed notification changed original Money payment identity")
	}
	f.recorded = false
	coupon := strings.Replace(plain, `"payer_total":100`, `"payer_total":80`, 1)
	coupon = strings.TrimSuffix(coupon, "}") + `,"promotion_detail":[{"coupon_id":"voucher","type":"NOCASH","amount":20,"currency":"CNY"}]}`
	if err := i.AcceptNotification(signedServiceCallback(t, key, p.config.APIv3Key, coupon, now)); err == nil || f.recorded {
		t.Fatal("V1 coupon notification reached original inbox")
	}
}

func TestSignedVoucherNotificationReachesOriginalInboxAndWake(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p, o, _ := wechatServiceFixture()
	p.publicKey = &key.PublicKey
	p.config.APIv3Key = "0123456789abcdef0123456789abcdef"
	now := time.Now()
	p.now = func() time.Time { return now }
	o.Source.Allocation.Basis = money.ServiceAllocationChannelNetFloorV2
	plain := `{"sp_appid":"app","sp_mchid":"platform","sub_mchid":"sub","out_trade_no":"original-trade","transaction_id":"transaction","trade_state":"SUCCESS","trade_type":"NATIVE","success_time":"` + now.Format(time.RFC3339) + `","amount":{"total":100,"payer_total":80,"currency":"CNY"},"promotion_detail":[{"coupon_id":"voucher","type":"NOCASH","amount":20,"currency":"CNY"}]}`
	f := &ingressFixture{order: o}
	i := NotificationIngress{&p, f, f}
	if err := i.AcceptNotification(signedServiceCallback(t, key, p.config.APIv3Key, plain, now)); err != nil || !f.recorded {
		t.Fatalf("valid signed coupon rejected before original inbox/wake %v", err)
	}
	for _, bad := range []string{strings.Replace(plain, `"payer_total":80,`, "", 1), strings.Replace(plain, `"NOCASH"`, `"UNKNOWN"`, 1)} {
		f.recorded = false
		if err := i.AcceptNotification(signedServiceCallback(t, key, p.config.APIv3Key, bad, now)); err == nil || f.recorded {
			t.Fatal("missing/unknown facts reached owner")
		}
	}
}
