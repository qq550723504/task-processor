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
	ErrPageOffsetOverflow    = errors.New("store page offset overflows")
	ErrLimitReached          = errors.New("store limit reached")
	ErrDependencyUnavailable = errors.New("store dependency unavailable")
)

type Platform string

const PlatformShein Platform = "shein"

// Store is the Organization-scoped Store Center aggregate. Its state is kept
// private so identity and lifecycle changes can only occur through its rules.
type Store struct {
	id                   string
	organizationID       string
	name                 string
	platform             Platform
	region               string
	externalStoreID      string
	recordStatus         RecordStatus
	serviceStatus        ServiceStatus
	serviceStartedAt     *time.Time
	serviceExpiresAt     *time.Time
	connectionRef        string
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
	RecordStatus         RecordStatus
	ServiceStatus        ServiceStatus
	ServiceStartedAt     *time.Time
	ServiceExpiresAt     *time.Time
	ConnectionRef        string
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
		RecordStatus:         RecordStatusActive,
		ServiceStatus:        ServiceStatusPendingActivation,
		ConnectionRef:        "",
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

func (s *Store) ID() string                   { return s.id }
func (s *Store) OrganizationID() string       { return s.organizationID }
func (s *Store) Name() string                 { return s.name }
func (s *Store) Platform() Platform           { return s.platform }
func (s *Store) Region() string               { return s.region }
func (s *Store) ExternalStoreID() string      { return s.externalStoreID }
func (s *Store) RecordStatus() RecordStatus   { return s.recordStatus }
func (s *Store) ServiceStatus() ServiceStatus { return s.serviceStatus }
func (s *Store) ServiceStartedAt() *time.Time { return copyTimePointer(s.serviceStartedAt) }
func (s *Store) ServiceExpiresAt() *time.Time { return copyTimePointer(s.serviceExpiresAt) }
func (s *Store) ConnectionRef() string        { return s.connectionRef }
func (s *Store) Version() int64               { return s.version }
func (s *Store) CreatedBy() string            { return s.createdBy }
func (s *Store) UpdatedBy() string            { return s.updatedBy }
func (s *Store) CreatedAt() time.Time         { return s.createdAt }
func (s *Store) UpdatedAt() time.Time         { return s.updatedAt }
func (s *Store) CreateIdempotencyKey() string { return s.createIdempotencyKey }
func (s *Store) DeleteOperationKey() string   { return s.deleteOperationKey }
func (s *Store) DeletedAt() *time.Time        { return copyTimePointer(s.deletedAt) }

func (s *Store) Snapshot() StoreSnapshot {
	return StoreSnapshot{
		ID:                   s.id,
		OrganizationID:       s.organizationID,
		Name:                 s.name,
		Platform:             s.platform,
		Region:               s.region,
		ExternalStoreID:      s.externalStoreID,
		RecordStatus:         s.recordStatus,
		ServiceStatus:        s.serviceStatus,
		ServiceStartedAt:     copyTimePointer(s.serviceStartedAt),
		ServiceExpiresAt:     copyTimePointer(s.serviceExpiresAt),
		ConnectionRef:        s.connectionRef,
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
	if s.recordStatus != RecordStatusActive && s.recordStatus != RecordStatusDisabled {
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
	if s.recordStatus == RecordStatusDeleting {
		if s.deleteOperationKey == operationKey {
			return nil
		}
		return ErrInvalidTransition
	}
	if s.recordStatus != RecordStatusActive && s.recordStatus != RecordStatusDisabled {
		return ErrInvalidTransition
	}
	actorSubject, err = validateOpaqueIdentity("actor subject", actorSubject, MaxSubjectBytes)
	if err != nil {
		return err
	}
	if occurredAt.IsZero() || occurredAt.Before(s.updatedAt) {
		return errors.New("delete time must not precede the last update")
	}
	s.recordStatus = RecordStatusDeleting
	s.serviceStatus = ""
	s.serviceStartedAt = nil
	s.serviceExpiresAt = nil
	s.deleteOperationKey = operationKey
	s.updatedBy = actorSubject
	s.updatedAt = occurredAt
	s.version++
	return nil
}

func (s *Store) TransitionTo(target RecordStatus, actorSubject string, occurredAt time.Time) error {
	if !canTransition(s.recordStatus, target) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, s.recordStatus, target)
	}
	actorSubject, err := validateOpaqueIdentity("actor subject", actorSubject, MaxSubjectBytes)
	if err != nil {
		return err
	}
	if occurredAt.IsZero() || occurredAt.Before(s.updatedAt) {
		return errors.New("transition time must not precede the last update")
	}
	s.recordStatus = target
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
	if !validRecordStatus(snapshot.RecordStatus) {
		return nil, errors.New("record status is invalid")
	}
	if err := ValidateStoreServiceState(StoreServiceState{RecordStatus: snapshot.RecordStatus, ServiceStatus: snapshot.ServiceStatus, StartedAt: snapshot.ServiceStartedAt, ExpiresAt: snapshot.ServiceExpiresAt}); err != nil {
		return nil, err
	}
	deleteOperationKey := ""
	if snapshot.RecordStatus == RecordStatusDeleting || snapshot.RecordStatus == RecordStatusDeleted {
		deleteOperationKey, err = canonicalUUID(snapshot.DeleteOperationKey)
		if err != nil {
			return nil, fmt.Errorf("delete operation key: %w", err)
		}
	} else if snapshot.DeleteOperationKey != "" {
		return nil, errors.New("only deleting stores may have a delete operation key")
	}
	if snapshot.Version < minimumRecordVersion(snapshot.RecordStatus) {
		return nil, fmt.Errorf("version %d cannot reach record status %s", snapshot.Version, snapshot.RecordStatus)
	}
	if snapshot.CreatedAt.IsZero() || snapshot.UpdatedAt.IsZero() {
		return nil, errors.New("created and updated times are required")
	}
	if snapshot.UpdatedAt.Before(snapshot.CreatedAt) {
		return nil, errors.New("updated time must not precede created time")
	}
	if snapshot.RecordStatus == RecordStatusDeleted && snapshot.DeletedAt == nil {
		return nil, errors.New("deleted stores require a deleted time")
	}
	if snapshot.DeletedAt != nil {
		if snapshot.DeletedAt.IsZero() || snapshot.DeletedAt.Before(snapshot.UpdatedAt) {
			return nil, errors.New("deleted time is invalid")
		}
		if snapshot.RecordStatus != RecordStatusDeleted {
			return nil, errors.New("only deleted stores may have a deleted time")
		}
	}

	return &Store{
		id:                   id,
		organizationID:       organizationID,
		name:                 name,
		platform:             platform,
		region:               region,
		externalStoreID:      externalStoreID,
		recordStatus:         snapshot.RecordStatus,
		serviceStatus:        snapshot.ServiceStatus,
		serviceStartedAt:     copyTimePointer(snapshot.ServiceStartedAt),
		serviceExpiresAt:     copyTimePointer(snapshot.ServiceExpiresAt),
		connectionRef:        connectionRef,
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

func canTransition(current, target RecordStatus) bool {
	switch current {
	case RecordStatusActive:
		return target == RecordStatusDisabled
	case RecordStatusDisabled:
		return target == RecordStatusActive
	default:
		return false
	}
}

func validRecordStatus(status RecordStatus) bool {
	switch status {
	case RecordStatusActive, RecordStatusDisabled, RecordStatusDeleting, RecordStatusDeleted:
		return true
	default:
		return false
	}
}

func minimumRecordVersion(status RecordStatus) int64 {
	switch status {
	case RecordStatusActive:
		return 1
	case RecordStatusDisabled, RecordStatusDeleting:
		return 2
	case RecordStatusDeleted:
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
	if containsBrowserTrimDiscrepantCharacter(value) {
		return "", fmt.Errorf("%s contains a browser-trim discrepant character", field)
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
	if containsBrowserTrimDiscrepantCharacter(value) {
		return "", fmt.Errorf("%s contains a browser-trim discrepant character", field)
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

func containsBrowserTrimDiscrepantCharacter(value string) bool {
	return strings.ContainsRune(value, '\ufeff')
}

func copyTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
