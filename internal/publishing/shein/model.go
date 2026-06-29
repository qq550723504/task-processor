package shein

import (
	"context"
	"time"

	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheinproduct "task-processor/internal/shein/api/product"
)

type BuildRequest struct {
	Country            string
	Language           string
	Text               string
	BrandHint          string
	TargetCategoryHint string
	SheinStoreID       int64
	ProductSize        string
	Context            context.Context `json:"-"`
}

type Package struct {
	SpuName                 string                     `json:"spu_name,omitempty"`
	BrandName               string                     `json:"brand_name,omitempty"`
	ProductNameEn           string                     `json:"product_name_en,omitempty"`
	ProductNameMulti        string                     `json:"product_name_multi,omitempty"`
	TitleDiagnostics        *TitleDiagnostics          `json:"title_diagnostics,omitempty"`
	CategoryName            string                     `json:"category_name,omitempty"`
	CategoryPath            []string                   `json:"category_path,omitempty"`
	CategoryID              int                        `json:"category_id,omitempty"`
	CategoryIDList          []int                      `json:"category_id_list,omitempty"`
	ProductTypeID           *int                       `json:"product_type_id,omitempty"`
	TopCategoryID           int                        `json:"top_category_id,omitempty"`
	CategoryResolution      *CategoryResolution        `json:"category_resolution,omitempty"`
	AttributeResolution     *AttributeResolution       `json:"attribute_resolution,omitempty"`
	SaleAttributeResolution *SaleAttributeResolution   `json:"sale_attribute_resolution,omitempty"`
	Inspection              *Inspection                `json:"inspection,omitempty"`
	Description             string                     `json:"description,omitempty"`
	SellingPoints           []string                   `json:"selling_points,omitempty"`
	Attributes              map[string]string          `json:"attributes,omitempty"`
	ProductAttributes       []common.Attribute         `json:"product_attributes,omitempty"`
	ResolvedAttributes      []ResolvedAttribute        `json:"resolved_attributes,omitempty"`
	SiteList                []common.Site              `json:"site_list,omitempty"`
	SkcList                 []SKCPackage               `json:"skc_list,omitempty"`
	Images                  *common.ImageSet           `json:"images,omitempty"`
	ImageBundle             *common.PublishImageBundle `json:"image_bundle,omitempty"`
	// Deprecated: kept only for JSON/history compatibility. New business code should use DraftPayload.
	RequestDraft *RequestDraft `json:"request_draft,omitempty"`
	// DraftPayload is the canonical SHEIN draft payload used by current business logic.
	DraftPayload *RequestDraft `json:"draft_payload,omitempty"`
	// Deprecated: kept only for JSON/history compatibility. New business code should use PreviewPayload.
	PreviewProduct *sheinproduct.Product `json:"preview_product,omitempty"`
	// PreviewPayload is the canonical SHEIN preview payload used by current business logic.
	PreviewPayload *sheinproduct.Product `json:"preview_payload,omitempty"`
	// Deprecated: kept only for JSON/history compatibility. New business code should use SubmissionState.
	Submission *SubmissionReport `json:"submission,omitempty"`
	// SubmissionState is the canonical SHEIN submission state used by current business logic.
	SubmissionState *SubmissionReport `json:"submission_state,omitempty"`
	Pricing         *PricingReview    `json:"pricing,omitempty"`
	// Deprecated: kept only for JSON/history compatibility. New business code should use FinalSubmissionDraft.
	FinalDraft *FinalDraft `json:"final_draft,omitempty"`
	// FinalSubmissionDraft is the canonical SHEIN final submission draft used by current business logic.
	FinalSubmissionDraft    *FinalDraft                              `json:"final_submission_draft,omitempty"`
	SubmissionEvents        []SubmissionEvent                        `json:"submission_events,omitempty"`
	CustomAttributeRelation []sheinattribute.CustomAttributeRelation `json:"custom_attribute_relation,omitempty"`
	Metadata                map[string]string                        `json:"metadata,omitempty"`
	ReviewNotes             []string                                 `json:"review_notes,omitempty"`
}

type PricingRule struct {
	SourceCurrency   string  `json:"source_currency,omitempty"`
	TargetCurrency   string  `json:"target_currency,omitempty"`
	ExchangeRate     float64 `json:"exchange_rate,omitempty"`
	MarkupMultiplier float64 `json:"markup_multiplier,omitempty"`
	MinimumPrice     float64 `json:"minimum_price,omitempty"`
	RoundTo          float64 `json:"round_to,omitempty"`
	PriceEnding      float64 `json:"price_ending,omitempty"`
}

