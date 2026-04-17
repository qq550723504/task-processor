package listingkit

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	sheinproduct "task-processor/internal/shein/api/product"
)

var ErrTaskNotFound = errors.New("task not found")
var ErrTaskNotPending = errors.New("task is not pending")

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
)

type GenerateRequest struct {
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
	ProcessImages bool `json:"process_images"`
}

type Task struct {
	ID         string            `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Request    *GenerateRequest  `json:"request" gorm:"type:text"`
	Status     TaskStatus        `json:"status" gorm:"type:varchar(20);index"`
	Result     *ListingKitResult `json:"result,omitempty" gorm:"type:text"`
	Error      string            `json:"error,omitempty" gorm:"type:text"`
	CreatedAt  time.Time         `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time         `json:"updated_at" gorm:"autoUpdateTime"`
	RetryCount int               `json:"retry_count" gorm:"default:0"`
}

type TaskResult struct {
	TaskID      string            `json:"task_id"`
	Status      TaskStatus        `json:"status"`
	Result      *ListingKitResult `json:"result,omitempty"`
	Error       string            `json:"error,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
}

type ChildTaskState struct {
	Kind   string `json:"kind"`
	TaskID string `json:"task_id,omitempty"`
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

type ListingKitResult struct {
	TaskID               string                           `json:"task_id"`
	Status               string                           `json:"status"`
	Platforms            []string                         `json:"platforms,omitempty"`
	Country              string                           `json:"country,omitempty"`
	Language             string                           `json:"language,omitempty"`
	CanonicalProduct     *productenrich.CanonicalProduct  `json:"canonical_product,omitempty"`
	ImageAssets          *productimage.ImageProcessResult `json:"image_assets,omitempty"`
	Amazon               *AmazonPackage                   `json:"amazon,omitempty"`
	Shein                *SheinPackage                    `json:"shein,omitempty"`
	Temu                 *TemuPackage                     `json:"temu,omitempty"`
	Walmart              *WalmartPackage                  `json:"walmart,omitempty"`
	Summary              *GenerationSummary               `json:"summary,omitempty"`
	Revision             *ListingKitRevisionSummary       `json:"revision,omitempty"`
	RevisionHistoryTotal int                              `json:"revision_history_total,omitempty"`
	RevisionHistory      []ListingKitRevisionRecord       `json:"revision_history,omitempty"`
	ChildTasks           []ChildTaskState                 `json:"child_tasks,omitempty"`
	CreatedAt            time.Time                        `json:"created_at"`
	UpdatedAt            time.Time                        `json:"updated_at"`
}

type GenerationSummary struct {
	SourceType   string   `json:"source_type,omitempty"`
	ImageCount   int      `json:"image_count"`
	VariantCount int      `json:"variant_count"`
	NeedsReview  bool     `json:"needs_review"`
	Warnings     []string `json:"warnings,omitempty"`
}

type AmazonPackage struct {
	Draft *amazonlisting.AmazonListingDraft `json:"draft,omitempty"`
}

type SheinPackage struct {
	SpuName                 string                        `json:"spu_name,omitempty"`
	BrandName               string                        `json:"brand_name,omitempty"`
	ProductNameEn           string                        `json:"product_name_en,omitempty"`
	ProductNameMulti        string                        `json:"product_name_multi,omitempty"`
	CategoryName            string                        `json:"category_name,omitempty"`
	CategoryPath            []string                      `json:"category_path,omitempty"`
	CategoryID              int                           `json:"category_id,omitempty"`
	CategoryIDList          []int                         `json:"category_id_list,omitempty"`
	ProductTypeID           *int                          `json:"product_type_id,omitempty"`
	TopCategoryID           int                           `json:"top_category_id,omitempty"`
	CategoryResolution      *SheinCategoryResolution      `json:"category_resolution,omitempty"`
	AttributeResolution     *SheinAttributeResolution     `json:"attribute_resolution,omitempty"`
	SaleAttributeResolution *SheinSaleAttributeResolution `json:"sale_attribute_resolution,omitempty"`
	Inspection              *SheinInspection              `json:"inspection,omitempty"`
	Description             string                        `json:"description,omitempty"`
	SellingPoints           []string                      `json:"selling_points,omitempty"`
	Attributes              map[string]string             `json:"attributes,omitempty"`
	ProductAttributes       []PlatformAttribute           `json:"product_attributes,omitempty"`
	ResolvedAttributes      []SheinResolvedAttribute      `json:"resolved_attributes,omitempty"`
	SiteList                []PlatformSite                `json:"site_list,omitempty"`
	SkcList                 []SheinSKCPackage             `json:"skc_list,omitempty"`
	Images                  *PlatformImageSet             `json:"images,omitempty"`
	RequestDraft            *SheinRequestDraft            `json:"request_draft,omitempty"`
	PreviewProduct          *sheinproduct.Product         `json:"preview_product,omitempty"`
	Metadata                map[string]string             `json:"metadata,omitempty"`
	ReviewNotes             []string                      `json:"review_notes,omitempty"`
}

type TemuPackage struct {
	GoodsName          string            `json:"goods_name,omitempty"`
	CategoryPath       []string          `json:"category_path,omitempty"`
	ShortDescription   string            `json:"short_description,omitempty"`
	BulletPoints       []string          `json:"bullet_points,omitempty"`
	Attributes         map[string]string `json:"attributes,omitempty"`
	SkcList            []TemuSKCPackage  `json:"skc_list,omitempty"`
	BatchSkuInfo       *TemuBatchSKUInfo `json:"batch_sku_info,omitempty"`
	Images             *PlatformImageSet `json:"images,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	CategoryDisclaimer []string          `json:"category_disclaimer,omitempty"`
	ReviewNotes        []string          `json:"review_notes,omitempty"`
}

