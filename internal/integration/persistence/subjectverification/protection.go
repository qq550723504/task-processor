package subjectverification

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

type Protection struct {
	aead      cipher.AEAD
	digestKey []byte
}

func NewProtection(key []byte) (*Protection, error) {
	if len(key) != 32 {
		return nil, errors.New("verification protection requires a 32-byte key")
	}
	encryption, err := hkdf.Key(sha256.New, key, nil, "subject-verification/link/v1", 32)
	if err != nil {
		return nil, err
	}
	digest, err := hkdf.Key(sha256.New, key, nil, "subject-verification/digest/v1", 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(encryption)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Protection{aead, digest}, nil
}
func (p *Protection) Digest(purpose, value string) string {
	h := hmac.New(sha256.New, p.digestKey)
	h.Write([]byte(purpose + "\x00" + value))
	return hex.EncodeToString(h.Sum(nil))
}
func (p *Protection) Seal(binding, value string) ([]byte, error) {
	nonce := make([]byte, p.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return p.aead.Seal(nonce, nonce, []byte(value), []byte(binding)), nil
}
func (p *Protection) Open(binding string, value []byte) (string, error) {
	n := p.aead.NonceSize()
	if len(value) < n+p.aead.Overhead() {
		return "", errors.New("invalid verification ciphertext")
	}
	plain, err := p.aead.Open(nil, value[:n], value[n:], []byte(binding))
	return string(plain), err
}