type TitleDiagnostics struct {
	Source             string `json:"source,omitempty"`
	PromptContaminated bool   `json:"prompt_contaminated,omitempty"`
	ResolutionNote     string `json:"resolution_note,omitempty"`
	SKCBaseTitle       string `json:"skc_base_title,omitempty"`
}

type PricingReview struct {
	RuleSnapshot     *PricingRule         `json:"rule_snapshot,omitempty"`
	SKUPrices        []SKUPriceReview     `json:"sku_prices,omitempty"`
	ManualOverrides  map[string]float64   `json:"manual_overrides,omitempty"`
	MissingPriceSKUs []string             `json:"missing_price_skus,omitempty"`
	Cache            *ResolutionCacheInfo `json:"cache,omitempty"`
	Ready            bool                 `json:"ready"`
	UpdatedAt        *time.Time           `json:"updated_at,omitempty"`
}

type SKUPriceReview struct {
	SupplierSKU     string  `json:"supplier_sku,omitempty"`
	SupplierCode    string  `json:"supplier_code,omitempty"`
	CostCNY         float64 `json:"cost_cny,omitempty"`
	CalculatedPrice float64 `json:"calculated_price,omitempty"`
	FinalPrice      float64 `json:"final_price,omitempty"`
	Currency        string  `json:"currency,omitempty"`
	Manual          bool    `json:"manual,omitempty"`
}

type FinalDraft struct {
	Confirmed             bool               `json:"confirmed"`
	ConfirmedAt           *time.Time         `json:"confirmed_at,omitempty"`
	SubmitMode            string             `json:"submit_mode,omitempty"`
	ManualPriceOverrides  map[string]float64 `json:"manual_price_overrides,omitempty"`
	FinalImageOrder       []string           `json:"final_image_order,omitempty"`
	MainImageURL          string             `json:"main_image_url,omitempty"`
	DeletedImageURLs      []string           `json:"deleted_image_urls,omitempty"`
	ImageRoleOverrides    map[string]string  `json:"image_role_overrides,omitempty"`
	SheinImageUploadCache map[string]string  `json:"shein_image_upload_cache,omitempty"`
	UpdatedAt             *time.Time         `json:"updated_at,omitempty"`
}

type RequestDraft struct {
	SpuName                 string                                   `json:"spu_name,omitempty"`
	SupplierCode            string                                   `json:"supplier_code,omitempty"`
	MultiLanguageNameList   []LocalizedText                          `json:"multi_language_name_list,omitempty"`
	MultiLanguageDescList   []LocalizedText                          `json:"multi_language_desc_list,omitempty"`
	ProductAttributeList    []common.Attribute                       `json:"product_attribute_list,omitempty"`
	ResolvedAttributes      []ResolvedAttribute                      `json:"resolved_attributes,omitempty"`
	SizeAttributeList       []sheinproduct.SizeAttribute             `json:"size_attribute_list,omitempty"`
	ImageInfo               *ImageDraft                              `json:"image_info,omitempty"`
	SiteList                []common.Site                            `json:"site_list,omitempty"`
	SKCList                 []SKCRequestDraft                        `json:"skc_list,omitempty"`
	CustomAttributeRelation []sheinattribute.CustomAttributeRelation `json:"custom_attribute_relation,omitempty"`
}

type ImageDraft struct {
	MainImage string   `json:"main_image,omitempty"`
	Gallery   []string `json:"gallery,omitempty"`
	WhiteBg   string   `json:"white_bg,omitempty"`
	Source    []string `json:"source,omitempty"`
}

type LocalizedText struct {
	Language string `json:"language,omitempty"`
	Name     string `json:"name,omitempty"`
}