type WalmartPackage struct {
	ProductName      string            `json:"product_name,omitempty"`
	Brand            string            `json:"brand,omitempty"`
	ProductType      string            `json:"product_type,omitempty"`
	ShortDescription string            `json:"short_description,omitempty"`
	LongDescription  string            `json:"long_description,omitempty"`
	KeyFeatures      []string          `json:"key_features,omitempty"`
	Attributes       map[string]string `json:"attributes,omitempty"`
	Variants         []PlatformVariant `json:"variants,omitempty"`
	Images           *PlatformImageSet `json:"images,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	ReviewNotes      []string          `json:"review_notes,omitempty"`
}

type PlatformVariant struct {
	SKU        string            `json:"sku,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Price      *PlatformPrice    `json:"price,omitempty"`
	Stock      int               `json:"stock,omitempty"`
	Image      string            `json:"image,omitempty"`
	Barcode    string            `json:"barcode,omitempty"`
	IsDefault  bool              `json:"is_default,omitempty"`
}

type PlatformPrice struct {
	Currency  string  `json:"currency,omitempty"`
	Amount    float64 `json:"amount,omitempty"`
	CostPrice float64 `json:"cost_price,omitempty"`
}

type PlatformImageSet struct {
	MainImage    string   `json:"main_image,omitempty"`
	WhiteBgImage string   `json:"white_bg_image,omitempty"`
	Gallery      []string `json:"gallery,omitempty"`
	SourceImages []string `json:"source_images,omitempty"`
}

type PlatformAttribute struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type PlatformSite struct {
	MainSite string   `json:"main_site,omitempty"`
	SubSites []string `json:"sub_sites,omitempty"`
}

