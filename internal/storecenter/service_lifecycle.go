package storecenter

import (
	"errors"
	"time"
)

const defaultStoreServicePeriod = 30 * 24 * time.Hour

var (
	ErrConnectionNotFresh      = errors.New("store connection is not fresh")
	ErrConnectionUnavailable   = errors.New("store connection status is unavailable")
	ErrServiceAlreadyActive    = errors.New("store service is already active")
	ErrServiceExpired          = errors.New("store service is expired")
	ErrServiceNotExpired       = errors.New("store service is not expired")
	ErrServiceSuspended        = errors.New("store service is suspended")
	ErrServiceQuantityInvalid  = errors.New("store service quantity is invalid")
	ErrServiceQuantityExceeded = errors.New("store service quantity exceeds maximum")
)

// ActivateStoreService applies the pure first-activation transition. The
// caller must persist the returned state together with the resource consume in
// one transaction; this function intentionally has no persistence effects.
func ActivateStoreService(state StoreServiceState, connection ConnectionStatus, now time.Time) (StoreServiceState, error) {
	if err := validateServiceTransitionState(state); err != nil {
		return StoreServiceState{}, err
	}
	if state.RecordStatus != RecordStatusActive {
		return StoreServiceState{}, ErrInvalidServiceTransition
	}
	switch state.ServiceStatus {
	case ServiceStatusActive:
		return StoreServiceState{}, ErrServiceAlreadyActive
	case ServiceStatusExpired:
		return StoreServiceState{}, ErrServiceExpired
	case ServiceStatusSuspended:
		return StoreServiceState{}, ErrServiceSuspended
	case ServiceStatusPendingActivation:
		// Continue below.
	default:
		return StoreServiceState{}, ErrInvalidServiceTransition
	}
	if connection != ConnectionStatusConnected {
		return StoreServiceState{}, ErrConnectionNotFresh
	}
	if now.IsZero() {
		return StoreServiceState{}, ErrInvalidServiceTransition
	}
	startedAt := now.UTC()
	expiresAt := startedAt.Add(defaultStoreServicePeriod)
	return StoreServiceState{
		RecordStatus:  RecordStatusActive,
		ServiceStatus: ServiceStatusActive,
		StartedAt:     &startedAt,
		ExpiresAt:     &expiresAt,
	}, nil
}

// RenewStoreService extends an effectively active period. Quantity limits are
// supplied by the server-side policy rather than by an HTTP caller.
func RenewStoreService(state StoreServiceState, quantity, maxQuantity int64, now time.Time) (StoreServiceState, error) {
	if err := validateServiceTransitionState(state); err != nil {
		return StoreServiceState{}, err
	}
	if state.RecordStatus != RecordStatusActive {
		return StoreServiceState{}, ErrInvalidServiceTransition
	}
	if err := validateServiceQuantity(quantity, maxQuantity); err != nil {
		return StoreServiceState{}, err
	}
	if now.IsZero() {
		return StoreServiceState{}, ErrInvalidServiceTransition
	}
	switch effectiveServiceStatus(state, now.UTC()) {
	case ServiceStatusExpired:
		return StoreServiceState{}, ErrServiceExpired
	case ServiceStatusSuspended:
		return StoreServiceState{}, ErrServiceSuspended
	case ServiceStatusPendingActivation:
		return StoreServiceState{}, ErrInvalidServiceTransition
	case ServiceStatusActive:
		// Continue below.
	default:
		return StoreServiceState{}, ErrInvalidServiceTransition
	}

	expiresAt := *state.ExpiresAt
	extension := time.Duration(quantity) * defaultStoreServicePeriod
	expiresAt = expiresAt.Add(extension)
	result := copyServiceState(state)
	result.ServiceStatus = ServiceStatusActive
	result.ExpiresAt = &expiresAt
	return result, nil
}

// ReactivateStoreService starts a new period for an effectively expired
// Store. A suspended Store is deliberately not treated as expired.
func ReactivateStoreService(state StoreServiceState, quantity, maxQuantity int64, now time.Time) (StoreServiceState, error) {
	if err := validateServiceTransitionState(state); err != nil {
		return StoreServiceState{}, err
	}
	if state.RecordStatus != RecordStatusActive {
		return StoreServiceState{}, ErrInvalidServiceTransition
	}
	if err := validateServiceQuantity(quantity, maxQuantity); err != nil {
		return StoreServiceState{}, err
	}
	if now.IsZero() {
		return StoreServiceState{}, ErrInvalidServiceTransition
	}
	if effectiveServiceStatus(state, now.UTC()) != ServiceStatusExpired {
		return StoreServiceState{}, ErrServiceNotExpired
	}
	startedAt := now.UTC()
	expiresAt := startedAt.Add(time.Duration(quantity) * defaultStoreServicePeriod)
	return StoreServiceState{
		RecordStatus:  RecordStatusActive,
		ServiceStatus: ServiceStatusActive,
		StartedAt:     &startedAt,
		ExpiresAt:     &expiresAt,
	}, nil
}

func validateServiceTransitionState(state StoreServiceState) error {
	if err := ValidateStoreServiceState(state); err != nil {
		return err
	}
	return nil
}

func validateServiceQuantity(quantity, maxQuantity int64) error {
	if quantity <= 0 || maxQuantity <= 0 {
		return ErrServiceQuantityInvalid
	}
	if quantity > maxQuantity {
		return ErrServiceQuantityExceeded
	}
	return nil
}

func effectiveServiceStatus(state StoreServiceState, now time.Time) ServiceStatus {
	if state.ServiceStatus == ServiceStatusActive && state.ExpiresAt != nil && !now.Before(*state.ExpiresAt) {
		return ServiceStatusExpired
	}
	return state.ServiceStatus
}
