package listingkit

type SheinStudioSelectionVariant struct {
	VariantID              int64    `json:"variant_id,omitempty"`
	VariantSKU             string   `json:"variant_sku,omitempty"`
	Size                   string   `json:"size,omitempty"`
	Color                  string   `json:"color,omitempty"`
	Price                  float64  `json:"price,omitempty"`
	Weight                 float64  `json:"weight,omitempty"`
	BoxLength              float64  `json:"box_length,omitempty"`
	BoxWidth               float64  `json:"box_width,omitempty"`
	BoxHeight              float64  `json:"box_height,omitempty"`
	ProductionCycle        int      `json:"production_cycle,omitempty"`
	PrototypeGroupID       int64    `json:"prototype_group_id,omitempty"`
	LayerID                string   `json:"layer_id,omitempty"`
	TemplateImageURL       string   `json:"template_image_url,omitempty"`
	MaskImageURL           string   `json:"mask_image_url,omitempty"`
	BlankDesignURL         string   `json:"blank_design_url,omitempty"`
	MockupImageURL         string   `json:"mockup_image_url,omitempty"`
	MockupImageURLs        []string `json:"mockup_image_urls,omitempty"`
	SizeReferenceImageURLs []string `json:"size_reference_image_urls,omitempty"`
}

type SheinStudioSelection struct {
	ProductID              int64                         `json:"product_id,omitempty"`
	ParentProductID        int64                         `json:"parent_product_id,omitempty"`
	VariantID              int64                         `json:"variant_id,omitempty"`
	PrototypeGroupID       int64                         `json:"prototype_group_id,omitempty"`
	LayerID                string                        `json:"layer_id,omitempty"`
	DesignType             string                        `json:"design_type,omitempty"`
	ProductName            string                        `json:"product_name,omitempty"`
	VariantLabel           string                        `json:"variant_label,omitempty"`
	PrintableWidth         int                           `json:"printable_width,omitempty"`
	PrintableHeight        int                           `json:"printable_height,omitempty"`
	TemplateImageURL       string                        `json:"template_image_url,omitempty"`
	MaskImageURL           string                        `json:"mask_image_url,omitempty"`
	BlankDesignURL         string                        `json:"blank_design_url,omitempty"`
	MockupImageURL         string                        `json:"mockup_image_url,omitempty"`
	MockupImageURLs        []string                      `json:"mockup_image_urls,omitempty"`
	SizeReferenceImageURLs []string                      `json:"size_reference_image_urls,omitempty"`
	SelectedVariantIDs     []int64                       `json:"selected_variant_ids,omitempty"`
	Variants               []SheinStudioSelectionVariant `json:"variants,omitempty"`
}

type SheinStudioProductImagePrompt struct {
	Role   string `json:"role,omitempty"`
	Label  string `json:"label,omitempty"`
	Prompt string `json:"prompt,omitempty"`
}

type SheinStudioSelectedSDSImageRecord struct {
	ImageURL   string `json:"image_url,omitempty"`
	VariantSKU string `json:"variant_sku,omitempty"`
	Color      string `json:"color,omitempty"`
}

type SheinStudioGroupedSelection struct {
	SelectionID        string               `json:"selection_id,omitempty"`
	Selection          SheinStudioSelection `json:"selection,omitempty"`
	BaselineKey        string               `json:"baseline_key,omitempty"`
	BaselineStatus     string               `json:"baseline_status,omitempty"`
	BaselineReason     string               `json:"baseline_reason,omitempty"`
	BaselineReasonCode string               `json:"baseline_reason_code,omitempty"`
	SheinStoreID       string               `json:"shein_store_id,omitempty"`
	Eligible           bool                 `json:"eligible"`
	EligibilityReason  string               `json:"eligibility_reason,omitempty"`
}

type StudioBatchDraft = SheinStudioSession

type StudioBatchDraftDetail struct {
	Batch   *StudioBatchDraft   `json:"batch,omitempty"`
	Designs []SheinStudioDesign `json:"designs,omitempty"`
}

