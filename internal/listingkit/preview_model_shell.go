package listingkit

import (
	"time"

	previewdomain "task-processor/internal/listing/preview"
	"task-processor/internal/listingkit/core"
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

type ListingKitPreview struct {
	TaskID                 string                               `json:"task_id"`
	Status                 core.TaskStatus                      `json:"status"`
	SelectedPlatform       string                               `json:"selected_platform,omitempty"`
	Platforms              []string                             `json:"platforms,omitempty"`
	NeedsReview            bool                                 `json:"needs_review"`
	Catalog                *catalog.ProductSnapshot             `json:"catalog,omitempty"`
	ApprovedAssetInventory *productasset.ApprovedAssetInventory `json:"approved_asset_inventory,omitempty"`
	ApplyResult            *RevisionApplyResult                 `json:"apply_result,omitempty"`
	AppliedChanges         *RevisionDiffPreview                 `json:"applied_changes,omitempty"`
	RestoreResult          *RevisionRestoreResult               `json:"restore_result,omitempty"`
	RevisionHistoryMeta    *ListingKitRevisionHistoryMeta       `json:"revision_history_meta,omitempty"`
	RevisionHistory        []ListingKitRevisionRecord           `json:"revision_history,omitempty"`
	CreatedAt              time.Time                            `json:"created_at"`
	CompletedAt            *time.Time                           `json:"completed_at,omitempty"`
	Overview               *ListingKitPreviewHeader             `json:"overview,omitempty"`
	Amazon                 *AmazonPreviewPayload                `json:"amazon,omitempty"`
	Shein                  *SheinPreviewPayload                 `json:"shein,omitempty"`
	Temu                   *TemuPreviewPayload                  `json:"temu,omitempty"`
	Walmart                *WalmartPreviewPayload               `json:"walmart,omitempty"`
}

type ListingKitRevisionHistoryMeta = previewdomain.RevisionHistoryMeta

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
	ReviewReasons []string                 `json:"review_reasons,omitempty"`
	PlatformCards []ListingKitPlatformCard `json:"platform_cards,omitempty"`
}

type ListingKitPlatformCard struct {
	Platform                  string         `json:"platform"`
	Status                    string         `json:"status"`
	Summary                   string         `json:"summary,omitempty"`
	NeedsReview               bool           `json:"needs_review"`
	PreviewableItems          int            `json:"previewable_items"`
	PreviewCapabilityCounts   map[string]int `json:"preview_capability_counts,omitempty"`
	QualityGradeCounts        map[string]int `json:"quality_grade_counts,omitempty"`
	DominantQualityGrade      string         `json:"dominant_quality_grade,omitempty"`
	DominantQualityGradeLabel string         `json:"dominant_quality_grade_label,omitempty"`
	ApprovedSections          int            `json:"approved_sections"`
	DeferredSections          int            `json:"deferred_sections"`
	ReviewPendingSections     int            `json:"review_pending_sections"`
}
