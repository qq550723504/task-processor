package paymentsecurity

import (
	"crypto/rsa"
	wechat "github.com/go-pay/gopay/wechat/v3"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"task-processor/internal/commercial/billing"
	"time"
)

// VerifiedWeChatNotification extracts the existing signed-envelope and AEAD
// checks; callers still validate their own product and original purchase.
func VerifiedWeChatNotification(req *http.Request, publicKeyID string, key *rsa.PublicKey, apiV3Key string, now time.Time) (eventID, eventType string, plain []byte, err error) {
	invalid := billing.ErrInvalid
	if req == nil || req.Body == nil || key == nil || len(apiV3Key) != 32 || req.Method != http.MethodPost || req.URL.RawQuery != "" || req.URL.ForceQuery || req.Header.Get("Content-Encoding") != "" || len(req.Header.Values("Content-Type")) != 1 || req.ContentLength > maxCallbackBytes {
		return "", "", nil, invalid
	}
	media, params, e := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if e != nil || media != "application/json" || len(params) > 1 || params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8") {
		return "", "", nil, invalid
	}
	raw, e := io.ReadAll(io.LimitReader(req.Body, maxCallbackBytes+1))
	if e != nil || len(raw) > maxCallbackBytes {
		return "", "", nil, invalid
	}
	for _, name := range []string{"Wechatpay-Timestamp", "Wechatpay-Nonce", "Wechatpay-Signature", "Wechatpay-Serial"} {
		if len(req.Header.Values(name)) != 1 || req.Header.Get(name) == "" || len(req.Header.Get(name)) > 4096 {
			return "", "", nil, invalid
		}
	}
	ts, e := strconv.ParseInt(req.Header.Get("Wechatpay-Timestamp"), 10, 64)
	if e != nil || req.Header.Get("Wechatpay-Serial") != publicKeyID || time.Unix(ts, 0).Before(now.Add(-5*time.Minute)) || time.Unix(ts, 0).After(now.Add(5*time.Minute)) {
		return "", "", nil, invalid
	}
	if wechat.V3VerifySignByPK(req.Header.Get("Wechatpay-Timestamp"), req.Header.Get("Wechatpay-Nonce"), string(raw), req.Header.Get("Wechatpay-Signature"), key) != nil {
		return "", "", nil, invalid
	}
	var envelope struct {
		ID           string           `json:"id"`
		EventType    string           `json:"event_type"`
		ResourceType string           `json:"resource_type"`
		Resource     *wechat.Resource `json:"resource"`
	}
	if StrictJSON(raw, &envelope) != nil || envelope.ID == "" || len(envelope.ID) > 128 || strings.TrimSpace(envelope.ID) != envelope.ID || strings.ContainsRune(envelope.ID, 0) || envelope.ResourceType != "encrypt-resource" || envelope.Resource == nil || envelope.Resource.Algorithm != "AEAD_AES_256_GCM" || len(envelope.Resource.Nonce) != 12 {
		return "", "", nil, invalid
	}
	plain, e = wechat.V3DecryptNotifyCipherTextToBytes(envelope.Resource.Ciphertext, envelope.Resource.Nonce, envelope.Resource.AssociatedData, apiV3Key)
	if e != nil || len(plain) > maxCallbackBytes {
		return "", "", nil, invalid
	}
	return envelope.ID, envelope.EventType, plain, nil
}
