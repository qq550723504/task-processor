package servicepayments

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func signedServiceCallback(t *testing.T, key *rsa.PrivateKey, apiKey, plain string, now time.Time) *http.Request {
	t.Helper()
	block, _ := aes.NewCipher([]byte(apiKey))
	aead, _ := cipher.NewGCM(block)
	nonce := "nonce1234567"
	encrypted := aead.Seal(nil, []byte(nonce), []byte(plain), []byte("transaction"))
	raw, _ := json.Marshal(map[string]any{"id": "verified-event", "event_type": "TRANSACTION.SUCCESS", "resource_type": "encrypt-resource", "resource": map[string]string{"algorithm": "AEAD_AES_256_GCM", "nonce": nonce, "associated_data": "transaction", "ciphertext": base64.StdEncoding.EncodeToString(encrypted)}})
	ts := strconv.FormatInt(now.Unix(), 10)
	hash := sha256.Sum256([]byte(ts + "\nheader-nonce\n" + string(raw) + "\n"))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "https://platform.example/api/v1/payments/ecoservices/wechat/notify", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Wechatpay-Timestamp", ts)
	req.Header.Set("Wechatpay-Nonce", "header-nonce")
	req.Header.Set("Wechatpay-Serial", "PUB_KEY_ID_fixture")
	req.Header.Set("Wechatpay-Signature", base64.StdEncoding.EncodeToString(signature))
	return req
}
func TestServiceCallbackVerifiesExactPartnerPaymentBeforeAnyOwnerWrite(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p, o, _ := wechatServiceFixture()
	now := time.Now()
	p.publicKey = &key.PublicKey
	p.config.APIv3Key = "0123456789abcdef0123456789abcdef"
	p.now = func() time.Time { return now }
	plain := `{"sp_appid":"app","sp_mchid":"platform","sub_mchid":"sub","out_trade_no":"original-trade","transaction_id":"transaction","trade_state":"SUCCESS","trade_type":"NATIVE","success_time":"` + now.Format(time.RFC3339) + `","amount":{"total":100,"payer_total":100,"currency":"CNY","payer_currency":"CNY"}}`
	obs, err := p.VerifyServiceNotification(signedServiceCallback(t, key, p.config.APIv3Key, plain, now))
	if err != nil || !obs.Matches(o) || obs.EventID != "verified-event" {
		t.Fatalf("original paid notification lost: %+v %v", obs, err)
	}
	for _, mutate := range []func(*http.Request){func(r *http.Request) { r.Header.Set("Wechatpay-Serial", "other") }, func(r *http.Request) { r.Header.Add("Wechatpay-Nonce", "second") }, func(r *http.Request) { r.URL.RawQuery = "merchant=other" }, func(r *http.Request) { r.Header.Set("Wechatpay-Timestamp", "1") }, func(r *http.Request) { r.Body = http.NoBody }} {
		r := signedServiceCallback(t, key, p.config.APIv3Key, plain, now)
		mutate(r)
		if _, err := p.VerifyServiceNotification(r); err == nil {
			t.Fatal("unverified callback accepted")
		}
	}
}
