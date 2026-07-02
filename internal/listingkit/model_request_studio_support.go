package listingkit

type SheinStudioOptions struct {
	StyleID                 string                        `json:"style_id,omitempty"`
	StyleName               string                        `json:"style_name,omitempty"`
	SourceDesignURLs        []string                      `json:"source_design_urls,omitempty"`
	SourceDesignWidth       int                           `json:"source_design_width,omitempty"`
	SourceDesignHeight      int                           `json:"source_design_height,omitempty"`
	ProductImageURLs        []string                      `json:"product_image_urls,omitempty"`
	SelectedSDSImages       []SheinStudioSelectedSDSImage `json:"selected_sds_images,omitempty"`
	VariantProductImages    []SheinStudioVariantImageSet  `json:"variant_product_images,omitempty"`
	SizeReferenceImageURLs  []string                      `json:"size_reference_image_urls,omitempty"`
	RenderSizeImagesWithSDS bool                          `json:"render_size_images_with_sds,omitempty"`
}

type SheinStudioSelectedSDSImage struct {
	ImageURL   string `json:"image_url,omitempty"`
	VariantSKU string `json:"variant_sku,omitempty"`
	Color      string `json:"color,omitempty"`
}

type SheinStudioVariantImageSet struct {
	VariantSKU string   `json:"variant_sku,omitempty"`
	Color      string   `json:"color,omitempty"`
	ImageURLs  []string `json:"image_urls,omitempty"`
}

type StudioProductImageRequest struct {
	Prompt                    string                     `json:"prompt,omitempty"`
	PromptMode                string                     `json:"prompt_mode,omitempty"`
	ProductName               string                     `json:"product_name,omitempty"`
	CategoryPath              []string                   `json:"category_path,omitempty"`
	StyleName                 string                     `json:"style_name,omitempty"`
	SourceDesignURL           string                     `json:"source_design_url,omitempty"`
	ProductReferenceImageURLs []string                   `json:"product_reference_image_urls,omitempty"`
	CustomPrompt              string                     `json:"custom_prompt,omitempty"`
	ImagePrompts              []StudioProductImagePrompt `json:"image_prompts,omitempty"`
	Count                     int                        `json:"count,omitempty"`
}

type StudioProductImagePrompt struct {
	Role   string `json:"role,omitempty"`
	Prompt string `json:"prompt,omitempty"`
}

type StudioProductImageResponse struct {
	Images   []StudioGeneratedImage `json:"images,omitempty"`
	Warnings []string               `json:"warnings,omitempty"`
}

type StudioGeneratedImage struct {
	ID                    string  `json:"id"`
	ImageURL              string  `json:"image_url"`
	Prompt                string  `json:"prompt,omitempty"`
	RevisedPrompt         string  `json:"revised_prompt,omitempty"`
	ImageModel            string  `json:"image_model,omitempty"`
	TransparentBackground bool    `json:"transparent_background,omitempty"`
	VariationIntensity    string  `json:"variation_intensity,omitempty"`
	Role                  string  `json:"role,omitempty"`
	RoleLabel             string  `json:"role_label,omitempty"`
	RequestID             string  `json:"request_id,omitempty"`
	UpstreamJobID         string  `json:"upstream_job_id,omitempty"`
	RawResponse           string  `json:"raw_response,omitempty"`
	Usage                 AIUsage `json:"usage,omitempty"`
}

type StudioDesignRequest struct {
	Prompt                    string   `json:"prompt,omitempty"`
	PromptMode                string   `json:"prompt_mode,omitempty"`
	Count                     int      `json:"count,omitempty"`
	VariationIntensity        string   `json:"variation_intensity,omitempty"`
	PrintableWidth            int      `json:"printable_width,omitempty"`
	PrintableHeight           int      `json:"printable_height,omitempty"`
	ProductReferenceImageURLs []string `json:"product_reference_image_urls,omitempty"`
	ImageModel                string   `json:"image_model,omitempty"`
	TransparentBackground     bool     `json:"transparent_background,omitempty"`
}

