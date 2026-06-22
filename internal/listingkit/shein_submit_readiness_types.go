package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

type sheinSubmitReadinessSummaryShape struct {
	blockingLabel       string
	warningLabel        string
	prependFirstBlocker bool
}

type SheinReadinessReason = sheinworkspace.ReadinessReason

type SheinRepairHint struct {
	Action        string                        `json:"action,omitempty"`
	Priority      string                        `json:"priority,omitempty"`
	Target        string                        `json:"target,omitempty"`
	EditorSection string                        `json:"editor_section,omitempty"`
	EditorFocus   []string                      `json:"editor_focus,omitempty"`
	RevisionPath  string                        `json:"revision_path,omitempty"`
	Description   string                        `json:"description,omitempty"`
	FieldPaths    []string                      `json:"field_paths,omitempty"`
	Patch         *SheinRepairPatchPayload      `json:"patch,omitempty"`
	Skeleton      *SheinEditorRevisionSkeleton  `json:"skeleton,omitempty"`
	Revision      *ApplyRevisionRequest         `json:"revision,omitempty"`
	Validation    *SheinRepairValidationPreview `json:"validation,omitempty"`
}

type sheinReadinessGuidance struct {
	reason      *SheinReadinessReason
	repairHints []SheinRepairHint
}
