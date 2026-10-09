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
	"io"
	"net/http"
	"strconv"
	"strings"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/integration/paymentsecurity"
	"testing"
	"time"
)

func TestNativeCheckoutRequiresSignedOriginalRecoveryProof(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	fixture, original, _ := wechatServiceFixture()
	cfg := fixture.config
	cfg.PrivateKey = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	cfg.PublicKey = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub}))
	cfg.SerialNumber, cfg.APIv3Key, cfg.NotifyURL = "fixture-serial", strings.Repeat("a", 32), "https://platform.example/notify"
	cfg.NewPayments, cfg.ProductQualified, cfg.PlatformPaysFees = true, true, true
	now := time.Now().UTC()
	original.PaymentDispatched = true
	original.ExpiresAt = now.Add(20 * time.Minute)
	for _, scenario := range []string{"unpaid", "absent", "wrong-amount", "wrong-currency", "wrong-merchant", "user-paying", "invalid-signature", "expired-absence", "native-up-url"} {
		t.Run(scenario, func(t *testing.T) {
			p, err := NewWeChat(cfg)
			if err != nil {
				t.Fatal(err)
			}
			p.now = func() time.Time { return now }
			o := original
			status := http.StatusOK
			payload := map[string]any{"sp_appid": o.Profile.AppID, "sp_mchid": o.Profile.PlatformMerchantID, "sub_mchid": o.Source.ProviderMerchantID, "out_trade_no": o.TradeNo, "trade_type": "NATIVE", "trade_state": "NOTPAY", "amount": map[string]any{"total": o.Source.AmountMinor, "currency": "CNY"}}
			switch scenario {
			case "absent", "invalid-signature", "expired-absence":
				status = http.StatusNotFound
				payload = map[string]any{"code": "ORDER_NOT_EXIST", "message": "original order absent"}
			case "wrong-amount":
				payload["amount"] = map[string]any{"total": o.Source.AmountMinor + 1, "currency": "CNY"}
			case "wrong-currency":
				payload["amount"] = map[string]any{"total": o.Source.AmountMinor, "currency": "USD"}
			case "wrong-merchant":
				payload["sub_mchid"] = "foreign-merchant"
			case "user-paying":
				payload["trade_state"] = "USERPAYING"
			case "native-up-url":
				payload = map[string]any{"code_url": "weixin://wxpay/bizpayurl/up?pr=original-native"}
			}
			if scenario == "expired-absence" {
				o.ExpiresAt = now.Add(-time.Second)
			}
			body, _ := json.Marshal(payload)
			requests := 0
			client := paymentsecurity.HTTP()
			client.HttpClient.Transport = feeBillTransport(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.URL.Host != "api.mch.weixin.qq.com" || req.Header.Get("Authorization") == "" {
					t.Fatal("uncontrolled or unsigned request")
				}
				if scenario == "native-up-url" {
					if req.Method != http.MethodPost || req.URL.Path != "/v3/pay/partner/transactions/native" {
						t.Fatal("not Native checkout")
					}
					var sent map[string]any
					if json.NewDecoder(req.Body).Decode(&sent) != nil || sent["out_trade_no"] != o.TradeNo || sent["sp_mchid"] != o.Profile.PlatformMerchantID || sent["sub_mchid"] != o.Source.ProviderMerchantID || sent["time_expire"] != o.ExpiresAt.Format(time.RFC3339) {
						t.Fatal("original Native parameters changed")
					}
				} else if req.Method != http.MethodGet || req.URL.Path != "/v3/pay/partner/transactions/out-trade-no/"+o.TradeNo || req.URL.Query().Get("sp_mchid") != o.Profile.PlatformMerchantID || req.URL.Query().Get("sub_mchid") != o.Source.ProviderMerchantID {
					t.Fatal("not the original merchant/order query")
				}
				ts := strconv.FormatInt(now.Unix(), 10)
				digest := sha256.Sum256([]byte(ts + "\nfixture-native\n" + string(body) + "\n"))
				sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
				if err != nil {
					t.Fatal(err)
				}
				encoded := base64.StdEncoding.EncodeToString(sig)
				if scenario == "invalid-signature" {
					encoded = "invalid"
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Wechatpay-Timestamp": {ts}, "Wechatpay-Nonce": {"fixture-native"}, "Wechatpay-Serial": {cfg.PublicKeyID}, "Wechatpay-Signature": {encoded}}, Body: io.NopCloser(strings.NewReader(string(body))), Request: req}, nil
			})
			p.client.SetHttpClient(client)
			if scenario == "native-up-url" {
				qr, err := p.CreateServiceCheckout(context.Background(), o)
				if err != nil || qr != "weixin://wxpay/bizpayurl/up?pr=original-native" {
					t.Fatalf("official Native URL rejected: %q %v", qr, err)
				}
			} else {
				obs, err := p.QueryServicePayment(context.Background(), o)
				switch scenario {
				case "unpaid":
					if err != nil || !obs.AllowsCheckoutReplay(o) || obs.CheckoutRecovery != billing.ServiceCheckoutUnpaidProof || obs.AmountMinor != o.Source.AmountMinor {
						t.Fatalf("strict unpaid proof missing: %+v %v", obs, err)
					}
				case "absent":
					if err != nil || !obs.AllowsCheckoutReplay(o) || obs.CheckoutRecovery != billing.ServiceCheckoutAbsentProof || obs.AmountMinor != 0 {
						t.Fatalf("signed absence proof conflated: %+v %v", obs, err)
					}
				case "user-paying", "expired-absence":
					if err != nil || obs.AllowsCheckoutReplay(o) {
						t.Fatalf("paying/closed order authorized reentry: %+v %v", obs, err)
					}
				default:
					if err == nil || obs.AllowsCheckoutReplay(o) {
						t.Fatalf("unverified/mismatched query authorized checkout: %+v %v", obs, err)
					}
				}
			}
			if requests != 1 {
				t.Fatalf("unexpected SDK dispatch count: %d", requests)
			}
		})
	}
}
