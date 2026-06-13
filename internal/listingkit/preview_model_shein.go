package listingkit

import (
	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
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