type StudioDesignResponse struct {
	Prompt                string                 `json:"prompt"`
	PrintableWidth        int                    `json:"printable_width,omitempty"`
	PrintableHeight       int                    `json:"printable_height,omitempty"`
	ImageModel            string                 `json:"image_model,omitempty"`
	TransparentBackground bool                   `json:"transparent_background"`
	RequestID             string                 `json:"request_id,omitempty"`
	UpstreamJobID         string                 `json:"upstream_job_id,omitempty"`
	RawResponse           string                 `json:"raw_response,omitempty"`
	Usage                 AIUsage                `json:"usage,omitempty"`
	Images                []StudioGeneratedImage `json:"images,omitempty"`
	Warnings              []string               `json:"warnings,omitempty"`
}

type StudioReferenceAnalysisRequest struct {
	ReferenceImageURLs []string `json:"reference_image_urls,omitempty"`
	ProductName        string   `json:"product_name,omitempty"`
	CategoryPath       []string `json:"category_path,omitempty"`
	BasePrompt         string   `json:"base_prompt,omitempty"`
	UserInstruction    string   `json:"user_instruction,omitempty"`
}

type StudioReferenceAnalysisResponse struct {
	ReferenceStyleBrief string   `json:"reference_style_brief,omitempty"`
	SanitizedPrompt     string   `json:"sanitized_prompt,omitempty"`
	Warnings            []string `json:"warnings,omitempty"`
}

type SDSSyncOptions struct {
	VariantID              int64                  `json:"variant_id,omitempty"`
	ParentProductID        int64                  `json:"parent_product_id,omitempty"`
	PrototypeGroupID       int64                  `json:"prototype_group_id,omitempty"`
	LayerID                string                 `json:"layer_id,omitempty"`
	DesignType             string                 `json:"design_type,omitempty"`
	FitLevel               float64                `json:"fit_level,omitempty"`
	ResizeMode             int                    `json:"resize_mode,omitempty"`
	ProductName            string                 `json:"product_name,omitempty"`
	ProductSKU             string                 `json:"product_sku,omitempty"`
	ProductEnglishName     string                 `json:"product_english_name,omitempty"`
	CategoryPath           []string               `json:"category_path,omitempty"`
	Material               string                 `json:"material,omitempty"`
	MaterialDescription    string                 `json:"material_description,omitempty"`
	ProductionProcess      string                 `json:"production_process,omitempty"`
	ProductPerformance     string                 `json:"product_performance,omitempty"`
	ApplicableScenarios    string                 `json:"applicable_scenarios,omitempty"`
	WashingInstructions    string                 `json:"washing_instructions,omitempty"`
	SpecialDescription     string                 `json:"special_description,omitempty"`
	ProductSize            string                 `json:"product_size,omitempty"`
	PackagingSpecification string                 `json:"packaging_specification,omitempty"`
	DesignArea             string                 `json:"design_area,omitempty"`
	PictureRequest         string                 `json:"picture_request,omitempty"`
	IsElectricity          *int                   `json:"is_electricity,omitempty"`
	VariantSKU             string                 `json:"variant_sku,omitempty"`
	VariantSize            string                 `json:"variant_size,omitempty"`
	VariantColor           string                 `json:"variant_color,omitempty"`
	VariantPrice           float64                `json:"variant_price,omitempty"`
	VariantWeight          float64                `json:"variant_weight,omitempty"`
	ProductionCycle        int                    `json:"production_cycle,omitempty"`
	BlankDesignURL         string                 `json:"blank_design_url,omitempty"`
	TemplateImageURL       string                 `json:"template_image_url,omitempty"`
	MaskImageURL           string                 `json:"mask_image_url,omitempty"`
	PrintableWidth         int                    `json:"printable_width,omitempty"`
	PrintableHeight        int                    `json:"printable_height,omitempty"`
	MockupImageURLs        []string               `json:"mockup_image_urls,omitempty"`
	StyleID                string                 `json:"style_id,omitempty"`
	StyleName              string                 `json:"style_name,omitempty"`
	Variants               []SDSSyncVariantOption `json:"variants,omitempty"`
}

type SDSSyncVariantOption struct {
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
