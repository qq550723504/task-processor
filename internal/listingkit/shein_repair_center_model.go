package listingkit

type SheinRepairCenter struct {
	Status        string                     `json:"status,omitempty"`
	Summary       []string                   `json:"summary,omitempty"`
	Stats         *SheinRepairCenterStats    `json:"stats,omitempty"`
	PrimaryAction *SheinRepairCenterAction   `json:"primary_action,omitempty"`
	PrimaryPlan   *SheinRepairPlan           `json:"primary_plan,omitempty"`
	ApplyQueue    *SheinRepairApplyQueue     `json:"apply_queue,omitempty"`
	Session       *SheinRepairSession        `json:"session,omitempty"`
	Sections      []SheinRepairCenterSection `json:"sections,omitempty"`
	Actions       []SheinRepairCenterAction  `json:"actions,omitempty"`
}

type SheinRepairCenterStats struct {
	TotalActions       int `json:"total_actions,omitempty"`
	BlockingActions    int `json:"blocking_actions,omitempty"`
	WarningActions     int `json:"warning_actions,omitempty"`
	DirectApplyActions int `json:"direct_apply_actions,omitempty"`
}

type SheinRepairCenterSection struct {
	Key              string   `json:"key,omitempty"`
	Label            string   `json:"label,omitempty"`
	ActionCount      int      `json:"action_count,omitempty"`
	DirectApplyCount int      `json:"direct_apply_count,omitempty"`
	Highlights       []string `json:"highlights,omitempty"`
}

type SheinRepairCenterAction struct {
	ID               string                        `json:"id,omitempty"`
	Key              string                        `json:"key,omitempty"`
	Label            string                        `json:"label,omitempty"`
	Status           string                        `json:"status,omitempty"`
	Priority         string                        `json:"priority,omitempty"`
	EditorSection    string                        `json:"editor_section,omitempty"`
	Target           string                        `json:"target,omitempty"`
	Description      string                        `json:"description,omitempty"`
	SuggestedAction  string                        `json:"suggested_action,omitempty"`
	CanApplyDirectly bool                          `json:"can_apply_directly"`
	SourceGroups     []string                      `json:"source_groups,omitempty"`
	FieldPaths       []string                      `json:"field_paths,omitempty"`
	EditorFocus      []string                      `json:"editor_focus,omitempty"`
	RevisionPath     string                        `json:"revision_path,omitempty"`
	Reason           *SheinReadinessReason         `json:"reason,omitempty"`
	Patch            *SheinRepairPatchPayload      `json:"patch,omitempty"`
	Skeleton         *SheinEditorRevisionSkeleton  `json:"skeleton,omitempty"`
	Revision         *ApplyRevisionRequest         `json:"revision,omitempty"`
	Validation       *SheinRepairValidationPreview `json:"validation,omitempty"`
}