type SheinStudioSessionGalleryItem struct {
	TenantID              string `json:"tenant_id,omitempty"`
	SessionID             string `json:"session_id"`
	DesignID              string `json:"design_id"`
	ImageURL              string `json:"image_url"`
	Prompt                string `json:"prompt,omitempty"`
	ProductName           string `json:"product_name,omitempty"`
	Status                string `json:"status,omitempty"`
	CreatedAt             string `json:"created_at,omitempty"`
	UpdatedAt             string `json:"updated_at,omitempty"`
	ReviewNote            string `json:"review_note,omitempty"`
	RevisedPrompt         string `json:"revised_prompt,omitempty"`
	ImageModel            string `json:"image_model,omitempty"`
	TransparentBackground bool   `json:"transparent_background,omitempty"`
	VariationIntensity    string `json:"variation_intensity,omitempty"`
}

type UpsertStudioBatchRequest struct {
	ID                      string                          `json:"id,omitempty"`
	ExpectedUpdatedAt       string                          `json:"expected_updated_at,omitempty"`
	BatchName               string                          `json:"batch_name,omitempty"`
	Prompt                  string                          `json:"prompt"`
	PromptMode              string                          `json:"prompt_mode,omitempty"`
	StyleCount              string                          `json:"style_count,omitempty"`
	VariationIntensity      string                          `json:"variation_intensity,omitempty"`
	ProductImageCount       string                          `json:"product_image_count,omitempty"`
	ProductImagePrompt      string                          `json:"product_image_prompt,omitempty"`
	ProductImagePrompts     []SheinStudioProductImagePrompt `json:"product_image_prompts,omitempty"`
	ArtworkModel            string                          `json:"artwork_model,omitempty"`
	ImageStrategy           string                          `json:"image_strategy,omitempty"`
	GroupedImageMode        string                          `json:"grouped_image_mode,omitempty"`
	SelectedSDSImages       []SheinStudioSelectedSDSImage   `json:"selected_sds_images,omitempty"`
	GroupedSelections       []SheinStudioGroupedSelection   `json:"grouped_selections,omitempty"`
	TransparentBackground   bool                            `json:"transparent_background,omitempty"`
	RenderSizeImagesWithSDS bool                            `json:"render_size_images_with_sds,omitempty"`
	SheinStoreID            string                          `json:"shein_store_id,omitempty"`
	Selection               *SheinStudioSelection           `json:"selection,omitempty"`
	ApprovedDesignIDs       []string                        `json:"approved_design_ids,omitempty"`
	CreatedTasks            []SheinStudioCreatedTask        `json:"created_tasks,omitempty"`
	GenerationJobs          []SheinStudioGenerationJob      `json:"generation_jobs,omitempty"`
	Designs                 []SheinStudioDesign             `json:"designs,omitempty"`
}

type SheinStudioBatchListItem struct {
	ID                      string                          `json:"id"`
	BatchName               string                          `json:"batch_name,omitempty"`
	Status                  string                          `json:"status,omitempty"`
	Prompt                  string                          `json:"prompt,omitempty"`
	PromptMode              string                          `json:"prompt_mode,omitempty"`
	StyleCount              string                          `json:"style_count,omitempty"`
	VariationIntensity      string                          `json:"variation_intensity,omitempty"`
	ProductImageCount       string                          `json:"product_image_count,omitempty"`
	ProductImagePrompt      string                          `json:"product_image_prompt,omitempty"`
	ProductImagePrompts     []SheinStudioProductImagePrompt `json:"product_image_prompts,omitempty"`
	ArtworkModel            string                          `json:"artwork_model,omitempty"`
	ImageStrategy           string                          `json:"image_strategy,omitempty"`
	GroupedImageMode        string                          `json:"grouped_image_mode,omitempty"`
	TransparentBackground   bool                            `json:"transparent_background,omitempty"`
	RenderSizeImagesWithSDS bool                            `json:"render_size_images_with_sds,omitempty"`
	SheinStoreID            string                          `json:"shein_store_id,omitempty"`
	Selection               *SheinStudioSelection           `json:"selection,omitempty"`
	GroupedSelections       []SheinStudioGroupedSelection   `json:"grouped_selections,omitempty"`
	ApprovedDesignIDs       []string                        `json:"approved_design_ids,omitempty"`
	CreatedTasks            []SheinStudioCreatedTask        `json:"created_tasks,omitempty"`
	GenerationJobs          []SheinStudioGenerationJob      `json:"generation_jobs,omitempty"`
	DesignCount             int                             `json:"design_count"`
	UpdatedAt               string                          `json:"updated_at,omitempty"`
}

type StudioBatchListResponse struct {
	Items []SheinStudioBatchListItem `json:"items,omitempty"`
	Total int                        `json:"total"`
}
