package workspace

type SuccessStatusSummary struct {
	Status        string   `json:"status,omitempty"`
	Headline      string   `json:"headline,omitempty"`
	Subheadline   string   `json:"subheadline,omitempty"`
	NeedsReview   bool     `json:"needs_review"`
	BlockingCount int      `json:"blocking_count,omitempty"`
	WarningCount  int      `json:"warning_count,omitempty"`
	Highlights    []string `json:"highlights,omitempty"`
}

type SuccessMessages struct {
	Title            string   `json:"title,omitempty"`
	Description      string   `json:"description,omitempty"`
	SuccessLabel     string   `json:"success_label,omitempty"`
	WarningTitle     string   `json:"warning_title,omitempty"`
	WarningSummaries []string `json:"warning_summaries,omitempty"`
}

type SuccessRecommendedView struct {
	View   string `json:"view,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type SuccessFollowUpChecklist[Item any] struct {
	Required    []Item `json:"required,omitempty"`
	Recommended []Item `json:"recommended,omitempty"`
}

type SuccessFollowUpOverview struct {
	Status           string   `json:"status,omitempty"`
	Headline         string   `json:"headline,omitempty"`
	Subheadline      string   `json:"subheadline,omitempty"`
	RequiredCount    int      `json:"required_count,omitempty"`
	RecommendedCount int      `json:"recommended_count,omitempty"`
	NextActions      []string `json:"next_actions,omitempty"`
}

type SuccessSummaryCard struct {
	Status        string   `json:"status,omitempty"`
	Title         string   `json:"title,omitempty"`
	Subtitle      string   `json:"subtitle,omitempty"`
	PrimaryAction string   `json:"primary_action,omitempty"`
	PrimaryView   string   `json:"primary_view,omitempty"`
	Highlights    []string `json:"highlights,omitempty"`
}

type SuccessInteractionPresentation struct {
	Scene           string                  `json:"scene,omitempty"`
	NextActions     []string                `json:"next_actions,omitempty"`
	Messages        *SuccessMessages        `json:"messages,omitempty"`
	RecommendedView *SuccessRecommendedView `json:"recommended_view,omitempty"`
	SummaryCard     *SuccessSummaryCard     `json:"summary_card,omitempty"`
}

type SuccessCoreData[ChecklistItem any] struct {
	ActionType                string                                   `json:"action_type,omitempty"`
	Headline                  string                                   `json:"headline,omitempty"`
	ChangeCount               int                                      `json:"change_count,omitempty"`
	SourceRevisionID          string                                   `json:"source_revision_id,omitempty"`
	RelationText              string                                   `json:"relation_text,omitempty"`
	StatusSummary             *SuccessStatusSummary                    `json:"status_summary,omitempty"`
	FollowUpChecklist         *SuccessFollowUpChecklist[ChecklistItem] `json:"follow_up_checklist,omitempty"`
	FollowUpOverview          *SuccessFollowUpOverview                 `json:"follow_up_overview,omitempty"`
	SuggestedFollowUpRevision *EditorRevisionSkeleton                  `json:"suggested_follow_up_revision,omitempty"`
	AppliedChanges            *RevisionDiffPreview                     `json:"applied_changes,omitempty"`
}

type SuccessPayload[ChecklistItem any] struct {
	Mode         string                          `json:"mode,omitempty"`
	Core         *SuccessCoreData[ChecklistItem] `json:"core,omitempty"`
	Presentation *SuccessInteractionPresentation `json:"presentation,omitempty"`
}