type SKCPackage struct {
	SkcName      string            `json:"skc_name,omitempty"`
	SaleName     string            `json:"sale_name,omitempty"`
	SupplierCode string            `json:"supplier_code,omitempty"`
	MainImageURL string            `json:"main_image_url,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
	SKUs         []common.Variant  `json:"skus,omitempty"`
}

type SKCRequestDraft struct {
	SkcName               string                 `json:"skc_name,omitempty"`
	SaleName              string                 `json:"sale_name,omitempty"`
	SupplierCode          string                 `json:"supplier_code,omitempty"`
	Sort                  int                    `json:"sort,omitempty"`
	SaleAttribute         *ResolvedSaleAttribute `json:"sale_attribute,omitempty"`
	MultiLanguageNameList []LocalizedText        `json:"multi_language_name_list,omitempty"`
	ImageInfo             *ImageDraft            `json:"image_info,omitempty"`
	SKUList               []SKUDraft             `json:"sku_list,omitempty"`
}

type SKUDraft struct {
	SupplierSKU    string                  `json:"supplier_sku,omitempty"`
	Attributes     map[string]string       `json:"attributes,omitempty"`
	Currency       string                  `json:"currency,omitempty"`
	CostPrice      string                  `json:"cost_price,omitempty"`
	BasePrice      string                  `json:"base_price,omitempty"`
	StockCount     int                     `json:"stock_count,omitempty"`
	Weight         float64                 `json:"weight,omitempty"`
	WeightUnit     string                  `json:"weight_unit,omitempty"`
	Length         string                  `json:"length,omitempty"`
	Width          string                  `json:"width,omitempty"`
	Height         string                  `json:"height,omitempty"`
	LengthUnit     string                  `json:"length_unit,omitempty"`
	MainImage      string                  `json:"main_image,omitempty"`
	Barcode        string                  `json:"barcode,omitempty"`
	IsDefault      bool                    `json:"is_default,omitempty"`
	SaleAttributes []ResolvedSaleAttribute `json:"sale_attributes,omitempty"`
	SitePriceList  []SitePrice             `json:"site_price_list,omitempty"`
	StockInfoList  []StockInfo             `json:"stock_info_list,omitempty"`
}

type SitePrice struct {
	SubSite   string `json:"sub_site,omitempty"`
	BasePrice string `json:"base_price,omitempty"`
	Currency  string `json:"currency,omitempty"`
}

type StockInfo struct {
	WarehouseCode string `json:"warehouse_code,omitempty"`
	InventoryNum  int    `json:"inventory_num,omitempty"`
}

type CategoryResolution struct {
	Status             string                      `json:"status,omitempty"`
	Source             string                      `json:"source,omitempty"`
	QueryText          string                      `json:"query_text,omitempty"`
	MatchedPath        []string                    `json:"matched_path,omitempty"`
	CategoryID         int                         `json:"category_id,omitempty"`
	CategoryIDList     []int                       `json:"category_id_list,omitempty"`
	ProductTypeID      int                         `json:"product_type_id,omitempty"`
	TopCategoryID      int                         `json:"top_category_id,omitempty"`
	SuggestedCategory  *CategorySuggestion         `json:"suggested_category,omitempty"`
	SemanticValidation *CategorySemanticValidation `json:"semantic_validation,omitempty"`
	Cache              *ResolutionCacheInfo        `json:"cache,omitempty"`
	ReviewNotes        []string                    `json:"review_notes,omitempty"`
}

type CategorySuggestion struct {
	Source         string   `json:"source,omitempty"`
	Reason         string   `json:"reason,omitempty"`
	MatchedPath    []string `json:"matched_path,omitempty"`
	CategoryID     int      `json:"category_id,omitempty"`
	CategoryIDList []int    `json:"category_id_list,omitempty"`
	ProductTypeID  int      `json:"product_type_id,omitempty"`
	TopCategoryID  int      `json:"top_category_id,omitempty"`
}

type CategorySemanticValidation struct {
	Source       string   `json:"source,omitempty"`
	ComparedPath []string `json:"compared_path,omitempty"`
	Verdict      string   `json:"verdict,omitempty"`
	Reason       string   `json:"reason,omitempty"`
}

type ResolvedAttribute struct {
	Name                string `json:"name,omitempty"`
	Value               string `json:"value,omitempty"`
	AttributeID         int    `json:"attribute_id,omitempty"`
	AttributeValueID    *int   `json:"attribute_value_id,omitempty"`
	AttributeExtraValue string `json:"attribute_extra_value,omitempty"`
	AttributeType       int    `json:"attribute_type,omitempty"`
	AttributeMode       int    `json:"attribute_mode,omitempty"`
	DataDimension       int    `json:"data_dimension,omitempty"`
	CascadeAttributeID  int    `json:"cascade_attribute_id,omitempty"`
	MatchedBy           string `json:"matched_by,omitempty"`
	Required            bool   `json:"required,omitempty"`
	Important           bool   `json:"important,omitempty"`
	SKCScope            bool   `json:"skc_scope,omitempty"`
}

type AttributeValueCandidate struct {
	AttributeValueID int    `json:"attribute_value_id,omitempty"`
	Value            string `json:"value,omitempty"`
	ValueEn          string `json:"value_en,omitempty"`
}

type PendingAttributeCandidate struct {
	Name               string                    `json:"name,omitempty"`
	AttributeID        int                       `json:"attribute_id,omitempty"`
	AttributeName      string                    `json:"attribute_name,omitempty"`
	AttributeNameEn    string                    `json:"attribute_name_en,omitempty"`
	AttributeType      int                       `json:"attribute_type,omitempty"`
	AttributeMode      int                       `json:"attribute_mode,omitempty"`
	AttributeInputNum  int                       `json:"attribute_input_num,omitempty"`
	DataDimension      int                       `json:"data_dimension,omitempty"`
	CascadeAttributeID int                       `json:"cascade_attribute_id,omitempty"`
	Required           bool                      `json:"required,omitempty"`
	Important          bool                      `json:"important,omitempty"`
	SKCScope           bool                      `json:"skc_scope,omitempty"`
	AttributeValueList []AttributeValueCandidate `json:"attribute_value_list,omitempty"`
}

type AttributeResolution struct {
	Status                         string                      `json:"status,omitempty"`
	Source                         string                      `json:"source,omitempty"`
	CategoryID                     int                         `json:"category_id,omitempty"`
	TemplateCount                  int                         `json:"template_count,omitempty"`
	ResolvedCount                  int                         `json:"resolved_count,omitempty"`
	UnresolvedCount                int                         `json:"unresolved_count,omitempty"`
	ResolvedAttributes             []ResolvedAttribute         `json:"resolved_attributes,omitempty"`
	PendingAttributes              []common.Attribute          `json:"pending_attributes,omitempty"`
	PendingAttributeCandidates     []PendingAttributeCandidate `json:"pending_attribute_candidates,omitempty"`
	RecommendedAttributeCandidates []PendingAttributeCandidate `json:"recommended_attribute_candidates,omitempty"`
	Cache                          *ResolutionCacheInfo        `json:"cache,omitempty"`
	ReviewNotes                    []string                    `json:"review_notes,omitempty"`
}

type ResolvedSaleAttribute struct {
	Scope            string `json:"scope,omitempty"`
	Name             string `json:"name,omitempty"`
	Value            string `json:"value,omitempty"`
	AttributeID      int    `json:"attribute_id,omitempty"`
	AttributeValueID *int   `json:"attribute_value_id,omitempty"`
	MatchedBy        string `json:"matched_by,omitempty"`
}

type SaleAttributeResolution struct {
	Status                   string                                   `json:"status,omitempty"`
	Source                   string                                   `json:"source,omitempty"`
	CategoryID               int                                      `json:"category_id,omitempty"`
	RecommendCategoryReview  bool                                     `json:"recommend_category_review,omitempty"`
	CategoryReviewReason     string                                   `json:"category_review_reason,omitempty"`
	PrimaryAttributeID       int                                      `json:"primary_attribute_id,omitempty"`
	SecondaryAttributeID     int                                      `json:"secondary_attribute_id,omitempty"`
	PrimarySourceDimension   string                                   `json:"primary_source_dimension,omitempty"`
	SecondarySourceDimension string                                   `json:"secondary_source_dimension,omitempty"`
	SourceDimensions         []SourceVariantDimension                 `json:"source_dimensions,omitempty"`
	TemplateOptions          []SaleAttributeTemplateOption            `json:"template_options,omitempty"`
	SKCAttributes            []ResolvedSaleAttribute                  `json:"skc_attributes,omitempty"`
	SKUAttributes            []ResolvedSaleAttribute                  `json:"sku_attributes,omitempty"`
	Candidates               []SaleAttributeCandidateInfo             `json:"candidates,omitempty"`
	SelectionSummary         []string                                 `json:"selection_summary,omitempty"`
	ReviewNotes              []string                                 `json:"review_notes,omitempty"`
	CustomAttributeRelation  []sheinattribute.CustomAttributeRelation `json:"custom_attribute_relation,omitempty"`
	Cache                    *ResolutionCacheInfo                     `json:"cache,omitempty"`
	ValueSanitized           bool                                     `json:"value_sanitized,omitempty"`
	ValueSanitizationSource  string                                   `json:"value_sanitization_source,omitempty"`
	ValuePromptContaminated  bool                                     `json:"value_prompt_contaminated,omitempty"`
	ValueResolutionNote      string                                   `json:"value_resolution_note,omitempty"`
	CacheRejectedReason      string                                   `json:"cache_rejected_reason,omitempty"`
	SKCValueAssignments      map[string]ResolvedSaleAttribute         `json:"skc_value_assignments,omitempty"`
	SKUValueAssignments      map[string]ResolvedSaleAttribute         `json:"sku_value_assignments,omitempty"`
	skcAssignments           map[string]ResolvedSaleAttribute
	skuAssignments           map[string][]ResolvedSaleAttribute
	skcValueAssignments      map[string]ResolvedSaleAttribute
	skuValueAssignments      map[string]ResolvedSaleAttribute
}

type SaleAttributeCandidateInfo struct {
	SourceDimension string   `json:"source_dimension,omitempty"`
	Name            string   `json:"name,omitempty"`
	AttributeID     int      `json:"attribute_id,omitempty"`
	SKCScope        bool     `json:"skc_scope,omitempty"`
	Required        bool     `json:"required,omitempty"`
	Important       bool     `json:"important,omitempty"`
	SKCDistinct     int      `json:"skc_distinct,omitempty"`
	SKUDistinct     int      `json:"sku_distinct,omitempty"`
	TotalDistinct   int      `json:"total_distinct,omitempty"`
	PrimaryScore    int      `json:"primary_score,omitempty"`
	SecondaryScore  int      `json:"secondary_score,omitempty"`
	SampleValue     string   `json:"sample_value,omitempty"`
	Reasons         []string `json:"reasons,omitempty"`
	SelectedScope   string   `json:"selected_scope,omitempty"`
}

type SaleAttributeTemplateOption struct {
	AttributeID        int                       `json:"attribute_id,omitempty"`
	Name               string                    `json:"name,omitempty"`
	NameEn             string                    `json:"name_en,omitempty"`
	SKCScope           bool                      `json:"skc_scope,omitempty"`
	Required           bool                      `json:"required,omitempty"`
	Important          bool                      `json:"important,omitempty"`
	AttributeValueList []AttributeValueCandidate `json:"attribute_value_list,omitempty"`
}

type Inspection struct {
	NeedsReview bool                `json:"needs_review"`
	Summary     []string            `json:"summary,omitempty"`
	Sections    []InspectionSection `json:"sections,omitempty"`
}

type InspectionSection struct {
	Key         string             `json:"key,omitempty"`
	Title       string             `json:"title,omitempty"`
	Status      string             `json:"status,omitempty"`
	Summary     string             `json:"summary,omitempty"`
	Highlights  []string           `json:"highlights,omitempty"`
	ActionItems []string           `json:"action_items,omitempty"`
	Actions     []InspectionAction `json:"actions,omitempty"`
}

type InspectionAction struct {
	Key         string                          `json:"key,omitempty"`
	Label       string                          `json:"label,omitempty"`
	Target      string                          `json:"target,omitempty"`
	ActionType  string                          `json:"action_type,omitempty"`
	Description string                          `json:"description,omitempty"`
	Payload     map[string]any                  `json:"payload,omitempty"`
	Category    *InspectionCategoryPayload      `json:"category,omitempty"`
	Attributes  *InspectionAttributePayload     `json:"attributes,omitempty"`
	Sale        *InspectionSaleAttributePayload `json:"sale,omitempty"`
}

type InspectionCategoryPayload struct {
	Platform          string              `json:"platform,omitempty"`
	Target            string              `json:"target,omitempty"`
	Status            string              `json:"status,omitempty"`
	Source            string              `json:"source,omitempty"`
	CategoryName      string              `json:"category_name,omitempty"`
	CategoryPath      []string            `json:"category_path,omitempty"`
	CategoryID        int                 `json:"category_id,omitempty"`
	CategoryIDList    []int               `json:"category_id_list,omitempty"`
	ProductTypeID     *int                `json:"product_type_id,omitempty"`
	TopCategoryID     int                 `json:"top_category_id,omitempty"`
	SuggestedCategory *CategorySuggestion `json:"suggested_category,omitempty"`
	ReviewNotes       []string            `json:"review_notes,omitempty"`
}

type InspectionAttributePayload struct {
	Platform                       string                      `json:"platform,omitempty"`
	Target                         string                      `json:"target,omitempty"`
	Status                         string                      `json:"status,omitempty"`
	Source                         string                      `json:"source,omitempty"`
	TemplateCount                  int                         `json:"template_count,omitempty"`
	ResolvedCount                  int                         `json:"resolved_count,omitempty"`
	UnresolvedCount                int                         `json:"unresolved_count,omitempty"`
	ProductAttributes              []common.Attribute          `json:"product_attributes,omitempty"`
	ResolvedAttributes             []ResolvedAttribute         `json:"resolved_attributes,omitempty"`
	PendingAttributes              []common.Attribute          `json:"pending_attributes,omitempty"`
	PendingAttributeCandidates     []PendingAttributeCandidate `json:"pending_attribute_candidates,omitempty"`
	RecommendedAttributeCandidates []PendingAttributeCandidate `json:"recommended_attribute_candidates,omitempty"`
	ReviewNotes                    []string                    `json:"review_notes,omitempty"`
}

type InspectionSaleAttributePayload struct {
	Platform                 string                           `json:"platform,omitempty"`
	Target                   string                           `json:"target,omitempty"`
	Status                   string                           `json:"status,omitempty"`
	Source                   string                           `json:"source,omitempty"`
	RecommendCategoryReview  bool                             `json:"recommend_category_review,omitempty"`
	CategoryReviewReason     string                           `json:"category_review_reason,omitempty"`
	PrimaryAttributeID       int                              `json:"primary_attribute_id,omitempty"`
	SecondaryAttributeID     int                              `json:"secondary_attribute_id,omitempty"`
	PrimarySourceDimension   string                           `json:"primary_source_dimension,omitempty"`
	SecondarySourceDimension string                           `json:"secondary_source_dimension,omitempty"`
	SKCValueAssignments      map[string]ResolvedSaleAttribute `json:"skc_value_assignments,omitempty"`
	SKUValueAssignments      map[string]ResolvedSaleAttribute `json:"sku_value_assignments,omitempty"`
	SourceDimensions         []SourceVariantDimension         `json:"source_dimensions,omitempty"`
	TemplateOptions          []SaleAttributeTemplateOption    `json:"template_options,omitempty"`
	SelectionSummary         []string                         `json:"selection_summary,omitempty"`
	SKCAttributes            []ResolvedSaleAttribute          `json:"skc_attributes,omitempty"`
	SKUAttributes            []ResolvedSaleAttribute          `json:"sku_attributes,omitempty"`
	CandidateCount           int                              `json:"candidate_count,omitempty"`
	Candidates               []SaleAttributeCandidateInfo     `json:"candidates,omitempty"`
	SKCPatches               []InspectionSKCPatchPayload      `json:"skc_patches,omitempty"`
	ReviewNotes              []string                         `json:"review_notes,omitempty"`
}

type InspectionSKCPatchPayload struct {
	SupplierCode  string                      `json:"supplier_code,omitempty"`
	SkcName       string                      `json:"skc_name,omitempty"`
	SaleName      string                      `json:"sale_name,omitempty"`
	MainImageURL  string                      `json:"main_image_url,omitempty"`
	Attributes    map[string]string           `json:"attributes,omitempty"`
	SaleAttribute *ResolvedSaleAttribute      `json:"sale_attribute,omitempty"`
	SKUPatches    []InspectionSKUPatchPayload `json:"sku_patches,omitempty"`
}

type InspectionSKUPatchPayload struct {
	SupplierSKU    string                  `json:"supplier_sku,omitempty"`
	Attributes     map[string]string       `json:"attributes,omitempty"`
	BasePrice      string                  `json:"base_price,omitempty"`
	CostPrice      string                  `json:"cost_price,omitempty"`
	Currency       string                  `json:"currency,omitempty"`
	StockCount     int                     `json:"stock_count,omitempty"`
	MainImage      string                  `json:"main_image,omitempty"`
	Barcode        string                  `json:"barcode,omitempty"`
	SaleAttributes []ResolvedSaleAttribute `json:"sale_attributes,omitempty"`
	SitePriceList  []SitePrice             `json:"site_price_list,omitempty"`
	StockInfoList  []StockInfo             `json:"stock_info_list,omitempty"`
}