type SheinSKCPackage struct {
	SkcName      string            `json:"skc_name,omitempty"`
	SaleName     string            `json:"sale_name,omitempty"`
	SupplierCode string            `json:"supplier_code,omitempty"`
	MainImageURL string            `json:"main_image_url,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
	SKUs         []PlatformVariant `json:"skus,omitempty"`
}

type SheinRequestDraft struct {
	SpuName               string                   `json:"spu_name,omitempty"`
	SupplierCode          string                   `json:"supplier_code,omitempty"`
	MultiLanguageNameList []LocalizedText          `json:"multi_language_name_list,omitempty"`
	MultiLanguageDescList []LocalizedText          `json:"multi_language_desc_list,omitempty"`
	ProductAttributeList  []PlatformAttribute      `json:"product_attribute_list,omitempty"`
	ResolvedAttributes    []SheinResolvedAttribute `json:"resolved_attributes,omitempty"`
	ImageInfo             *SheinImageDraft         `json:"image_info,omitempty"`
	SiteList              []PlatformSite           `json:"site_list,omitempty"`
	SKCList               []SheinSKCRequestDraft   `json:"skc_list,omitempty"`
}

type SheinImageDraft struct {
	MainImage string   `json:"main_image,omitempty"`
	Gallery   []string `json:"gallery,omitempty"`
	WhiteBg   string   `json:"white_bg,omitempty"`
	Source    []string `json:"source,omitempty"`
}

type LocalizedText struct {
	Language string `json:"language,omitempty"`
	Name     string `json:"name,omitempty"`
}

type SheinSKCRequestDraft struct {
	SkcName               string                      `json:"skc_name,omitempty"`
	SaleName              string                      `json:"sale_name,omitempty"`
	SupplierCode          string                      `json:"supplier_code,omitempty"`
	Sort                  int                         `json:"sort,omitempty"`
	SaleAttribute         *SheinResolvedSaleAttribute `json:"sale_attribute,omitempty"`
	MultiLanguageNameList []LocalizedText             `json:"multi_language_name_list,omitempty"`
	ImageInfo             *SheinImageDraft            `json:"image_info,omitempty"`
	SKUList               []SheinSKUDraft             `json:"sku_list,omitempty"`
}

type SheinSKUDraft struct {
	SupplierSKU    string                       `json:"supplier_sku,omitempty"`
	Attributes     map[string]string            `json:"attributes,omitempty"`
	Currency       string                       `json:"currency,omitempty"`
	CostPrice      string                       `json:"cost_price,omitempty"`
	BasePrice      string                       `json:"base_price,omitempty"`
	StockCount     int                          `json:"stock_count,omitempty"`
	Weight         float64                      `json:"weight,omitempty"`
	WeightUnit     string                       `json:"weight_unit,omitempty"`
	Length         string                       `json:"length,omitempty"`
	Width          string                       `json:"width,omitempty"`
	Height         string                       `json:"height,omitempty"`
	LengthUnit     string                       `json:"length_unit,omitempty"`
	MainImage      string                       `json:"main_image,omitempty"`
	Barcode        string                       `json:"barcode,omitempty"`
	IsDefault      bool                         `json:"is_default,omitempty"`
	SaleAttributes []SheinResolvedSaleAttribute `json:"sale_attributes,omitempty"`
	SitePriceList  []SheinSitePrice             `json:"site_price_list,omitempty"`
	StockInfoList  []SheinStockInfo             `json:"stock_info_list,omitempty"`
}

type SheinSitePrice struct {
	SubSite   string `json:"sub_site,omitempty"`
	BasePrice string `json:"base_price,omitempty"`
	Currency  string `json:"currency,omitempty"`
}

type SheinStockInfo struct {
	WarehouseCode string `json:"warehouse_code,omitempty"`
	InventoryNum  int    `json:"inventory_num,omitempty"`
}

type SheinCategoryResolution struct {
	Status         string   `json:"status,omitempty"`
	Source         string   `json:"source,omitempty"`
	QueryText      string   `json:"query_text,omitempty"`
	MatchedPath    []string `json:"matched_path,omitempty"`
	CategoryID     int      `json:"category_id,omitempty"`
	CategoryIDList []int    `json:"category_id_list,omitempty"`
	ProductTypeID  int      `json:"product_type_id,omitempty"`
	TopCategoryID  int      `json:"top_category_id,omitempty"`
	ReviewNotes    []string `json:"review_notes,omitempty"`
}

type SheinResolvedAttribute struct {
	Name                string `json:"name,omitempty"`
	Value               string `json:"value,omitempty"`
	AttributeID         int    `json:"attribute_id,omitempty"`
	AttributeValueID    *int   `json:"attribute_value_id,omitempty"`
	AttributeExtraValue string `json:"attribute_extra_value,omitempty"`
	MatchedBy           string `json:"matched_by,omitempty"`
	Required            bool   `json:"required,omitempty"`
	SKCScope            bool   `json:"skc_scope,omitempty"`
}

type SheinAttributeResolution struct {
	Status             string                   `json:"status,omitempty"`
	Source             string                   `json:"source,omitempty"`
	CategoryID         int                      `json:"category_id,omitempty"`
	TemplateCount      int                      `json:"template_count,omitempty"`
	ResolvedCount      int                      `json:"resolved_count,omitempty"`
	UnresolvedCount    int                      `json:"unresolved_count,omitempty"`
	ResolvedAttributes []SheinResolvedAttribute `json:"resolved_attributes,omitempty"`
	ReviewNotes        []string                 `json:"review_notes,omitempty"`
}

type SheinResolvedSaleAttribute struct {
	Scope            string `json:"scope,omitempty"`
	Name             string `json:"name,omitempty"`
	Value            string `json:"value,omitempty"`
	AttributeID      int    `json:"attribute_id,omitempty"`
	AttributeValueID *int   `json:"attribute_value_id,omitempty"`
	MatchedBy        string `json:"matched_by,omitempty"`
}

type SheinSaleAttributeResolution struct {
	Status               string                            `json:"status,omitempty"`
	Source               string                            `json:"source,omitempty"`
	CategoryID           int                               `json:"category_id,omitempty"`
	PrimaryAttributeID   int                               `json:"primary_attribute_id,omitempty"`
	SecondaryAttributeID int                               `json:"secondary_attribute_id,omitempty"`
	SKCAttributes        []SheinResolvedSaleAttribute      `json:"skc_attributes,omitempty"`
	SKUAttributes        []SheinResolvedSaleAttribute      `json:"sku_attributes,omitempty"`
	Candidates           []SheinSaleAttributeCandidateInfo `json:"candidates,omitempty"`
	SelectionSummary     []string                          `json:"selection_summary,omitempty"`
	ReviewNotes          []string                          `json:"review_notes,omitempty"`
}

type SheinSaleAttributeCandidateInfo struct {
	Name           string   `json:"name,omitempty"`
	AttributeID    int      `json:"attribute_id,omitempty"`
	SKCScope       bool     `json:"skc_scope,omitempty"`
	Required       bool     `json:"required,omitempty"`
	SKCDistinct    int      `json:"skc_distinct,omitempty"`
	SKUDistinct    int      `json:"sku_distinct,omitempty"`
	TotalDistinct  int      `json:"total_distinct,omitempty"`
	PrimaryScore   int      `json:"primary_score,omitempty"`
	SecondaryScore int      `json:"secondary_score,omitempty"`
	SampleValue    string   `json:"sample_value,omitempty"`
	Reasons        []string `json:"reasons,omitempty"`
	SelectedScope  string   `json:"selected_scope,omitempty"`
}

type SheinInspection struct {
	NeedsReview bool                     `json:"needs_review"`
	Summary     []string                 `json:"summary,omitempty"`
	Sections    []SheinInspectionSection `json:"sections,omitempty"`
}

type SheinInspectionSection struct {
	Key         string                  `json:"key,omitempty"`
	Title       string                  `json:"title,omitempty"`
	Status      string                  `json:"status,omitempty"`
	Summary     string                  `json:"summary,omitempty"`
	Highlights  []string                `json:"highlights,omitempty"`
	ActionItems []string                `json:"action_items,omitempty"`
	Actions     []SheinInspectionAction `json:"actions,omitempty"`
}

type SheinInspectionCategoryPayload struct {
	Platform       string   `json:"platform,omitempty"`
	Target         string   `json:"target,omitempty"`
	Status         string   `json:"status,omitempty"`
	Source         string   `json:"source,omitempty"`
	CategoryName   string   `json:"category_name,omitempty"`
	CategoryPath   []string `json:"category_path,omitempty"`
	CategoryID     int      `json:"category_id,omitempty"`
	CategoryIDList []int    `json:"category_id_list,omitempty"`
	ProductTypeID  *int     `json:"product_type_id,omitempty"`
	TopCategoryID  int      `json:"top_category_id,omitempty"`
	ReviewNotes    []string `json:"review_notes,omitempty"`
}

type SheinInspectionAttributePayload struct {
	Platform           string                   `json:"platform,omitempty"`
	Target             string                   `json:"target,omitempty"`
	Status             string                   `json:"status,omitempty"`
	Source             string                   `json:"source,omitempty"`
	TemplateCount      int                      `json:"template_count,omitempty"`
	ResolvedCount      int                      `json:"resolved_count,omitempty"`
	UnresolvedCount    int                      `json:"unresolved_count,omitempty"`
	ProductAttributes  []PlatformAttribute      `json:"product_attributes,omitempty"`
	ResolvedAttributes []SheinResolvedAttribute `json:"resolved_attributes,omitempty"`
	PendingAttributes  []PlatformAttribute      `json:"pending_attributes,omitempty"`
	ReviewNotes        []string                 `json:"review_notes,omitempty"`
}

type SheinInspectionSaleAttributePayload struct {
	Platform             string                            `json:"platform,omitempty"`
	Target               string                            `json:"target,omitempty"`
	Status               string                            `json:"status,omitempty"`
	Source               string                            `json:"source,omitempty"`
	PrimaryAttributeID   int                               `json:"primary_attribute_id,omitempty"`
	SecondaryAttributeID int                               `json:"secondary_attribute_id,omitempty"`
	SelectionSummary     []string                          `json:"selection_summary,omitempty"`
	SKCAttributes        []SheinResolvedSaleAttribute      `json:"skc_attributes,omitempty"`
	SKUAttributes        []SheinResolvedSaleAttribute      `json:"sku_attributes,omitempty"`
	CandidateCount       int                               `json:"candidate_count,omitempty"`
	Candidates           []SheinSaleAttributeCandidateInfo `json:"candidates,omitempty"`
	SKCPatches           []SheinInspectionSKCPatchPayload  `json:"skc_patches,omitempty"`
	ReviewNotes          []string                          `json:"review_notes,omitempty"`
}

type SheinInspectionSKCPatchPayload struct {
	SupplierCode  string                           `json:"supplier_code,omitempty"`
	SkcName       string                           `json:"skc_name,omitempty"`
	SaleName      string                           `json:"sale_name,omitempty"`
	MainImageURL  string                           `json:"main_image_url,omitempty"`
	SaleAttribute *SheinResolvedSaleAttribute      `json:"sale_attribute,omitempty"`
	SKUPatches    []SheinInspectionSKUPatchPayload `json:"sku_patches,omitempty"`
}

type SheinInspectionSKUPatchPayload struct {
	SupplierSKU    string                       `json:"supplier_sku,omitempty"`
	Attributes     map[string]string            `json:"attributes,omitempty"`
	BasePrice      string                       `json:"base_price,omitempty"`
	CostPrice      string                       `json:"cost_price,omitempty"`
	Currency       string                       `json:"currency,omitempty"`
	StockCount     int                          `json:"stock_count,omitempty"`
	MainImage      string                       `json:"main_image,omitempty"`
	Barcode        string                       `json:"barcode,omitempty"`
	SaleAttributes []SheinResolvedSaleAttribute `json:"sale_attributes,omitempty"`
	SitePriceList  []SheinSitePrice             `json:"site_price_list,omitempty"`
	StockInfoList  []SheinStockInfo             `json:"stock_info_list,omitempty"`
}

type TemuSKCPackage struct {
	Priority        int               `json:"priority,omitempty"`
	ColorImageURL   string            `json:"color_image_url,omitempty"`
	Spec            []TemuSpecPackage `json:"spec,omitempty"`
	CarouselGallery []string          `json:"carousel_gallery,omitempty"`
	SKUs            []PlatformVariant `json:"skus,omitempty"`
}

type TemuSpecPackage struct {
	Name       string `json:"name,omitempty"`
	Value      string `json:"value,omitempty"`
	ParentName string `json:"parent_name,omitempty"`
}

type TemuBatchSKUInfo struct {
	Currency  string `json:"currency,omitempty"`
	Quantity  string `json:"quantity,omitempty"`
	OutSkuSN  string `json:"out_sku_sn,omitempty"`
	Weight    string `json:"weight,omitempty"`
	Length    string `json:"length,omitempty"`
	Width     string `json:"width,omitempty"`
	Height    string `json:"height,omitempty"`
	Price     string `json:"price,omitempty"`
	CostPrice string `json:"cost_price,omitempty"`
}

func (r GenerateRequest) Value() (driver.Value, error) { return json.Marshal(r) }

func (r *GenerateRequest) Scan(value any) error {
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, r)
}

func (r ListingKitResult) Value() (driver.Value, error) { return json.Marshal(r) }

func (r *ListingKitResult) Scan(value any) error {
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, r)
}
