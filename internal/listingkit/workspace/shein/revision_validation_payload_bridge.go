package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type ValidationPayload[RestorePreview any] = sheinmarketplace.ValidationPayload[RestorePreview]

func BuildValidationPayload[RestorePreview any](pkg *Package, restorePreview *RestorePreview) *ValidationPayload[RestorePreview] {
	return sheinmarketplace.BuildValidationPayload(pkg, restorePreview)
}
