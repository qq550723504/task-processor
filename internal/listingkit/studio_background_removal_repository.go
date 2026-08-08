package listingkit

import "context"

// studioBackgroundRemovalRepository narrows background-removal writes to the
// fields owned by the removal operation. This keeps concurrent review updates
// out of the provider completion path.
type studioBackgroundRemovalRepository interface {
	ClaimStudioMaterializedDesignBackgroundRemoval(
		ctx context.Context,
		design *StudioMaterializedDesignRecord,
	) (bool, error)
	UpdateStudioMaterializedDesignBackgroundRemoval(
		ctx context.Context,
		design *StudioMaterializedDesignRecord,
	) error
}

type manualBackgroundRemovalApplier interface {
	ApplyManualStudioMaterializedDesignBackgroundRemoval(
		ctx context.Context,
		design *StudioMaterializedDesignRecord,
	) (bool, error)
}
