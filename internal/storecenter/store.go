package storecenter

import (
	"errors"
	"fmt"
	"strings"
	"time"
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

type LifecycleStatus string

// StoreStatus remains an alias while the aggregate exposes the authoritative
// LifecycleStatus field name in its persistence snapshot.
type StoreStatus = LifecycleStatus

const (
	StoreStatusProvisioning LifecycleStatus = "provisioning"
	StoreStatusActive       LifecycleStatus = "active"
	StoreStatusDisabled     LifecycleStatus = "disabled"
	StoreStatusDeleting     LifecycleStatus = "deleting"
)

// Store is the Organization-scoped Store Center aggregate. Its state is kept
// private so identity and lifecycle changes can only occur through its rules.
type Store struct {
	id                   string
	organizationID       string
	name                 string
	platform             Platform
	region               string
	externalStoreID      string
	lifecycleStatus      LifecycleStatus
	connectionRef        string
	quotaAllocationID    string
	version              int64
	createdBy            string
	updatedBy            string
	createdAt            time.Time
	updatedAt            time.Time
	deletedAt            *time.Time
	createIdempotencyKey string
	deleteOperationKey   string
}

// StoreSnapshot is the explicit persistence rehydration boundary. It is not
// an HTTP DTO; Task 6 owns the separate public response contract.
type StoreSnapshot struct {
	ID                   string
	OrganizationID       string
	Name                 string
	Platform             Platform
	Region               string
	ExternalStoreID      string
	LifecycleStatus      LifecycleStatus
	ConnectionRef        string
	QuotaAllocationID    string
	Version              int64
	CreatedBy            string
	UpdatedBy            string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            *time.Time
	CreateIdempotencyKey string
	DeleteOperationKey   string
}

type CreateStoreInput struct {
	ID                   string
	OrganizationID       string
	ActorSubject         string
	Name                 string
	Platform             string
	Region               string
	ExternalStoreID      string
	CreateIdempotencyKey string
	QuotaAllocationID    string
	OccurredAt           time.Time
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
	actorSubject, err := validateOpaqueIdentity("actor subject", input.ActorSubject, MaxSubjectBytes)
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
	quotaAllocationID, err := canonicalUUID(input.QuotaAllocationID)
	if err != nil {
		return nil, fmt.Errorf("quota allocation ID: %w", err)
	}
	if input.OccurredAt.IsZero() {
		return nil, errors.New("occurred at is required")
	}

	return newStoreFromSnapshot(StoreSnapshot{
		ID:                   id,
		OrganizationID:       organizationID,
		Name:                 name,
		Platform:             platform,
		Region:               region,
		ExternalStoreID:      externalStoreID,
		LifecycleStatus:      StoreStatusProvisioning,
		ConnectionRef:        "",
		QuotaAllocationID:    quotaAllocationID,
		Version:              1,
		CreatedBy:            actorSubject,
		UpdatedBy:            actorSubject,
		CreatedAt:            input.OccurredAt,
		UpdatedAt:            input.OccurredAt,
		CreateIdempotencyKey: createIdempotencyKey,
	})
}

// RehydrateStore reconstructs a validated aggregate from its persistence
// boundary without exposing mutable aggregate fields.
func RehydrateStore(snapshot StoreSnapshot) (*Store, error) {
	return newStoreFromSnapshot(snapshot)
}

func (s *Store) ID() string                       { return s.id }
func (s *Store) OrganizationID() string           { return s.organizationID }
func (s *Store) Name() string                     { return s.name }
func (s *Store) Platform() Platform               { return s.platform }
func (s *Store) Region() string                   { return s.region }
func (s *Store) ExternalStoreID() string          { return s.externalStoreID }
func (s *Store) LifecycleStatus() LifecycleStatus { return s.lifecycleStatus }
func (s *Store) ConnectionRef() string            { return s.connectionRef }
func (s *Store) QuotaAllocationID() string        { return s.quotaAllocationID }
func (s *Store) Version() int64                   { return s.version }
func (s *Store) CreatedBy() string                { return s.createdBy }
func (s *Store) UpdatedBy() string                { return s.updatedBy }
func (s *Store) CreatedAt() time.Time             { return s.createdAt }
func (s *Store) UpdatedAt() time.Time             { return s.updatedAt }
func (s *Store) CreateIdempotencyKey() string     { return s.createIdempotencyKey }
func (s *Store) DeleteOperationKey() string       { return s.deleteOperationKey }
func (s *Store) DeletedAt() *time.Time            { return copyTimePointer(s.deletedAt) }

func (s *Store) Snapshot() StoreSnapshot {
	return StoreSnapshot{
		ID:                   s.id,
		OrganizationID:       s.organizationID,
		Name:                 s.name,
		Platform:             s.platform,
		Region:               s.region,
		ExternalStoreID:      s.externalStoreID,
		LifecycleStatus:      s.lifecycleStatus,
		ConnectionRef:        s.connectionRef,
		QuotaAllocationID:    s.quotaAllocationID,
		Version:              s.version,
		CreatedBy:            s.createdBy,
		UpdatedBy:            s.updatedBy,
		CreatedAt:            s.createdAt,
		UpdatedAt:            s.updatedAt,
		DeletedAt:            copyTimePointer(s.deletedAt),
		CreateIdempotencyKey: s.createIdempotencyKey,
		DeleteOperationKey:   s.deleteOperationKey,
	}
}

// EditBasic applies the aggregate-owned mutable Store profile. A normalized
// no-op is accepted without changing provenance or version.
func (s *Store) EditBasic(name, region, actorSubject string, occurredAt time.Time) (bool, error) {
	if s.lifecycleStatus != StoreStatusActive && s.lifecycleStatus != StoreStatusDisabled {
		return false, ErrInvalidTransition
	}
	normalizedName, err := normalizeUserValue("name", name, MaxStoreNameCodePoints, true)
	if err != nil {
		return false, err
	}
	normalizedRegion, err := normalizeUserValue("region", region, MaxStoreRegionCodePoints, true)
	if err != nil {
		return false, err
	}
	actorSubject, err = validateOpaqueIdentity("actor subject", actorSubject, MaxSubjectBytes)
	if err != nil {
		return false, err
	}
	if occurredAt.IsZero() || occurredAt.Before(s.updatedAt) {
		return false, errors.New("edit time must not precede the last update")
	}
	if normalizedName == s.name && normalizedRegion == s.region {
		return false, nil
	}
	s.name = normalizedName
	s.region = normalizedRegion
	s.updatedBy = actorSubject
	s.updatedAt = occurredAt
	s.version++
	return true, nil
}

// BeginDelete binds a single canonical operation to the destructive state.
// The same key is an idempotent aggregate replay; no other key may take over.
func (s *Store) BeginDelete(operationKey, actorSubject string, occurredAt time.Time) error {
	operationKey, err := canonicalUUID(operationKey)
	if err != nil {
		return fmt.Errorf("delete operation key: %w", err)
	}
	if s.lifecycleStatus == StoreStatusDeleting {
		if s.deleteOperationKey == operationKey {
			return nil
		}
		return ErrInvalidTransition
	}
	if s.lifecycleStatus != StoreStatusActive && s.lifecycleStatus != StoreStatusDisabled {
		return ErrInvalidTransition
	}
	actorSubject, err = validateOpaqueIdentity("actor subject", actorSubject, MaxSubjectBytes)
	if err != nil {
		return err
	}
	if occurredAt.IsZero() || occurredAt.Before(s.updatedAt) {
		return errors.New("delete time must not precede the last update")
	}
	s.lifecycleStatus = StoreStatusDeleting
	s.deleteOperationKey = operationKey
	s.updatedBy = actorSubject
	s.updatedAt = occurredAt
	s.version++
	return nil
}

func (s *Store) TransitionTo(target LifecycleStatus, actorSubject string, occurredAt time.Time) error {
	if !canTransition(s.lifecycleStatus, target) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, s.lifecycleStatus, target)
	}
	actorSubject, err := validateOpaqueIdentity("actor subject", actorSubject, MaxSubjectBytes)
	if err != nil {
		return err
	}
	if occurredAt.IsZero() || occurredAt.Before(s.updatedAt) {
		return errors.New("transition time must not precede the last update")
	}
	s.lifecycleStatus = target
	s.updatedBy = actorSubject
	s.updatedAt = occurredAt
	s.version++
	return nil
}

