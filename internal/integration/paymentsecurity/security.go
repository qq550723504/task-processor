// Package paymentsecurity contains shared bounded provider parsing and transport.
package paymentsecurity

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"github.com/go-pay/gopay/pkg/xhttp"
	wechat "github.com/go-pay/gopay/wechat/v3"
	"io"
	"net/http"
	"strconv"
	"task-processor/internal/commercial/billing"
	"time"
)

const maxCallbackBytes = 128 * 1024

func HTTP() *xhttp.Client {
	c := xhttp.NewClient().SetTimeout(10 * time.Second)
	c.HttpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return billing.ErrInvalid }
	return c
}
func VerifiedWeChatError(s *wechat.SignInfo, publicKeyID string, key *rsa.PublicKey, now time.Time, expectedCode string) bool {
	if s == nil || key == nil || s.HeaderSerial != publicKeyID || s.HeaderNonce == "" {
		return false
	}
	ts, err := strconv.ParseInt(s.HeaderTimestamp, 10, 64)
	if err != nil || time.Unix(ts, 0).Before(now.Add(-5*time.Minute)) || time.Unix(ts, 0).After(now.Add(5*time.Minute)) {
		return false
	}
	var body struct {
		Code string `json:"code"`
	}
	return StrictJSON([]byte(s.SignBody), &body) == nil && body.Code == expectedCode && wechat.V3VerifySignByPK(s.HeaderTimestamp, s.HeaderNonce, s.SignBody, s.HeaderSignature, key) == nil
}
func StrictJSON(raw []byte, out any) error {
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
