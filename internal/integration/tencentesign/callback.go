package tencentesign

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	domain "task-processor/internal/subjectverification"
	"time"
)

type Callback struct{ signingKey, encryptionKey []byte }

func NewCallback(signingKey, encryptionKey []byte) (*Callback, error) {
	if len(signingKey) < 16 || len(encryptionKey) != 32 {
		return nil, domain.ErrInvalid
	}
	return &Callback{append([]byte(nil), signingKey...), append([]byte(nil), encryptionKey...)}, nil
}

// Parse follows Tencent's encrypted callback protocol: authenticate the exact
// envelope first, then AES-256-CBC with the key's first 16 bytes as the IV.
func (c *Callback) Parse(raw []byte, signature string) (domain.Event, bool, error) {
	invalid := func() (domain.Event, bool, error) { return domain.Event{}, false, domain.ErrInvalid }
	if c == nil || len(raw) == 0 || len(raw) > 64*1024 || !strings.HasPrefix(signature, "sha256=") {
		return invalid()
	}
	sig, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return invalid()
	}
	mac := hmac.New(sha256.New, c.signingKey)
	_, _ = mac.Write(raw)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return invalid()
	}
	var envelope struct {
		Encrypt string `json:"encrypt"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return invalid()
	}
	encrypted, err := base64.StdEncoding.DecodeString(envelope.Encrypt)
	if err != nil || len(encrypted) == 0 || len(encrypted)%aes.BlockSize != 0 {
		return invalid()
	}
	block, err := aes.NewCipher(c.encryptionKey)
	if err != nil {
		return invalid()
	}
	plain := make([]byte, len(encrypted))
	cipher.NewCBCDecrypter(block, c.encryptionKey[:16]).CryptBlocks(plain, encrypted)
	padding := int(plain[len(plain)-1])
	if padding < 1 || padding > aes.BlockSize || padding > len(plain) {
		return invalid()
	}
	for _, b := range plain[len(plain)-padding:] {
		if int(b) != padding {
			return invalid()
		}
	}
	plain = plain[:len(plain)-padding]
	var event struct {
		MsgId, MsgType, MsgVersion string
		MsgData                    struct {
			OrganizationId, OrganizationName, UniformSocialCreditCode, AdminMobile, AdminUserId, UserData string
			CreateTime                                                                                    int64
		}
	}
	if json.Unmarshal(plain, &event) != nil || event.MsgId == "" {
		return invalid()
	}
	if event.MsgType != "CreateOrganization" || event.MsgVersion != "CustomApp" {
		return domain.Event{}, false, nil
	}
	d := event.MsgData
	if d.UserData == "" {
		return domain.Event{}, false, nil
	} // Contract-signing callbacks are outside this flow.
	if d.CreateTime <= 0 {
		return invalid()
	}
	digest := sha256.Sum256(raw)
	return domain.Event{MessageID: event.MsgId, Digest: hex.EncodeToString(digest[:]), Correlation: d.UserData, CompanyName: d.OrganizationName, CreditCode: d.UniformSocialCreditCode, Phone: d.AdminMobile, ProviderOrganizationID: d.OrganizationId, ProviderAdminID: d.AdminUserId, VerifiedAt: time.Unix(d.CreateTime, 0).UTC()}, true, nil
}
