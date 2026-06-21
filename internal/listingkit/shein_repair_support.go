package listingkit

import (
	listingworkspace "task-processor/internal/listingkit/workspace/shein"
)

type SheinRepairValidationPreview = listingworkspace.RepairValidationPreview[RevisionFieldError]
type SheinRepairPatchPayload = listingworkspace.RepairPatchPayload

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
