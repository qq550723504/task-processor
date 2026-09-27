package wallettopup

import (
	"bytes"
	"strings"
	"testing"
)

func TestAmountAndStrictJSON(t *testing.T) {
	for text, want := range map[string]int64{"0.01": 1, "10": 1000, "10.2": 1020, "92233720368547758.07": 9223372036854775807} {
		got, err := parseYuan(text)
		if err != nil || got != want {
			t.Fatalf("%q: %d %v", text, got, err)
		}
	}
	for _, text := range []string{"1e2", "1.001", "-1", "+1", " 1", "01.0", "NaN", "92233720368547758.08"} {
		if _, err := parseYuan(text); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
	for _, raw := range []string{`{"amount":{"total":1,"total":2}}`, `{"a":1} {"a":2}`, `{"a":1,"\u0061":2}`} {
		var v any
		if strictJSON([]byte(raw), &v) == nil {
			t.Fatalf("ambiguous JSON: %s", raw)
		}
	}
	var v any
	if err := strictJSON([]byte(`{"amount":{"total":1},"items":[{},2]}`), &v); err != nil {
		t.Fatal(err)
	}
}

func TestCheckoutProtectionBindsAttempt(t *testing.T) {
	p, err := NewPayloadProtection(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := p.Seal(strings.Repeat("a", 64), "weixin://wxpay/bizpayurl?pr=secret")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("secret")) {
		t.Fatal("plaintext stored")
	}
	if _, err = p.Open(strings.Repeat("b", 64), sealed); err == nil {
		t.Fatal("accepted another attempt")
	}
	sealed[20] ^= 1
	if _, err = p.Open(strings.Repeat("a", 64), sealed); err == nil {
		t.Fatal("accepted tampered ciphertext")
	}
}
