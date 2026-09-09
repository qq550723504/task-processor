package sourceaccountregistry

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MaxDisplayNameBytes        = 120
	MaxOrganizationIDBytes     = 128
	MaxActorSubjectBytes       = 256
	MaxAccountsPerOrganization = 100
	DefaultPageLimit           = 20
	MaxPageLimit               = 100
)

var (
	ErrInvalid                = errors.New("invalid source account request")
	ErrAuthenticationRequired = errors.New("source account authentication required")
	ErrForbidden              = errors.New("source account permission denied")
	ErrNotFound               = errors.New("source account not found")
	ErrIdempotencyConflict    = errors.New("source account idempotency conflict")
	ErrVersionConflict        = errors.New("source account version conflict")
	ErrInvalidTransition      = errors.New("source account invalid transition")
	ErrResourceLimitReached   = errors.New("source account resource limit reached")
	ErrTooLarge               = errors.New("source account input too large")
	ErrUnavailable            = errors.New("source account dependency unavailable")
	ErrOutcomeUnknown         = errors.New("source account transaction outcome unknown")
)

type Platform string

const Platform1688 Platform = "1688"

type ManagementStatus string

const (
	ManagementStatusEnabled  ManagementStatus = "enabled"
	ManagementStatusDisabled ManagementStatus = "disabled"
)

type ConnectionStatus string

const ConnectionStatusPending ConnectionStatus = "pending_connection"

type Account struct {
	ID               string
	OrganizationID   string
	Platform         Platform
	DisplayName      string
	ManagementStatus ManagementStatus
	ConnectionStatus ConnectionStatus
	Version          int64
	CreatedBy        string
	UpdatedBy        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewAccount(id, organizationID, actorSubject, displayName string, platform Platform, now time.Time) (Account, error) {
	displayName, err := NormalizeDisplayName(displayName)
	if err != nil || !validUUIDv7(id) || !validScopeValue(organizationID, MaxOrganizationIDBytes) || !validScopeValue(actorSubject, MaxActorSubjectBytes) || platform != Platform1688 || now.IsZero() {
		return Account{}, ErrInvalid
	}
	now = now.UTC().Truncate(time.Microsecond)
	return Account{
		ID: id, OrganizationID: organizationID, Platform: platform, DisplayName: displayName,
		ManagementStatus: ManagementStatusEnabled, ConnectionStatus: ConnectionStatusPending,
		Version: 1, CreatedBy: actorSubject, UpdatedBy: actorSubject, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (a Account) Validate() error {
	if !validUUIDv7(a.ID) || !validScopeValue(a.OrganizationID, MaxOrganizationIDBytes) || !validScopeValue(a.CreatedBy, MaxActorSubjectBytes) || !validScopeValue(a.UpdatedBy, MaxActorSubjectBytes) {
		return ErrUnavailable
	}
	if a.Platform != Platform1688 || a.ConnectionStatus != ConnectionStatusPending || a.Version <= 0 || a.CreatedAt.IsZero() || a.UpdatedAt.IsZero() || a.UpdatedAt.Before(a.CreatedAt) {
		return ErrUnavailable
	}
	if a.ManagementStatus != ManagementStatusEnabled && a.ManagementStatus != ManagementStatusDisabled {
		return ErrUnavailable
	}
	normalized, err := NormalizeDisplayName(a.DisplayName)
	if err != nil || normalized != a.DisplayName {
		return ErrUnavailable
	}
	return nil
}

func (a Account) Transition(expectedVersion int64, target ManagementStatus, actor string, now time.Time) (Account, error) {
	if err := a.Validate(); err != nil {
		return Account{}, err
	}
	if expectedVersion <= 0 || expectedVersion != a.Version {
		return Account{}, ErrVersionConflict
	}
	if target != ManagementStatusEnabled && target != ManagementStatusDisabled {
		return Account{}, ErrInvalid
	}
	if a.ManagementStatus == target {
		return Account{}, ErrInvalidTransition
	}
	if !validScopeValue(actor, MaxActorSubjectBytes) || now.IsZero() {
		return Account{}, ErrInvalid
	}
	now = now.UTC().Truncate(time.Microsecond)
	if now.Before(a.CreatedAt) || a.Version == int64(^uint64(0)>>1) {
		return Account{}, ErrUnavailable
	}
	a.ManagementStatus = target
	a.Version++
	a.UpdatedBy = actor
	a.UpdatedAt = now
	return a, nil
}

func NormalizeDisplayName(value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", ErrInvalid
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", ErrInvalid
		}
	}
	value = strings.TrimSpace(value)
	if value == "" || len([]byte(value)) > MaxDisplayNameBytes {
		return "", ErrInvalid
	}
	return value, nil
}

func validScopeValue(value string, maxBytes int) bool {
	return value != "" && strings.TrimSpace(value) == value && utf8.ValidString(value) && len([]byte(value)) <= maxBytes
}

func validCanonicalUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil && parsed.String() == value && parsed.Variant() == uuid.RFC4122
}

func validUUIDv7(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil && parsed.String() == value && parsed.Variant() == uuid.RFC4122 && parsed.Version() == 7
}
