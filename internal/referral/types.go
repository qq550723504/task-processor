// Package referral owns personal attribution; organizations are not ownership keys.
package referral

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalid         = errors.New("referral_invalid")
	ErrConflict        = errors.New("referral_conflict")
	ErrUnavailable     = errors.New("referral_unavailable")
	ErrMissing         = errors.New("referral_missing")
	ErrUnauthenticated = errors.New("referral_authentication_required")
	ErrExpired         = errors.New("referral_expired")
	ErrPending         = errors.New("referral_verification_pending")
	ErrUnknown         = errors.New("referral_outcome_unknown")
	ErrLimited         = errors.New("referral_capacity_exceeded")
)

type Intent struct {
	ID, Issuer, Instance, Organization, Subject, Referrer       string
	KeyHash, EmailHash, Fingerprint, SecretHash, KeyID          string
	Ciphertext                                                  []byte
	CreatedAt, CreateExpiresAt, CompletionExpiresAt, LeaseUntil time.Time
	State                                                       string
}

type Receipt struct {
	IntentID, Issuer, Subject, Referrer, Fingerprint string
	BoundAt                                          time.Time
}

type Projection struct {
	Code                 string
	Count                int64
	EarningsAvailability string
}

// Store operations confirm a durable commit before returning success. An unknown
// commit returns ErrUnknown; callers must re-read instead of assuming rollback.
type Store interface {
	AllowIP(context.Context, string, time.Time) error
	Admit(context.Context, Intent, string) (Intent, error)
	Find(context.Context, string, string, string) (Intent, error)
	// Claim returns a database-clock lease identity; zero means another caller owns it.
	Claim(context.Context, string) (time.Time, error)
	// PermitCreate rechecks this lease and the fixed creation deadline after readback.
	// The duration is database time remaining, bounded by the lease; the caller
	// must subtract elapsed round-trip time and use a monotonic dispatch deadline.
	PermitCreate(context.Context, string, time.Time) (time.Duration, error)
	Created(context.Context, string) error
	Receipt(context.Context, string, string) (Receipt, error)
	Consume(context.Context, Intent, time.Time) (Receipt, error)
	Read(context.Context, string, string) (Projection, error)
	CreateCode(context.Context, string, string, string) (string, error)
	Cleanup(context.Context, time.Time) error
}
