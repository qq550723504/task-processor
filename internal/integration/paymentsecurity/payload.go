package paymentsecurity

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"task-processor/internal/commercial/billing"
)

// PayloadProtection extracts the existing AES-GCM behavior. A consumer's fixed
// namespace and immutable order binding authenticate every encrypted payload.
type PayloadProtection struct {
	aead      cipher.AEAD
	namespace string
}

func NewPayloadProtection(key []byte, namespace string) (*PayloadProtection, error) {
	if len(key) != 32 || namespace != "wallet-topup:v1:" && namespace != "ecoservices-payment:v1:" && namespace != "ecoservices-merchant:v1:" {
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
	return &PayloadProtection{aead: aead, namespace: namespace}, nil
}
func (p *PayloadProtection) Seal(binding, plaintext string) ([]byte, error) {
	if p == nil || len(binding) < 1 || len(binding) > 256 || len(plaintext) == 0 || len(plaintext) > 64*1024 {
		return nil, billing.ErrInvalid
	}
	nonce := make([]byte, p.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, billing.ErrFeatureUnavailable
	}
	return p.aead.Seal(nonce, nonce, []byte(plaintext), []byte(p.namespace+binding)), nil
}
func (p *PayloadProtection) Open(binding string, sealed []byte) (string, error) {
	if p == nil || len(binding) < 1 || len(binding) > 256 {
		return "", billing.ErrInvalid
	}
	n := p.aead.NonceSize()
	if len(sealed) < n+p.aead.Overhead() || len(sealed) > 64*1024+n+p.aead.Overhead() {
		return "", billing.ErrInvalid
	}
	plain, err := p.aead.Open(nil, sealed[:n], sealed[n:], []byte(p.namespace+binding))
	if err != nil {
		return "", billing.ErrFeatureUnavailable
	}
	return string(plain), nil
}
