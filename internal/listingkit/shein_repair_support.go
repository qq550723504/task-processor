package listingkit

import (
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
)

type SheinRepairValidationPreview = sheinworkspace.RepairValidationPreview[RevisionFieldError]
type SheinRepairPatchPayload = sheinworkspace.RepairPatchPayload

type sheinRepairRevisionBundle = sheinworkspace.RepairRevisionBundle[SheinRevisionInput, SheinEditorRevisionSkeleton, ApplyRevisionRequest]

type sheinRepairArtifacts = sheinworkspace.RepairArtifacts[SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview]
