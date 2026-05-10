package listingkit

import (
	"time"

	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
)

type GenerateRequest struct {
	TenantID           string           `json:"tenant_id,omitempty"`
	UserID             string           `json:"user_id,omitempty"`
	ImageURLs          []string         `json:"image_urls,omitempty"`
	Text               string           `json:"text,omitempty"`
	ProductURL         string           `json:"product_url,omitempty"`
	Platforms          []string         `json:"platforms,omitempty"`
	Country            string           `json:"country,omitempty"`
	Language           string           `json:"language,omitempty"`
	SheinStoreID       int64            `json:"shein_store_id,omitempty"`
	TargetCategoryHint string           `json:"target_category_hint,omitempty"`
	BrandHint          string           `json:"brand_hint,omitempty"`
	Options            *GenerateOptions `json:"options,omitempty"`
}

type GenerateOptions struct {
	ImageStrategy string                               `json:"image_strategy,omitempty"`
	ProcessImages bool                                 `json:"process_images"`
	Scene         *productimage.SceneGenerationOptions `json:"scene,omitempty"`
	SheinStudio   *SheinStudioOptions                  `json:"shein_studio,omitempty"`
	SDS           *SDSSyncOptions                      `json:"sds,omitempty"`
}

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
	Images []StudioGeneratedImage `json:"images,omitempty"`
}

type StudioGeneratedImage struct {
	ID                    string `json:"id"`
	ImageURL              string `json:"image_url"`
	Prompt                string `json:"prompt,omitempty"`
	RevisedPrompt         string `json:"revised_prompt,omitempty"`
	ImageModel            string `json:"image_model,omitempty"`
	TransparentBackground bool   `json:"transparent_background,omitempty"`
	VariationIntensity    string `json:"variation_intensity,omitempty"`
	Role                  string `json:"role,omitempty"`
	RoleLabel             string `json:"role_label,omitempty"`
}

type StudioDesignRequest struct {
	Prompt                    string   `json:"prompt,omitempty"`
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
	Images                []StudioGeneratedImage `json:"images,omitempty"`
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

type SubmitTaskRequest struct {
	Platform       string `json:"platform,omitempty"`
	Action         string `json:"action,omitempty"`
	ConfirmedFinal bool   `json:"confirmed_final,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type SheinSettings struct {
	DefaultStoreID    int64                `json:"default_store_id,omitempty"`
	Site              string               `json:"site,omitempty"`
	WarehouseCode     string               `json:"warehouse_code,omitempty"`
	DefaultStock      int                  `json:"default_stock,omitempty"`
	DefaultSubmitMode string               `json:"default_submit_mode,omitempty"`
	Pricing           sheinpub.PricingRule `json:"pricing,omitempty"`
	UpdatedAt         *time.Time           `json:"updated_at,omitempty"`
}

type AIClientSettings struct {
	Scope         string `json:"scope,omitempty"`
	ClientName    string `json:"client_name,omitempty"`
	APIKey        string `json:"api_key,omitempty"`
	APIKeySet     bool   `json:"api_key_set"`
	BaseURL       string `json:"base_url,omitempty"`
	Model         string `json:"model,omitempty"`
	TimeoutSecond int    `json:"timeout_second,omitempty"`
	Enabled       bool   `json:"enabled"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

type SheinPricePreviewRequest struct {
	Rule            *sheinpub.PricingRule `json:"rule,omitempty"`
	ManualOverrides map[string]float64    `json:"manual_price_overrides,omitempty"`
	ApplyToTask     bool                  `json:"apply_to_task,omitempty"`
}

type SheinFinalDraftUpdateRequest struct {
	Confirmed            *bool              `json:"confirmed,omitempty"`
	SubmitMode           string             `json:"submit_mode,omitempty"`
	ManualPriceOverrides map[string]float64 `json:"manual_price_overrides,omitempty"`
	FinalImageOrder      *[]string          `json:"final_image_order,omitempty"`
	MainImageURL         string             `json:"main_image_url,omitempty"`
	DeletedImageURLs     *[]string          `json:"deleted_image_urls,omitempty"`
	ImageRoleOverrides   map[string]string  `json:"image_role_overrides,omitempty"`
}

type SheinSubmissionEventPage struct {
	TaskID string                     `json:"task_id"`
	Items  []sheinpub.SubmissionEvent `json:"items,omitempty"`
}

type SheinResolutionCacheClearResult struct {
	TaskID       string   `json:"task_id"`
	Kind         string   `json:"kind"`
	DeletedKinds []string `json:"deleted_kinds,omitempty"`
}

type SheinCategorySearchCandidate struct {
	CategoryID     int      `json:"category_id"`
	CategoryIDList []int    `json:"category_id_list,omitempty"`
	CategoryPath   []string `json:"category_path,omitempty"`
	ProductTypeID  int      `json:"product_type_id,omitempty"`
	TopCategoryID  int      `json:"top_category_id,omitempty"`
	Source         string   `json:"source,omitempty"`
	MatchReason    string   `json:"match_reason,omitempty"`
}

type SheinCategorySearchResult struct {
	TaskID string                         `json:"task_id"`
	Query  string                         `json:"query"`
	Items  []SheinCategorySearchCandidate `json:"items,omitempty"`
}
