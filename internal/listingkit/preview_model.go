package listingkit

import (
	"time"

	"task-processor/internal/amazonlisting"
	sheinproduct "task-processor/internal/shein/api/product"
)

type ListingKitPreview struct {
	TaskID              string                         `json:"task_id"`
	Status              TaskStatus                     `json:"status"`
	SelectedPlatform    string                         `json:"selected_platform,omitempty"`
	Platforms           []string                       `json:"platforms,omitempty"`
	NeedsReview         bool                           `json:"needs_review"`
	ApplyResult         *RevisionApplyResult           `json:"apply_result,omitempty"`
	AppliedChanges      *RevisionDiffPreview           `json:"applied_changes,omitempty"`
	RestoreResult       *RevisionRestoreResult         `json:"restore_result,omitempty"`
	RevisionHistoryMeta *ListingKitRevisionHistoryMeta `json:"revision_history_meta,omitempty"`
	RevisionHistory     []ListingKitRevisionRecord     `json:"revision_history,omitempty"`
	CreatedAt           time.Time                      `json:"created_at"`
	CompletedAt         *time.Time                     `json:"completed_at,omitempty"`
	Overview            *ListingKitPreviewHeader       `json:"overview,omitempty"`
	Amazon              *AmazonPreviewPayload          `json:"amazon,omitempty"`
	Shein               *SheinPreviewPayload           `json:"shein,omitempty"`
	Temu                *TemuPreviewPayload            `json:"temu,omitempty"`
	Walmart             *WalmartPreviewPayload         `json:"walmart,omitempty"`
}

type ListingKitRevisionHistoryMeta struct {
	TotalRecords    int  `json:"total_records"`
	ReturnedRecords int  `json:"returned_records"`
	HasMore         bool `json:"has_more"`
	IsTruncated     bool `json:"is_truncated"`
	MaxRecords      int  `json:"max_records"`
}

type RevisionRestoreResult struct {
	Applied        bool                    `json:"applied"`
	SuccessPayload *RevisionSuccessPayload `json:"success_payload,omitempty"`
}

type RevisionStatusSummary struct {
	Status        string   `json:"status,omitempty"`
	Headline      string   `json:"headline,omitempty"`
	Subheadline   string   `json:"subheadline,omitempty"`
	NeedsReview   bool     `json:"needs_review"`
	BlockingCount int      `json:"blocking_count,omitempty"`
	WarningCount  int      `json:"warning_count,omitempty"`
	Highlights    []string `json:"highlights,omitempty"`
}

type RevisionResultMessages struct {
	Title            string   `json:"title,omitempty"`
	Description      string   `json:"description,omitempty"`
	SuccessLabel     string   `json:"success_label,omitempty"`
	WarningTitle     string   `json:"warning_title,omitempty"`
	WarningSummaries []string `json:"warning_summaries,omitempty"`
}

