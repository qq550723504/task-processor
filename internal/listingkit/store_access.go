package listingkit

import (
	"context"
	"errors"
)

const (
	// StoreAccessUnavailable does not disclose whether a requested store exists.
	StoreAccessUnavailable = "listingkit_store_unavailable"
	// StoreAccessDisabled identifies a same-tenant store that must not be used.
	StoreAccessDisabled = "listingkit_store_disabled"
	// StoreAccessStale identifies a persisted store snapshot that must be selected again.
	StoreAccessStale = "listingkit_store_snapshot_stale"
)

// StoreAccess is the sanitized ownership state for one validated store.
type StoreAccess struct {
	ID       int64
	TenantID int64
	Platform string
	Enabled  bool
}

// StoreAccessValidator validates a tenant's access to one platform store.
type StoreAccessValidator interface {
	ValidateStoreAccess(context.Context, int64, int64, string) (StoreAccess, error)
}

// StoreAccessError represents a stable customer-visible store access failure.
type StoreAccessError struct {
	code    string
	message string
}

func (e *StoreAccessError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

// NewStoreAccessError creates a stable store-access error without store details.
func NewStoreAccessError(code, message string) error {
	return &StoreAccessError{code: code, message: message}
}

// StoreAccessErrorCode returns the stable code for a store-access error.
func StoreAccessErrorCode(err error) string {
	var accessErr *StoreAccessError
	if !errors.As(err, &accessErr) || accessErr == nil {
		return ""
	}
	return accessErr.code
}