func newStoreFromSnapshot(snapshot StoreSnapshot) (*Store, error) {
	id, err := canonicalUUID(snapshot.ID)
	if err != nil {
		return nil, fmt.Errorf("store ID: %w", err)
	}
	organizationID, err := validateOpaqueIdentity("organization ID", snapshot.OrganizationID, MaxOrganizationIDBytes)
	if err != nil {
		return nil, err
	}
	name, err := validateNormalizedUserValue("name", snapshot.Name, MaxStoreNameCodePoints, true)
	if err != nil {
		return nil, err
	}
	platform, err := normalizePlatform(string(snapshot.Platform))
	if err != nil || platform != snapshot.Platform {
		return nil, errors.New("platform must be normalized and supported")
	}
	region, err := validateNormalizedUserValue("region", snapshot.Region, MaxStoreRegionCodePoints, true)
	if err != nil {
		return nil, err
	}
	externalStoreID, err := validateNormalizedUserValue("external store ID", snapshot.ExternalStoreID, MaxExternalStoreIDCodePoints, false)
	if err != nil {
		return nil, err
	}
	connectionRef, err := validateOpaqueOptionalValue("connection reference", snapshot.ConnectionRef)
	if err != nil {
		return nil, err
	}
	quotaAllocationID, err := canonicalUUID(snapshot.QuotaAllocationID)
	if err != nil {
		return nil, fmt.Errorf("quota allocation ID: %w", err)
	}
	createIdempotencyKey, err := canonicalUUID(snapshot.CreateIdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("create idempotency key: %w", err)
	}
	createdBy, err := validateOpaqueIdentity("created by", snapshot.CreatedBy, MaxSubjectBytes)
	if err != nil {
		return nil, err
	}
	updatedBy, err := validateOpaqueIdentity("updated by", snapshot.UpdatedBy, MaxSubjectBytes)
	if err != nil {
		return nil, err
	}
	if !validLifecycleStatus(snapshot.LifecycleStatus) {
		return nil, errors.New("lifecycle status is invalid")
	}
	deleteOperationKey := ""
	if snapshot.LifecycleStatus == StoreStatusDeleting {
		deleteOperationKey, err = canonicalUUID(snapshot.DeleteOperationKey)
		if err != nil {
			return nil, fmt.Errorf("delete operation key: %w", err)
		}
	} else if snapshot.DeleteOperationKey != "" {
		return nil, errors.New("only deleting stores may have a delete operation key")
	}
	if snapshot.Version < minimumLifecycleVersion(snapshot.LifecycleStatus) {
		return nil, fmt.Errorf("version %d cannot reach lifecycle status %s", snapshot.Version, snapshot.LifecycleStatus)
	}
	if snapshot.CreatedAt.IsZero() || snapshot.UpdatedAt.IsZero() {
		return nil, errors.New("created and updated times are required")
	}
	if snapshot.UpdatedAt.Before(snapshot.CreatedAt) {
		return nil, errors.New("updated time must not precede created time")
	}
	if snapshot.DeletedAt != nil {
		if snapshot.DeletedAt.IsZero() || snapshot.DeletedAt.Before(snapshot.UpdatedAt) {
			return nil, errors.New("deleted time is invalid")
		}
		if snapshot.LifecycleStatus != StoreStatusDeleting {
			return nil, errors.New("only deleting stores may have a deleted time")
		}
	}

	return &Store{
		id:                   id,
		organizationID:       organizationID,
		name:                 name,
		platform:             platform,
		region:               region,
		externalStoreID:      externalStoreID,
		lifecycleStatus:      snapshot.LifecycleStatus,
		connectionRef:        connectionRef,
		quotaAllocationID:    quotaAllocationID,
		version:              snapshot.Version,
		createdBy:            createdBy,
		updatedBy:            updatedBy,
		createdAt:            snapshot.CreatedAt,
		updatedAt:            snapshot.UpdatedAt,
		deletedAt:            copyTimePointer(snapshot.DeletedAt),
		createIdempotencyKey: createIdempotencyKey,
		deleteOperationKey:   deleteOperationKey,
	}, nil
}