type RevisionRecommendedView struct {
	View   string `json:"view,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type RevisionFollowUpChecklist struct {
	Required    []SheinChecklistGroupItem `json:"required,omitempty"`
	Recommended []SheinChecklistGroupItem `json:"recommended,omitempty"`
}

type RevisionApplyResult struct {
	Applied        bool                    `json:"applied"`
	SuccessPayload *RevisionSuccessPayload `json:"success_payload,omitempty"`
}

type RevisionFollowUpOverview struct {
	Status           string   `json:"status,omitempty"`
	Headline         string   `json:"headline,omitempty"`
	Subheadline      string   `json:"subheadline,omitempty"`
	RequiredCount    int      `json:"required_count,omitempty"`
	RecommendedCount int      `json:"recommended_count,omitempty"`
	NextActions      []string `json:"next_actions,omitempty"`
}

type RevisionInteractionPresentation struct {
	Scene           string                      `json:"scene,omitempty"`
	NextActions     []string                    `json:"next_actions,omitempty"`
	Messages        *RevisionResultMessages     `json:"messages,omitempty"`
	RecommendedView *RevisionRecommendedView    `json:"recommended_view,omitempty"`
	SummaryCard     *RevisionSuccessSummaryCard `json:"summary_card,omitempty"`
}

type RevisionSuccessCoreData struct {
	ActionType                string                       `json:"action_type,omitempty"`
	Headline                  string                       `json:"headline,omitempty"`
	ChangeCount               int                          `json:"change_count,omitempty"`
	SourceRevisionID          string                       `json:"source_revision_id,omitempty"`
	RelationText              string                       `json:"relation_text,omitempty"`
	StatusSummary             *RevisionStatusSummary       `json:"status_summary,omitempty"`
	FollowUpChecklist         *RevisionFollowUpChecklist   `json:"follow_up_checklist,omitempty"`
	FollowUpOverview          *RevisionFollowUpOverview    `json:"follow_up_overview,omitempty"`
	SuggestedFollowUpRevision *SheinEditorRevisionSkeleton `json:"suggested_follow_up_revision,omitempty"`
	AppliedChanges            *RevisionDiffPreview         `json:"applied_changes,omitempty"`
}

type RevisionSuccessPayload struct {
	Mode         string                           `json:"mode,omitempty"`
	Core         *RevisionSuccessCoreData         `json:"core,omitempty"`
	Presentation *RevisionInteractionPresentation `json:"presentation,omitempty"`
}

type RevisionSuccessSummaryCard struct {
	Status        string   `json:"status,omitempty"`
	Title         string   `json:"title,omitempty"`
	Subtitle      string   `json:"subtitle,omitempty"`
	PrimaryAction string   `json:"primary_action,omitempty"`
	PrimaryView   string   `json:"primary_view,omitempty"`
	Highlights    []string `json:"highlights,omitempty"`
}

type ListingKitPreviewHeader struct {
	Country       string                   `json:"country,omitempty"`
	Language      string                   `json:"language,omitempty"`
	SourceType    string                   `json:"source_type,omitempty"`
	ImageCount    int                      `json:"image_count,omitempty"`
	VariantCount  int                      `json:"variant_count,omitempty"`
	StatusMessage string                   `json:"status_message,omitempty"`
	Warnings      []string                 `json:"warnings,omitempty"`
	PlatformCards []ListingKitPlatformCard `json:"platform_cards,omitempty"`
}

type ListingKitPlatformCard struct {
	Platform    string `json:"platform"`
	Status      string `json:"status"`
	Summary     string `json:"summary,omitempty"`
	NeedsReview bool   `json:"needs_review"`
}

type AmazonPreviewPayload struct {
	Title       string                            `json:"title,omitempty"`
	Brand       string                            `json:"brand,omitempty"`
	ProductType string                            `json:"product_type,omitempty"`
	Draft       *amazonlisting.AmazonListingDraft `json:"draft,omitempty"`
}

type SheinPreviewPayload struct {
	Headline          string                  `json:"headline,omitempty"`
	BrandName         string                  `json:"brand_name,omitempty"`
	CategoryPath      []string                `json:"category_path,omitempty"`
	CategoryID        int                     `json:"category_id,omitempty"`
	NeedsReview       bool                    `json:"needs_review"`
	Summary           []string                `json:"summary,omitempty"`
	ReviewNotes       []string                `json:"review_notes,omitempty"`
	Inspection        *SheinInspection        `json:"inspection,omitempty"`
	SubmitReadiness   *SheinSubmitReadiness   `json:"submit_readiness,omitempty"`
	SubmitChecklist   *SheinSubmitChecklist   `json:"submit_checklist,omitempty"`
	RepairCenter      *SheinRepairCenter      `json:"repair_center,omitempty"`
	StatusOverview    *SheinStatusOverview    `json:"status_overview,omitempty"`
	WorkspaceOverview *SheinWorkspaceOverview `json:"workspace_overview,omitempty"`
	EditorContext     *SheinEditorContext     `json:"editor_context,omitempty"`
	RequestDraft      *SheinRequestDraft      `json:"request_draft,omitempty"`
	PreviewProduct    *sheinproduct.Product   `json:"preview_product,omitempty"`
}

type TemuPreviewPayload struct {
	Headline    string       `json:"headline,omitempty"`
	NeedsReview bool         `json:"needs_review"`
	ReviewNotes []string     `json:"review_notes,omitempty"`
	Package     *TemuPackage `json:"package,omitempty"`
}

type WalmartPreviewPayload struct {
	Headline    string          `json:"headline,omitempty"`
	NeedsReview bool            `json:"needs_review"`
	ReviewNotes []string        `json:"review_notes,omitempty"`
	Package     *WalmartPackage `json:"package,omitempty"`
}
