// Package wallettopup adapts signed provider facts to the commercial top-up contract.
package wallettopup

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/commercial/billing"
	"task-processor/internal/ledger/money"
)

const maxCallbackBytes = 128 * 1024

type payloadProtection struct{ aead cipher.AEAD }

func NewPayloadProtection(key []byte) (billing.TopUpPayloadProtection, error) {
	if len(key) != 32 {
		return nil, billing.ErrInvalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, billing.ErrInvalid
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, billing.ErrInvalid
	}
	return payloadProtection{aead}, nil
}
func (p payloadProtection) Seal(binding, plaintext string) ([]byte, error) {
	if len(binding) != 64 || len(plaintext) == 0 || len(plaintext) > 64*1024 {
		return nil, billing.ErrInvalid
	}
	nonce := make([]byte, p.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, billing.ErrFeatureUnavailable
	}
	return p.aead.Seal(nonce, nonce, []byte(plaintext), []byte("wallet-topup:v1:"+binding)), nil
}
func (p payloadProtection) Open(binding string, sealed []byte) (string, error) {
	n := p.aead.NonceSize()
	if len(sealed) < n+p.aead.Overhead() || len(sealed) > 64*1024+n+p.aead.Overhead() {
		return "", billing.ErrInvalid
	}
	plain, err := p.aead.Open(nil, sealed[:n], sealed[n:], []byte("wallet-topup:v1:"+binding))
	if err != nil {
		return "", billing.ErrFeatureUnavailable
	}
	return string(plain), nil
}

// Provider decimals are converted without floating point, exponents or rounding.
func parseYuan(raw string) (int64, error) {
	whole, fraction, hasDot := strings.Cut(raw, ".")
	if whole == "" || len(whole) > 1 && whole[0] == '0' || hasDot && (len(fraction) < 1 || len(fraction) > 2) {
		return 0, billing.ErrInvalid
	}
	for _, v := range whole + fraction {
		if v < '0' || v > '9' {
			return 0, billing.ErrInvalid
		}
	}
	fraction += strings.Repeat("0", 2-len(fraction))
	minor, err := strconv.ParseInt(whole+fraction, 10, 64)
	if err != nil {
		return 0, billing.ErrInvalid
	}
	return minor, nil
}
func formatYuan(minor int64) string { return fmt.Sprintf("%d.%02d", minor/100, minor%100) }

// encoding/json accepts duplicate keys. Reject ambiguity before consuming signed JSON.
func strictJSON(raw []byte, out any) error {
	if len(raw) == 0 || len(raw) > maxCallbackBytes {
		return billing.ErrInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 24 {
			return billing.ErrInvalid
		}
		tok, err := dec.Token()
		if err != nil {
			return billing.ErrInvalid
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				key, err := dec.Token()
				s, ok := key.(string)
				if err != nil || !ok || seen[s] {
					return billing.ErrInvalid
				}
				seen[s] = true
				if err = walk(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for dec.More() {
				if err = walk(depth + 1); err != nil {
					return err
				}
			}
		default:
			return billing.ErrInvalid
		}
		_, err = dec.Token()
		return err
	}
	if err := walk(0); err != nil {
		return billing.ErrInvalid
	}
	if _, err := dec.Token(); err != io.EOF {
		return billing.ErrInvalid
	}
	if json.Unmarshal(raw, out) != nil {
		return billing.ErrInvalid
	}
	return nil
}
func callbackBody(r *http.Request, mediaType string) ([]byte, error) {
	if r == nil || r.Method != http.MethodPost || r.URL.RawQuery != "" || len(r.Header.Values("Content-Type")) != 1 || strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]) != mediaType || r.ContentLength > maxCallbackBytes {
		return nil, billing.ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxCallbackBytes+1))
	if err != nil || len(raw) > maxCallbackBytes {
		return nil, billing.ErrInvalid
	}
	return raw, nil
}
func validHTTPS(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Fragment == ""
}
func action(a billing.TopUpPaymentAttempt, kind, payload string) billing.CheckoutAction {
	return billing.CheckoutAction{Kind: kind, Payload: payload, OrderID: a.OrderID, AttemptID: a.AttemptID, Provider: a.Merchant.Provider, ExpiresAt: a.ExpiresAt, RequestFingerprint: a.Fingerprint()}
}
func observationID(o billing.ProviderObservation) billing.ProviderObservation {
	o.EventID = "fact:" + money.TopUpFingerprint(o)
	return o
}

// Absence cannot close an attempt while its original checkout could still be
// presented to the channel. The absolute provider expiry is never extended.
func absentPayment(a billing.TopUpPaymentAttempt, now time.Time, verification string) billing.ProviderObservation {
	state := "NOT_FOUND"
	if !a.ExpiresAt.IsZero() && !now.Before(a.ExpiresAt) {
		state = "CLOSED"
	}
	return observationID(billing.ProviderObservation{Merchant: a.Merchant, MerchantOrderID: a.MerchantOrderID, Kind: "PAYMENT", State: state, Currency: a.Currency, VerificationVersion: verification})
}
