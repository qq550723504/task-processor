package wallettopup

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"task-processor/internal/commercial/billing"

	"github.com/go-pay/gopay"
)

func fixtureKeys(t *testing.T) (*rsa.PrivateKey, string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return key, string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})), string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub}))
}
func signFixture(t *testing.T, key *rsa.PrivateKey, raw string) string {
	t.Helper()
	h := sha256.Sum256([]byte(raw))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

func TestAlipayCallbackSignatureMerchantAndFormAmbiguity(t *testing.T) {
	key, private, public := fixtureKeys(t)
	p, err := NewAlipay(AlipayConfig{Merchant: billing.TopUpMerchant{Provider: billing.PaymentAlipay, Environment: "SANDBOX", ProfileVersion: "v1", MerchantID: "merchant-1", AppID: "app-1", Product: "PAGE_PAY"}, PrivateKey: private, PublicKey: public, NotifyURL: "https://merchant.example/notify", ReturnURL: "https://merchant.example/orders"})
	if err != nil {
		t.Fatal(err)
	}
	values := url.Values{"app_id": {"app-1"}, "seller_id": {"merchant-1"}, "out_trade_no": {"order-1"}, "notify_id": {"event-1"}, "trade_no": {"trade-1"}, "trade_status": {"TRADE_SUCCESS"}, "total_amount": {"100.00"}, "gmt_payment": {"2026-09-27 12:00:00"}}
	bm := gopay.BodyMap{}
	for k, v := range values {
		bm[k] = v[0]
	}
	values.Set("sign", signFixture(t, key, bm.EncodeAliPaySignParams()))
	values.Set("sign_type", "RSA2")
	verify := func(v url.Values, query string) error {
		r := httptest.NewRequest("POST", "https://merchant.example/notify"+query, strings.NewReader(v.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		o, err := p.VerifyNotification(r)
		if err == nil && (o.State != "PAID" || o.AmountMinor != 10000) {
			t.Fatal("incorrect payment")
		}
		return err
	}
	if err = verify(values, ""); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []func(url.Values){func(v url.Values) { v.Set("total_amount", "1000.00") }, func(v url.Values) { v.Add("seller_id", "merchant-1") }, func(v url.Values) { v.Set("app_id", "other") }, func(v url.Values) { v.Set("sign_type", "RSA") }} {
		v := url.Values{}
		for k, items := range values {
			v[k] = append([]string(nil), items...)
		}
		mutation(v)
		if verify(v, "") == nil {
			t.Fatal("accepted tampering")
		}
	}
	if verify(values, "?seller_id=other") == nil {
		t.Fatal("accepted query contamination")
	}
}

func TestWeChatCallbackExactBytesSignatureAEADAndBinding(t *testing.T) {
	key, private, public := fixtureKeys(t)
	now := time.Now().UTC().Truncate(time.Second)
	p, err := NewWeChat(WeChatConfig{Merchant: billing.TopUpMerchant{Provider: billing.PaymentWeChat, Environment: "PRODUCTION", ProfileVersion: "v1", MerchantID: "merchant-1", AppID: "app-1", Product: "NATIVE"}, PrivateKey: private, PublicKey: public, SerialNumber: "merchant-serial", PublicKeyID: "PUB_KEY_ID_fixture", APIv3Key: strings.Repeat("k", 32), NotifyURL: "https://merchant.example/notify"})
	if err != nil {
		t.Fatal(err)
	}
	p.now = func() time.Time { return now }
	plain := `{"appid":"app-1","mchid":"merchant-1","out_trade_no":"order-1","transaction_id":"trade-1","trade_type":"NATIVE","trade_state":"SUCCESS","success_time":"2026-09-27T12:00:00+08:00","amount":{"total":10000,"currency":"CNY"}}`
	makeBody := func(plain string) []byte {
		block, _ := aes.NewCipher([]byte(p.config.APIv3Key))
		aead, _ := cipher.NewGCM(block)
		nonce := "012345678901"
		sealed := aead.Seal(nil, []byte(nonce), []byte(plain), []byte("transaction"))
		raw, _ := json.Marshal(map[string]any{"id": "event-1", "event_type": "TRANSACTION.SUCCESS", "resource_type": "encrypt-resource", "resource": map[string]any{"algorithm": "AEAD_AES_256_GCM", "nonce": nonce, "associated_data": "transaction", "ciphertext": base64.StdEncoding.EncodeToString(sealed)}})
		return raw
	}
	raw := makeBody(plain)
	verify := func(body []byte, signBody []byte, serial string, ts time.Time, duplicate bool) error {
		r := httptest.NewRequest("POST", "https://merchant.example/notify", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		stamp := strconv.FormatInt(ts.Unix(), 10)
		r.Header.Set("Wechatpay-Timestamp", stamp)
		r.Header.Set("Wechatpay-Nonce", "nonce")
		r.Header.Set("Wechatpay-Serial", serial)
		r.Header.Set("Wechatpay-Signature", signFixture(t, key, stamp+"\nnonce\n"+string(signBody)+"\n"))
		if duplicate {
			r.Header.Add("Wechatpay-Serial", serial)
		}
		o, e := p.VerifyNotification(r)
		if e == nil && (o.State != "PAID" || o.AmountMinor != 10000) {
			t.Fatal("incorrect projection")
		}
		return e
	}
	if err = verify(raw, raw, p.config.PublicKeyID, now, false); err != nil {
		t.Fatal(err)
	}
	if verify(append(append([]byte(nil), raw...), ' '), raw, p.config.PublicKeyID, now, false) == nil {
		t.Fatal("reordered body accepted")
	}
	for _, scenario := range []struct {
		plain, serial string
		ts            time.Time
		duplicate     bool
	}{{plain, "unknown", now, false}, {plain, p.config.PublicKeyID, now.Add(-6 * time.Minute), false}, {plain, p.config.PublicKeyID, now, true}, {strings.Replace(plain, "merchant-1", "other", 1), p.config.PublicKeyID, now, false}, {strings.Replace(plain, `"total":10000`, `"total":1,"total":10000`, 1), p.config.PublicKeyID, now, false}} {
		body := makeBody(scenario.plain)
		if verify(body, body, scenario.serial, scenario.ts, scenario.duplicate) == nil {
			t.Fatal("accepted invalid callback")
		}
	}
}