func canTransition(current, target LifecycleStatus) bool {
	switch current {
	case StoreStatusProvisioning:
		return target == StoreStatusActive
	case StoreStatusActive:
		return target == StoreStatusDisabled
	case StoreStatusDisabled:
		return target == StoreStatusActive
	default:
		return false
	}
}

func validLifecycleStatus(status LifecycleStatus) bool {
	switch status {
	case StoreStatusProvisioning, StoreStatusActive, StoreStatusDisabled, StoreStatusDeleting:
		return true
	default:
		return false
	}
}

func minimumLifecycleVersion(status LifecycleStatus) int64 {
	switch status {
	case StoreStatusProvisioning:
		return 1
	case StoreStatusActive:
		return 2
	case StoreStatusDisabled, StoreStatusDeleting:
		return 3
	default:
		return 1
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

func validateOpaqueOptionalValue(field, value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("%s must be valid UTF-8", field)
	}
	if containsControlCharacter(value) {
		return "", fmt.Errorf("%s contains a control character", field)
	}
	return value, nil
}

func validateNormalizedUserValue(field, value string, maxCodePoints int, required bool) (string, error) {
	normalized, err := normalizeUserValue(field, value, maxCodePoints, required)
	if err != nil {
		return "", err
	}
	if normalized != value {
		return "", fmt.Errorf("%s must already be normalized", field)
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

func copyTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
