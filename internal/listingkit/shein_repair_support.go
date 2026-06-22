package listingkit

import (
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
)

type SheinRepairValidationPreview = sheinworkspace.RepairValidationPreview[RevisionFieldError]
type SheinRepairPatchPayload = sheinworkspace.RepairPatchPayload
