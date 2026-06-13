package listingkit

import (
	"time"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	"task-processor/internal/catalog"
	previewdomain "task-processor/internal/listing/preview"
	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
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
	SourceProduct     *SheinSourceProductSummary        `json:"source_product,omitempty"`
	NeedsReview       bool                              `json:"needs_review"`
	Summary           []string                          `json:"summary,omitempty"`
	ReviewNotes       []string                          `json:"review_notes,omitempty"`
	Inspection        *sheinpub.Inspection              `json:"inspection,omitempty"`
	SubmitReadiness   *SheinSubmitReadiness             `json:"submit_readiness,omitempty"`
	SubmitChecklist   *SheinSubmitChecklist             `json:"submit_checklist,omitempty"`
	ImageUpload       *SheinImageUploadPreflight        `json:"image_upload,omitempty"`
	ResolutionCache   *SheinResolutionCacheSummary      `json:"resolution_cache,omitempty"`
	RepairCenter      *SheinRepairCenter                `json:"repair_center,omitempty"`
	StatusOverview    *sheinworkspace.StatusOverview    `json:"status_overview,omitempty"`
	WorkspaceOverview *sheinworkspace.WorkspaceOverview `json:"workspace_overview,omitempty"`
	EditorContext     *SheinEditorContext               `json:"editor_context,omitempty"`
	ImageBundle       *common.PublishImageBundle        `json:"image_bundle,omitempty"`
	RenderPreviews    *PlatformAssetRenderPreviews      `json:"render_previews,omitempty"`
	ScenePresets      []PlatformScenePresetSummary      `json:"scene_presets,omitempty"`
	// Deprecated: kept only for preview JSON compatibility. New business code should use DraftPayload.
	RequestDraft *sheinpub.RequestDraft `json:"request_draft,omitempty"`
	// DraftPayload is the canonical SHEIN draft payload exposed to internal preview builders.
	DraftPayload *sheinpub.RequestDraft `json:"draft_payload,omitempty"`
	// Deprecated: kept only for preview JSON compatibility. New business code should use PreviewPayload.
	PreviewProduct *sheinproduct.Product `json:"preview_product,omitempty"`
	// PreviewPayload is the canonical SHEIN preview payload exposed to internal preview builders.
	PreviewPayload *sheinproduct.Product `json:"preview_payload,omitempty"`
	// Deprecated: kept only for preview JSON compatibility. New business code should use SubmissionState.
	Submission *sheinpub.SubmissionReport `json:"submission,omitempty"`
	// SubmissionState is the canonical SHEIN submission state exposed to internal preview builders.
	SubmissionState  *sheinpub.SubmissionReport   `json:"submission_state,omitempty"`
	Pricing          *sheinpub.PricingReview      `json:"pricing,omitempty"`
	FinalReview      *SheinFinalReview            `json:"final_review,omitempty"`
	StoreResolution  *SheinStoreResolutionSummary `json:"store_resolution,omitempty"`
	SubmissionEvents []sheinpub.SubmissionEvent   `json:"submission_events,omitempty"`
	InspectionData   *sheinpub.Inspection         `json:"inspection_data,omitempty"`
}

type SheinStoreResolutionSummary struct {
	StoreID          int64    `json:"store_id,omitempty"`
	Site             string   `json:"site,omitempty"`
	Strategy         string   `json:"strategy,omitempty"`
	Reason           string   `json:"reason,omitempty"`
	MatchedRuleKinds []string `json:"matched_rule_kinds,omitempty"`
	MatchedProfileID int64    `json:"matched_profile_id,omitempty"`
	ManualOverride   bool     `json:"manual_override,omitempty"`
	Fallback         bool     `json:"fallback,omitempty"`
	ResolvedAt       string   `json:"resolved_at,omitempty"`
}

type SheinFinalReview struct {
	Confirmed      bool                             `json:"confirmed"`
	SubmitMode     string                           `json:"submit_mode,omitempty"`
	StoreID        int64                            `json:"store_id,omitempty"`
	Site           string                           `json:"site,omitempty"`
	SourceProduct  *SheinSourceProductSummary       `json:"source_product,omitempty"`
	Title          string                           `json:"title,omitempty"`
	Description    string                           `json:"description,omitempty"`
	CategoryPath   []string                         `json:"category_path,omitempty"`
	CategoryID     int                              `json:"category_id,omitempty"`
	Attributes     []sheinpub.ResolvedAttribute     `json:"attributes,omitempty"`
	SaleAttributes []sheinpub.ResolvedSaleAttribute `json:"sale_attributes,omitempty"`
	SKUs           []SheinFinalReviewSKU            `json:"skus,omitempty"`
	Images         []SheinFinalReviewImage          `json:"images,omitempty"`
	BlockingItems  []SheinReadinessItem             `json:"blocking_items,omitempty"`
}

type SheinFinalReviewSKU struct {
	SupplierCode string  `json:"supplier_code,omitempty"`
	SupplierSKU  string  `json:"supplier_sku,omitempty"`
	Color        string  `json:"color,omitempty"`
	Size         string  `json:"size,omitempty"`
	Price        float64 `json:"price,omitempty"`
	Currency     string  `json:"currency,omitempty"`
	Stock        int     `json:"stock,omitempty"`
	Weight       float64 `json:"weight,omitempty"`
}

type SheinFinalReviewImage struct {
	URL     string `json:"url,omitempty"`
	Role    string `json:"role,omitempty"`
	Sort    int    `json:"sort,omitempty"`
	Final   bool   `json:"final"`
	Main    bool   `json:"main,omitempty"`
	Swatch  bool   `json:"swatch,omitempty"`
	SizeMap bool   `json:"size_map,omitempty"`
}

type SheinResolutionCacheSummary struct {
	Category       *sheinpub.ResolutionCacheInfo `json:"category,omitempty"`
	Attributes     *sheinpub.ResolutionCacheInfo `json:"attributes,omitempty"`
	SaleAttributes *sheinpub.ResolutionCacheInfo `json:"sale_attributes,omitempty"`
	Pricing        *sheinpub.ResolutionCacheInfo `json:"pricing,omitempty"`
}

type SheinImageUploadPreflight struct {
	TotalImageReferences int      `json:"total_image_references"`
	UniqueImageURLs      int      `json:"unique_image_urls"`
	PendingUploadURLs    int      `json:"pending_upload_urls"`
	SheinUploadedURLs    int      `json:"shein_uploaded_urls"`
	SDSMockupURLs        int      `json:"sds_mockup_urls"`
	UsesSDSMockups       bool     `json:"uses_sds_mockups"`
	ReadyForUpload       bool     `json:"ready_for_upload"`
	Summary              []string `json:"summary,omitempty"`
}

type SheinSourceProductSummary struct {
	Title           string            `json:"title,omitempty"`
	SKU             string            `json:"sku,omitempty"`
	CategoryPath    []string          `json:"category_path,omitempty"`
	Attributes      map[string]string `json:"attributes,omitempty"`
	VariantSKU      string            `json:"variant_sku,omitempty"`
	VariantSize     string            `json:"variant_size,omitempty"`
	VariantColor    string            `json:"variant_color,omitempty"`
	VariantPrice    float64           `json:"variant_price,omitempty"`
	VariantWeight   float64           `json:"variant_weight,omitempty"`
	ProductionCycle string            `json:"production_cycle,omitempty"`
	ImageURLs       []string          `json:"image_urls,omitempty"`
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
