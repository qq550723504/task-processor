package shein

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"task-processor/internal/storecenter"
)

type CredentialProtection struct {
	keyID string
	aead  cipher.AEAD
}

func NewCredentialProtection(keyID string, key []byte) (*CredentialProtection, error) {
	if len(key) != 32 || len(keyID) < 1 || len(keyID) > 128 || strings.TrimSpace(keyID) != keyID {
		return nil, errors.New("official credential protection configuration unavailable")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &CredentialProtection{keyID: keyID, aead: aead}, nil
}

type sealedCredential struct{ AppID, OpenKeyID, SecretKey, SupplierID string }

func associatedData(a storecenter.OfficialConnectionAttempt, keyID string) []byte {
	data, _ := json.Marshal([]any{"shein-merchant-v1", a.OrganizationID, a.StoreID, a.AttemptID, a.AppID, a.AppVersion, a.ActorID, a.MemberID, a.ConnectionVersion, keyID})
	return data
}
func (p *CredentialProtection) Seal(a storecenter.OfficialConnectionAttempt, c storecenter.OfficialMerchantCredential) (string, string, error) {
	if c.AppID != a.AppID || c.OpenKeyID == "" || c.SecretKey == "" || c.SupplierID == "" {
		return "", "", storecenter.ErrOfficialExchangeUnknown
	}
	plain, err := json.Marshal(sealedCredential{c.AppID, c.OpenKeyID, c.SecretKey, c.SupplierID})
	if err != nil {
		return "", "", err
	}
	defer clear(plain)
	nonce := make([]byte, p.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", storecenter.ErrOfficialConnectionUnavailable
	}
	encrypted := p.aead.Seal(nonce, nonce, plain, associatedData(a, p.keyID))
	return p.keyID, base64.StdEncoding.EncodeToString(encrypted), nil
}
func (p *CredentialProtection) Open(a storecenter.OfficialConnectionAttempt, keyID, encrypted string) (storecenter.OfficialMerchantCredential, error) {
	if keyID != p.keyID || len(encrypted) > 16384 {
		return storecenter.OfficialMerchantCredential{}, storecenter.ErrOfficialConnectionUnavailable
	}
	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil || len(data) < p.aead.NonceSize()+p.aead.Overhead() {
		return storecenter.OfficialMerchantCredential{}, storecenter.ErrOfficialConnectionUnavailable
	}
	plain, err := p.aead.Open(nil, data[:p.aead.NonceSize()], data[p.aead.NonceSize():], associatedData(a, keyID))
	if err != nil {
		return storecenter.OfficialMerchantCredential{}, storecenter.ErrOfficialConnectionUnavailable
	}
	defer clear(plain)
	var c sealedCredential
	if err := json.Unmarshal(plain, &c); err != nil || c.AppID != a.AppID || c.OpenKeyID == "" || c.SecretKey == "" || c.SupplierID == "" {
		return storecenter.OfficialMerchantCredential{}, storecenter.ErrOfficialConnectionUnavailable
	}
	return storecenter.OfficialMerchantCredential{AppID: c.AppID, OpenKeyID: c.OpenKeyID, SecretKey: c.SecretKey, SupplierID: c.SupplierID}, nil
}
