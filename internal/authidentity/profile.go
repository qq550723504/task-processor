package authidentity

import (
	"context"
	"errors"
)

var (
	ErrProfileNotConfigured  = errors.New("profile reader is not configured")
	ErrProfileAuthentication = errors.New("profile authentication required")
	ErrProfilePermission     = errors.New("profile permission denied")
	ErrProfileUnavailable    = errors.New("profile dependency unavailable")
	ErrProfileInvalid        = errors.New("profile response invalid")
)

// SelfProfile is the provider-disclosed subset of the current user's claims.
// It is neither a local identity record nor an authorization source.
type SelfProfile struct {
	UserID              string
	DisplayName         *string
	Email               *string
	EmailVerified       *bool
	PhoneNumber         *string
	PhoneNumberVerified *bool
}

type SelfProfileReader interface {
	ReadSelf(ctx context.Context, bearerToken, verifiedSubject string) (SelfProfile, error)
}
