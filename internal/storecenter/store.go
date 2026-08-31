package storecenter

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	// MaxStoreNameCodePoints is shared with the HTTP and browser validation layers.
	MaxStoreNameCodePoints = 120
	// MaxStoreRegionCodePoints is shared with the HTTP and browser validation layers.
	MaxStoreRegionCodePoints = 64
	// MaxExternalStoreIDCodePoints is shared with the HTTP and browser validation layers.
	MaxExternalStoreIDCodePoints = 128

	MaxOrganizationIDBytes = 200
	MaxSubjectBytes        = 200
)

var (
	ErrNotFound              = errors.New("store not found")
	ErrAlreadyExists         = errors.New("store already exists")
	ErrVersionConflict       = errors.New("store version conflict")
	ErrInvalidTransition     = errors.New("invalid store lifecycle transition")
	ErrLimitReached          = errors.New("store limit reached")
	ErrDependencyUnavailable = errors.New("store dependency unavailable")
)

type Platform string

const PlatformShein Platform = "shein"

type StoreStatus string

const (
	StoreStatusProvisioning StoreStatus = "provisioning"
	StoreStatusActive       StoreStatus = "active"
	StoreStatusDisabled     StoreStatus = "disabled"
	StoreStatusDeleting     StoreStatus = "deleting"
)

// Store is the Organization-scoped Store Center aggregate. It contains only
// administrative metadata; provider authentication material belongs outside this domain.
type Store struct {
	ID                   string      `json:"id"`
	OrganizationID       string      `json:"organizationId"`
	CreatedBySubject     string      `json:"createdBySubject"`
	Name                 string      `json:"name"`
	Platform             Platform    `json:"platform"`
	Region               string      `json:"region"`
	ExternalStoreID      string      `json:"externalStoreId,omitempty"`
	CreateIdempotencyKey string      `json:"-"`
	Status               StoreStatus `json:"status"`
	Version              int64       `json:"version"`
}

type CreateStoreInput struct {
	ID                   string
	OrganizationID       string
	CreatedBySubject     string
	Name                 string
	Platform             string
	Region               string
	ExternalStoreID      string
	CreateIdempotencyKey string
}

func NewStore(input CreateStoreInput) (*Store, error) {
	id, err := canonicalUUID(input.ID)
	if err != nil {
		return nil, fmt.Errorf("store ID: %w", err)
	}
	organizationID, err := validateOpaqueIdentity("organization ID", input.OrganizationID, MaxOrganizationIDBytes)
	if err != nil {
		return nil, err
	}
	createdBySubject, err := validateOpaqueIdentity("subject", input.CreatedBySubject, MaxSubjectBytes)
	if err != nil {
		return nil, err
	}
	name, err := normalizeUserValue("name", input.Name, MaxStoreNameCodePoints, true)
	if err != nil {
		return nil, err
	}
	platform, err := normalizePlatform(input.Platform)
	if err != nil {
		return nil, err
	}
	region, err := normalizeUserValue("region", input.Region, MaxStoreRegionCodePoints, true)
	if err != nil {
		return nil, err
	}
	externalStoreID, err := normalizeUserValue("external store ID", input.ExternalStoreID, MaxExternalStoreIDCodePoints, false)
	if err != nil {
		return nil, err
	}
	createIdempotencyKey, err := canonicalUUID(input.CreateIdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("create idempotency key: %w", err)
	}

	return &Store{
		ID:                   id,
		OrganizationID:       organizationID,
		CreatedBySubject:     createdBySubject,
		Name:                 name,
		Platform:             platform,
		Region:               region,
		ExternalStoreID:      externalStoreID,
		CreateIdempotencyKey: createIdempotencyKey,
		Status:               StoreStatusProvisioning,
		Version:              1,
	}, nil
}

func (s *Store) TransitionTo(target StoreStatus) error {
	if !canTransition(s.Status, target) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, s.Status, target)
	}
	s.Status = target
	s.Version++
	return nil
}

func canTransition(current, target StoreStatus) bool {
	switch current {
	case StoreStatusProvisioning:
		return target == StoreStatusActive
	case StoreStatusActive:
		return target == StoreStatusDisabled || target == StoreStatusDeleting
	case StoreStatusDisabled:
		return target == StoreStatusActive || target == StoreStatusDeleting
	default:
		return false
	}
}

func canonicalUUID(value string) (string, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return "", errors.New("must be a canonical RFC 4122 UUID")
	}
	if parsed == uuid.Nil || parsed.String() != value {
		return "", errors.New("must be a non-nil canonical RFC 4122 UUID")
	}
	return parsed.String(), nil
}

func validateOpaqueIdentity(field, value string, maxBytes int) (string, error) {
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("%s must be valid UTF-8", field)
	}
	if value == "" || value != strings.TrimSpace(value) {
		return "", fmt.Errorf("%s must be nonblank and exactly trimmed", field)
	}
	if len(value) > maxBytes {
		return "", fmt.Errorf("%s exceeds %d bytes", field, maxBytes)
	}
	if containsControlCharacter(value) {
		return "", fmt.Errorf("%s contains a control character", field)
	}
	return value, nil
}

func normalizeUserValue(field, value string, maxCodePoints int, required bool) (string, error) {
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("%s must be valid UTF-8", field)
	}
	if containsControlCharacter(value) {
		return "", fmt.Errorf("%s contains a control character", field)
	}
	normalized := strings.TrimSpace(value)
	if required && normalized == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	if utf8.RuneCountInString(normalized) > maxCodePoints {
		return "", fmt.Errorf("%s exceeds %d Unicode code points", field, maxCodePoints)
	}
	return normalized, nil
}

func normalizePlatform(value string) (Platform, error) {
	if !utf8.ValidString(value) || containsControlCharacter(value) {
		return "", errors.New("platform is invalid")
	}
	normalized := strings.ToLower(strings.TrimSpace(value))
	if Platform(normalized) != PlatformShein {
		return "", errors.New("platform is unsupported")
	}
	return PlatformShein, nil
}

func containsControlCharacter(value string) bool {
	return strings.IndexFunc(value, unicode.IsControl) >= 0
}
