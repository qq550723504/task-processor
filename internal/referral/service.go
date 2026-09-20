package referral

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"
)

type Service struct{ Store Store }

func (s Service) Read(ctx context.Context, issuer, subject string) (Projection, error) {
	if issuer == "" || subject == "" {
		return Projection{}, ErrUnauthenticated
	}
	if s.Store == nil || ctx == nil {
		return Projection{}, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := s.Store.Read(ctx, issuer, subject)
	out.EarningsAvailability = "unavailable"
	return out, err
}
func (s Service) CreateCode(ctx context.Context, issuer, subject string) (string, error) {
	if issuer == "" || subject == "" {
		return "", ErrUnauthenticated
	}
	if s.Store == nil || ctx == nil {
		return "", ErrUnavailable
	}
	code := make([]byte, 32)
	if _, err := rand.Read(code); err != nil {
		return "", ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return s.Store.CreateCode(ctx, issuer, subject, base64.RawURLEncoding.EncodeToString(code))
}
