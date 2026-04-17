package listingkit

import (
	"time"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/asset"
	"task-processor/internal/catalog"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
	sheinworkspace "task-processor/internal/workspace/shein"
)

type ListingKitPreview struct {
	TaskID              string                         `json:"task_id"`
	Status              TaskStatus                     `json:"status"`
	SelectedPlatform    string                         `json:"selected_platform,omitempty"`
	Platforms           []string                       `json:"platforms,omitempty"`
	NeedsReview         bool                           `json:"needs_review"`
	Catalog             *catalog.Product               `json:"catalog,omitempty"`
	Assets              *asset.Bundle                  `json:"assets,omitempty"`
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

type RevisionStatusSummary = sheinworkspace.SuccessStatusSummary
type RevisionResultMessages = sheinworkspace.SuccessMessages
type RevisionRecommendedView = sheinworkspace.SuccessRecommendedView
type RevisionFollowUpChecklist = sheinworkspace.SuccessFollowUpChecklist[SheinChecklistGroupItem]

type RevisionApplyResult struct {
	Applied        bool                    `json:"applied"`
	SuccessPayload *RevisionSuccessPayload `json:"success_payload,omitempty"`
}

type RevisionFollowUpOverview = sheinworkspace.SuccessFollowUpOverview
type RevisionInteractionPresentation = sheinworkspace.SuccessInteractionPresentation
type RevisionSuccessCoreData = sheinworkspace.SuccessCoreData[SheinChecklistGroupItem]
type RevisionSuccessPayload = sheinworkspace.SuccessPayload[SheinChecklistGroupItem]
type RevisionSuccessSummaryCard = sheinworkspace.SuccessSummaryCard

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
	Headline          string                            `json:"headline,omitempty"`
	BrandName         string                            `json:"brand_name,omitempty"`
	CategoryPath      []string                          `json:"category_path,omitempty"`
	CategoryID        int                               `json:"category_id,omitempty"`
	NeedsReview       bool                              `json:"needs_review"`
	Summary           []string                          `json:"summary,omitempty"`
	ReviewNotes       []string                          `json:"review_notes,omitempty"`
	Inspection        *sheinpub.Inspection              `json:"inspection,omitempty"`
	SubmitReadiness   *SheinSubmitReadiness             `json:"submit_readiness,omitempty"`
	SubmitChecklist   *SheinSubmitChecklist             `json:"submit_checklist,omitempty"`
	RepairCenter      *SheinRepairCenter                `json:"repair_center,omitempty"`
	StatusOverview    *sheinworkspace.StatusOverview    `json:"status_overview,omitempty"`
	WorkspaceOverview *sheinworkspace.WorkspaceOverview `json:"workspace_overview,omitempty"`
	EditorContext     *SheinEditorContext               `json:"editor_context,omitempty"`
	RequestDraft      *sheinpub.RequestDraft            `json:"request_draft,omitempty"`
	PreviewProduct    *sheinproduct.Product             `json:"preview_product,omitempty"`
	InspectionData    *sheinpub.Inspection              `json:"inspection_data,omitempty"`
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
