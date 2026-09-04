package storecenter

import (
	"errors"
	"time"
)

// RecordStatus is the durable Store record lifecycle introduced by the V7
// expand phase. It is intentionally separate from the legacy lifecycle_status
// compatibility column.
type RecordStatus string

const (
	RecordStatusProvisioning RecordStatus = "provisioning"
	RecordStatusActive       RecordStatus = "active"
	RecordStatusDeleting     RecordStatus = "deleting"
	RecordStatusDeleted      RecordStatus = "deleted"
)

type ServiceStatus string

const (
	ServiceStatusPendingActivation ServiceStatus = "pending_activation"
	ServiceStatusActive            ServiceStatus = "active"
	ServiceStatusExpired           ServiceStatus = "expired"
	ServiceStatusSuspended         ServiceStatus = "suspended"
)

var (
	ErrInvalidServiceState       = errors.New("invalid store record/service state")
	ErrInvalidServiceTransition  = errors.New("invalid store service transition")
	ErrConnectionSnapshotChanged = errors.New("store connection snapshot changed")
)

// StoreServiceState is a pure validation boundary for the expanded state.
// History resolution evidence is deliberately not inferred here; a suspended
// state may have no timestamps until the authoritative history resolver has
// produced a durable result.
type StoreServiceState struct {
	RecordStatus  RecordStatus
	ServiceStatus ServiceStatus
	StartedAt     *time.Time
	ExpiresAt     *time.Time
}

// ServiceStoreIdentity and the transaction DTOs are the narrow persistence
// boundary used by the cross-aggregate Store+Resource unit of work. They do
// not expose GORM records or permit changing Store identity/profile fields.
type ServiceStoreIdentity struct {
	OrganizationID string
	StoreID        string
}

type ServiceStoreSnapshot struct {
	Identity          ServiceStoreIdentity
	QuotaAllocationID string
	ConnectionRef     string
	Version           int64
	UpdatedAt         time.Time
	State             StoreServiceState
}

type ServiceStoreMutation struct {
	Identity              ServiceStoreIdentity
	ExpectedVersion       int64
	ExpectedConnectionRef string
	State                 StoreServiceState
	ActorSubject          string
	OccurredAt            time.Time
}

func ValidateStoreServiceState(state StoreServiceState) error {
	switch state.RecordStatus {
	case RecordStatusProvisioning, RecordStatusDeleting, RecordStatusDeleted:
		if state.ServiceStatus != "" || state.StartedAt != nil || state.ExpiresAt != nil {
			return ErrInvalidServiceState
		}
		return nil
	case RecordStatusActive:
		// Continue with service-state validation below.
	default:
		return ErrInvalidServiceState
	}

	switch state.ServiceStatus {
	case ServiceStatusPendingActivation:
		if state.StartedAt != nil || state.ExpiresAt != nil {
			return ErrInvalidServiceState
		}
	case ServiceStatusActive, ServiceStatusExpired:
		if !validServicePeriod(state.StartedAt, state.ExpiresAt) {
			return ErrInvalidServiceState
		}
	case ServiceStatusSuspended:
		if (state.StartedAt == nil) != (state.ExpiresAt == nil) {
			return ErrInvalidServiceState
		}
		if state.StartedAt != nil && !validServicePeriod(state.StartedAt, state.ExpiresAt) {
			return ErrInvalidServiceState
		}
	default:
		return ErrInvalidServiceState
	}
	return nil
}

func validServicePeriod(startedAt, expiresAt *time.Time) bool {
	return startedAt != nil && expiresAt != nil && !startedAt.IsZero() && !expiresAt.IsZero() && expiresAt.After(*startedAt)
}

func copyServiceState(state StoreServiceState) StoreServiceState {
	state.StartedAt = copyTimePointer(state.StartedAt)
	state.ExpiresAt = copyTimePointer(state.ExpiresAt)
	return state
}
