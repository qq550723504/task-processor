package storecenter

import "context"

// SourcingStoreReader is the existing Organization-scoped Store read contract.
type SourcingStoreReader interface {
	Get(context.Context, string, string) (*Store, error)
}

// ValidateSourcingStoreAccess checks the target store for a source import.
// Authentication and effective-Organization selection belong to the caller.
// This does not reserve quota, authorize publication or consult legacy stores.
func ValidateSourcingStoreAccess(ctx context.Context, reader SourcingStoreReader, organizationID, storeID string, platform Platform) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if isNilDependency(reader) {
		return ErrDependencyUnavailable
	}
	if _, err := validateOpaqueIdentity("organization ID", organizationID, MaxOrganizationIDBytes); err != nil {
		return ErrNotFound
	}
	if _, err := canonicalUUID(storeID); err != nil {
		return ErrNotFound
	}
	if platform != PlatformShein {
		return ErrNotFound
	}
	store, err := reader.Get(ctx, organizationID, storeID)
	if err != nil {
		return err
	}
	if store == nil || store.OrganizationID() != organizationID || store.ID() != storeID || store.Platform() != platform || store.LifecycleStatus() != StoreStatusActive || store.Snapshot().DeletedAt != nil {
		return ErrNotFound
	}
	return ctx.Err()
}
