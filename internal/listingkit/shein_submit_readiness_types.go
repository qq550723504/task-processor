package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

type sheinSubmitReadinessSummaryShape struct {
	blockingLabel       string
	warningLabel        string
	prependFirstBlocker bool
}

type SheinReadinessReason = sheinworkspace.ReadinessReason

type SheinRepairHint = sheinworkspace.RepairHint[SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview]

type sheinReadinessGuidance struct {
	reason      *SheinReadinessReason
	repairHints []SheinRepairHint
}
