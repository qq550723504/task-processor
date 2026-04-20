package listingkit

import (
	"time"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	"task-processor/internal/catalog"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
	sheinworkspace "task-processor/internal/workspace/shein"
)

type ListingKitPreview struct {
	TaskID                      string                         `json:"task_id"`
	Status                      TaskStatus                     `json:"status"`
	SelectedPlatform            string                         `json:"selected_platform,omitempty"`
	Platforms                   []string                       `json:"platforms,omitempty"`
	NeedsReview                 bool                           `json:"needs_review"`
	Catalog                     *catalog.Product               `json:"catalog,omitempty"`
	Assets                      *asset.Bundle                  `json:"assets,omitempty"`
	AssetInventory              *asset.InventorySummary        `json:"asset_inventory,omitempty"`
	AssetRenderPreviews         []AssetRenderPreview           `json:"asset_render_previews,omitempty"`
	PlatformAssetRenderPreviews []PlatformAssetRenderPreviews  `json:"platform_asset_render_previews,omitempty"`
	AssetGenerationSummary      *AssetGenerationSummary        `json:"asset_generation_summary,omitempty"`
	AssetGenerationTasks        []assetgeneration.Task         `json:"asset_generation_tasks,omitempty"`
	AssetGenerationQueue        *GenerationWorkQueue           `json:"asset_generation_queue,omitempty"`
	AssetGenerationOverview     *AssetGenerationOverview       `json:"asset_generation_overview,omitempty"`
	ApplyResult                 *RevisionApplyResult           `json:"apply_result,omitempty"`
	AppliedChanges              *RevisionDiffPreview           `json:"applied_changes,omitempty"`
	RestoreResult               *RevisionRestoreResult         `json:"restore_result,omitempty"`
	RevisionHistoryMeta         *ListingKitRevisionHistoryMeta `json:"revision_history_meta,omitempty"`
	RevisionHistory             []ListingKitRevisionRecord     `json:"revision_history,omitempty"`
	CreatedAt                   time.Time                      `json:"created_at"`
	CompletedAt                 *time.Time                     `json:"completed_at,omitempty"`
	Overview                    *ListingKitPreviewHeader       `json:"overview,omitempty"`
	Amazon                      *AmazonPreviewPayload          `json:"amazon,omitempty"`
	Shein                       *SheinPreviewPayload           `json:"shein,omitempty"`
	Temu                        *TemuPreviewPayload            `json:"temu,omitempty"`
	Walmart                     *WalmartPreviewPayload         `json:"walmart,omitempty"`
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
	Platform                  string                             `json:"platform"`
	Status                    string                             `json:"status"`
	Summary                   string                             `json:"summary,omitempty"`
	NeedsReview               bool                               `json:"needs_review"`
	PreviewableItems          int                                `json:"previewable_items"`
	PreviewCapabilityCounts   map[string]int                     `json:"preview_capability_counts,omitempty"`
	QualityGradeCounts        map[string]int                     `json:"quality_grade_counts,omitempty"`
	DominantQualityGrade      string                             `json:"dominant_quality_grade,omitempty"`
	DominantQualityGradeLabel string                             `json:"dominant_quality_grade_label,omitempty"`
	PreviewSummary            *PlatformAssetRenderPreviewSummary `json:"preview_summary,omitempty"`
	ApprovedSections          int                                `json:"approved_sections"`
	DeferredSections          int                                `json:"deferred_sections"`
	ReviewPendingSections     int                                `json:"review_pending_sections"`
	PrimaryActionKey          string                             `json:"primary_action_key,omitempty"`
	PrimaryActionTarget       *AssetGenerationActionTarget       `json:"primary_action_target,omitempty"`
	PrimaryCTAKind            string                             `json:"primary_cta_kind,omitempty"`
	PrimaryNavigationTarget   *GenerationReviewNavigationTarget  `json:"primary_navigation_target,omitempty"`
	ResolvedActionSummary     *GenerationResolvedActionSummary   `json:"resolved_action_summary,omitempty"`
	ReviewTarget              *GenerationReviewTarget            `json:"review_target,omitempty"`
	RecoverySummary           *GenerationRecoverySummary         `json:"recovery_summary,omitempty"`
}

type AmazonPreviewPayload struct {
	Title          string                            `json:"title,omitempty"`
	Brand          string                            `json:"brand,omitempty"`
	ProductType    string                            `json:"product_type,omitempty"`
	ImageBundle    *common.PublishImageBundle        `json:"image_bundle,omitempty"`
	RenderPreviews *PlatformAssetRenderPreviews      `json:"render_previews,omitempty"`
	ScenePresets   []PlatformScenePresetSummary      `json:"scene_presets,omitempty"`
	Draft          *amazonlisting.AmazonListingDraft `json:"draft,omitempty"`
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
	ImageBundle       *common.PublishImageBundle        `json:"image_bundle,omitempty"`
	RenderPreviews    *PlatformAssetRenderPreviews      `json:"render_previews,omitempty"`
	ScenePresets      []PlatformScenePresetSummary      `json:"scene_presets,omitempty"`
	RequestDraft      *sheinpub.RequestDraft            `json:"request_draft,omitempty"`
	PreviewProduct    *sheinproduct.Product             `json:"preview_product,omitempty"`
	InspectionData    *sheinpub.Inspection              `json:"inspection_data,omitempty"`
}

type TemuPreviewPayload struct {
	Headline       string                       `json:"headline,omitempty"`
	NeedsReview    bool                         `json:"needs_review"`
	ReviewNotes    []string                     `json:"review_notes,omitempty"`
	ImageBundle    *common.PublishImageBundle   `json:"image_bundle,omitempty"`
	RenderPreviews *PlatformAssetRenderPreviews `json:"render_previews,omitempty"`
	ScenePresets   []PlatformScenePresetSummary `json:"scene_presets,omitempty"`
	Package        *TemuPackage                 `json:"package,omitempty"`
}

type WalmartPreviewPayload struct {
	Headline       string                       `json:"headline,omitempty"`
	NeedsReview    bool                         `json:"needs_review"`
	ReviewNotes    []string                     `json:"review_notes,omitempty"`
	ImageBundle    *common.PublishImageBundle   `json:"image_bundle,omitempty"`
	RenderPreviews *PlatformAssetRenderPreviews `json:"render_previews,omitempty"`
	ScenePresets   []PlatformScenePresetSummary `json:"scene_presets,omitempty"`
	Package        *WalmartPackage              `json:"package,omitempty"`
}
