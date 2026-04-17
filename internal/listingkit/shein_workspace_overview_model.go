package listingkit

type SheinWorkspaceOverview struct {
	Status           string                      `json:"status,omitempty"`
	Headline         string                      `json:"headline,omitempty"`
	Subheadline      string                      `json:"subheadline,omitempty"`
	PrimaryAction    string                      `json:"primary_action,omitempty"`
	PrimaryActionKey string                      `json:"primary_action_key,omitempty"`
	PrimaryView      string                      `json:"primary_view,omitempty"`
	NeedsReview      bool                        `json:"needs_review"`
	BlockingCount    int                         `json:"blocking_count,omitempty"`
	WarningCount     int                         `json:"warning_count,omitempty"`
	Highlights       []string                    `json:"highlights,omitempty"`
	NextActions      []string                    `json:"next_actions,omitempty"`
	ActiveSession    *SheinWorkspaceSessionEntry `json:"active_session,omitempty"`
	SubmitState      *SheinWorkspaceSubmitState  `json:"submit_state,omitempty"`
	RepairState      *SheinWorkspaceRepairState  `json:"repair_state,omitempty"`
}

type SheinWorkspaceSessionEntry struct {
	Status        string   `json:"status,omitempty"`
	CurrentStepID string   `json:"current_step_id,omitempty"`
	NextStepID    string   `json:"next_step_id,omitempty"`
	ResumeMode    string   `json:"resume_mode,omitempty"`
	RefreshBlocks []string `json:"refresh_blocks,omitempty"`
}

type SheinWorkspaceSubmitState struct {
	Status        string   `json:"status,omitempty"`
	Ready         bool     `json:"ready"`
	BlockingCount int      `json:"blocking_count,omitempty"`
	WarningCount  int      `json:"warning_count,omitempty"`
	Summary       []string `json:"summary,omitempty"`
}

type SheinWorkspaceRepairState struct {
	Status             string `json:"status,omitempty"`
	TotalActions       int    `json:"total_actions,omitempty"`
	DirectApplyActions int    `json:"direct_apply_actions,omitempty"`
	PrimaryPlanStatus  string `json:"primary_plan_status,omitempty"`
	SessionStatus      string `json:"session_status,omitempty"`
}
