package listingkit

import (
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
)

type SheinRepairValidationPreview = sheinworkspace.RepairValidationPreview[RevisionFieldError]
type SheinRepairPatchPayload = sheinworkspace.RepairPatchPayload

type sheinRepairRevisionBundle struct {
	input    *SheinRevisionInput
	skeleton *SheinEditorRevisionSkeleton
	request  *ApplyRevisionRequest
}

type sheinRepairArtifacts struct {
	patch      *SheinRepairPatchPayload
	skeleton   *SheinEditorRevisionSkeleton
	request    *ApplyRevisionRequest
	validation *SheinRepairValidationPreview
}
