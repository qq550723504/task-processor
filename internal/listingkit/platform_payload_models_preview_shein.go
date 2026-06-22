package listingkit

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

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
	RequestDraft      *sheinpub.RequestDraft            `json:"request_draft,omitempty"`
	DraftPayload      *sheinpub.RequestDraft            `json:"draft_payload,omitempty"`
	PreviewProduct    *sheinproduct.Product             `json:"preview_product,omitempty"`
	PreviewPayload    *sheinproduct.Product             `json:"preview_payload,omitempty"`
	Submission        *sheinpub.SubmissionReport        `json:"submission,omitempty"`
	SubmissionState   *sheinpub.SubmissionReport        `json:"submission_state,omitempty"`
	Pricing           *sheinpub.PricingReview           `json:"pricing,omitempty"`
	FinalReview       *SheinFinalReview                 `json:"final_review,omitempty"`
	StoreResolution   *SheinStoreResolutionSummary      `json:"store_resolution,omitempty"`
	SubmissionEvents  []sheinpub.SubmissionEvent        `json:"submission_events,omitempty"`
	InspectionData    *sheinpub.Inspection              `json:"inspection_data,omitempty"`
}

type SheinStoreResolutionSummary = sheinmarketplace.StoreResolutionSummary

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

type SheinFinalReviewSKU = sheinmarketplace.FinalReviewSKU
type SheinFinalReviewImage = sheinmarketplace.FinalReviewImage

type SheinResolutionCacheSummary = sheinmarketplace.ResolutionCacheSummary
type SheinImageUploadPreflight = sheinmarketplace.ImageUploadPreflight

type SheinSourceProductSummary = sheinmarketplace.SourceProductSummary
