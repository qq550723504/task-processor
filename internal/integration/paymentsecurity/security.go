// Package paymentsecurity contains shared bounded provider parsing and transport.
package paymentsecurity

import (
	"crypto/rsa"
	"github.com/go-pay/gopay/pkg/xhttp"
	wechat "github.com/go-pay/gopay/wechat/v3"
	"net/http"
	"strconv"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/httproute"
	"time"
)

const maxCallbackBytes = 128 * 1024

func HTTP() *xhttp.Client {
	c := xhttp.NewClient().SetTimeout(10 * time.Second)
	c.HttpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return billing.ErrInvalid }
	return c
}
func VerifiedWeChatError(s *wechat.SignInfo, publicKeyID string, key *rsa.PublicKey, now time.Time, expectedCode string) bool {
	if !VerifiedWeChatResponse(s, publicKeyID, key, now) {
		return false
	}
	var body struct {
		Code string `json:"code"`
	}
	return StrictJSON([]byte(s.SignBody), &body) == nil && body.Code == expectedCode
}

func VerifiedWeChatResponse(s *wechat.SignInfo, publicKeyID string, key *rsa.PublicKey, now time.Time) bool {
	if s == nil || key == nil || s.HeaderSerial != publicKeyID || s.HeaderNonce == "" {
		return false
	}
	ts, err := strconv.ParseInt(s.HeaderTimestamp, 10, 64)
	if err != nil || time.Unix(ts, 0).Before(now.Add(-5*time.Minute)) || time.Unix(ts, 0).After(now.Add(5*time.Minute)) {
		return false
	}
	var body any
	return StrictJSON([]byte(s.SignBody), &body) == nil && wechat.V3VerifySignByPK(s.HeaderTimestamp, s.HeaderNonce, s.SignBody, s.HeaderSignature, key) == nil
}
func StrictJSON(raw []byte, out any) error {
	if httproute.DecodeJSON(raw, out, maxCallbackBytes, false) != nil {
		return billing.ErrInvalid
	}
	return nil
}
