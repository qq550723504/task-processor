package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

type ValidationPayload[RestorePreview any] = sheinmarketplace.ValidationPayload[RestorePreview]
type RepairValidationPreview[FieldError any] = sheinmarketplace.RepairValidationPreview[FieldError]

func BuildValidationPayload[RestorePreview any](pkg *sheinpub.Package, restorePreview *RestorePreview) *ValidationPayload[RestorePreview] {
	return sheinmarketplace.BuildValidationPayload(pkg, restorePreview)
}

func BuildRepairValidationPreview[FieldError any](
	pkg *sheinpub.Package,
	editorSection string,
	skeleton *EditorRevisionSkeleton,
	valid bool,
	fieldErrors []FieldError,
) *RepairValidationPreview[FieldError] {
	return sheinmarketplace.BuildRepairValidationPreview(pkg, editorSection, skeleton, valid, fieldErrors)
}
