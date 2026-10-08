package model

type ApplicationMode string

const (
	ModeSelfOperated ApplicationMode = "self_operated"
	ModeSemiManaged  ApplicationMode = "semi_managed"
	ModeFullyManaged ApplicationMode = "fully_managed"
)

func (m ApplicationMode) Valid() bool {
	return m == ModeSelfOperated || m == ModeSemiManaged || m == ModeFullyManaged
}

// These DTOs follow the current official OpenAPI, excluding browser-backend
// fields and platform-generated identifiers from new-product requests.
type LanguageContent struct {
	Language string `json:"language"`
	Name     string `json:"name"`
}
type AttributeValue struct {
	AttributeID      int64  `json:"attribute_id"`
	AttributeValueID *int64 `json:"attribute_value_id,omitempty"`
	ExtraValue       string `json:"attribute_extra_value,omitempty"`
	CustomValue      string `json:"custom_attribute_value,omitempty"`
	Language         string `json:"language,omitempty"`
}
type ImageInfo struct {
	Images []ProductImage `json:"image_info_list"`
}
type ProductImage struct {
	Sort int    `json:"image_sort"`
	Type int    `json:"image_type"`
	URL  string `json:"image_url"`
}
type SiteSelection struct {
	MainSite string   `json:"main_site"`
	SubSites []string `json:"sub_site_list"`
}
type ProductPrice struct {
	BasePrice float64 `json:"base_price"`
	Currency  string  `json:"currency"`
	SubSite   string  `json:"sub_site"`
}
type ProductStock struct {
	Quantity      int    `json:"inventory_num"`
	WarehouseID   string `json:"supplier_warehouse_id,omitempty"`
	WarehouseName string `json:"supplier_warehouse_name,omitempty"`
}
type QuantityInfo struct {
	Type     int `json:"quantity_type"`
	Unit     int `json:"quantity_unit"`
	Quantity int `json:"quantity"`
}
type ProductSKU struct {
	SupplierSKU          string           `json:"supplier_sku"`
	Length               string           `json:"length,omitempty"`
	Width                string           `json:"width,omitempty"`
	Height               string           `json:"height,omitempty"`
	Weight               *float64         `json:"weight,omitempty"`
	WeightUnit           string           `json:"weight_unit,omitempty"`
	DimensionUnit        string           `json:"length_width_height_unit,omitempty"`
	MallState            int              `json:"mall_state"`
	Prices               []ProductPrice   `json:"price_info_list,omitempty"`
	Stock                []ProductStock   `json:"stock_info_list"`
	SaleAttributes       []AttributeValue `json:"sale_attribute_list"`
	Attributes           []AttributeValue `json:"sku_scope_attribute_list,omitempty"`
	ImageInfo            *ImageInfo       `json:"image_info,omitempty"`
	Quantity             *QuantityInfo    `json:"quantity_info,omitempty"`
	PackageType          *string          `json:"package_type,omitempty"`
	CompetingProductLink string           `json:"competing_product_link,omitempty"`
	Cost                 *CostPrice       `json:"cost_info,omitempty"`
	MinimumStockQuantity string           `json:"minimum_stock_quantity,omitempty"`
	StopPurchase         *int             `json:"stop_purchase,omitempty"`
}
type ProductSKC struct {
	SupplierCode         string             `json:"supplier_code,omitempty"`
	SaleAttribute        AttributeValue     `json:"sale_attribute"`
	ImageInfo            ImageInfo          `json:"image_info"`
	Names                []LanguageContent  `json:"skc_multi_language_name_list,omitempty"`
	ShelfWay             string             `json:"shelf_way,omitempty"`
	ShelfRequire         string             `json:"shelf_require,omitempty"`
	HopeOnSaleDate       string             `json:"hope_on_sale_date,omitempty"`
	SuggestedRetailPrice *RetailPrice       `json:"suggested_retail_price,omitempty"`
	SiteDetailImages     []SiteDetailImages `json:"site_detail_image_info_list,omitempty"`
	StockProofs          []StockProof       `json:"proof_of_stock_list,omitempty"`
	SKUs                 []ProductSKU       `json:"sku_list"`
}
type PublishProduct struct {
	CategoryID        int64             `json:"category_id"`
	ProductTypeID     int64             `json:"product_type_id"`
	BrandCode         string            `json:"brand_code,omitempty"`
	SourceSystem      string            `json:"source_system"`
	SuitFlag          string            `json:"suit_flag"`
	IsSPUPic          bool              `json:"is_spu_pic"`
	SupplierCode      string            `json:"supplier_code,omitempty"`
	Names             []LanguageContent `json:"multi_language_name_list"`
	Descriptions      []LanguageContent `json:"multi_language_desc_list,omitempty"`
	Attributes        []AttributeValue  `json:"product_attribute_list"`
	Sites             []SiteSelection   `json:"site_list,omitempty"`
	SKCs              []ProductSKC      `json:"skc_list"`
	ImageInfo         *ImageInfo        `json:"image_info,omitempty"`
	SizeAttributes    []SizeAttribute   `json:"size_attribute_list,omitempty"`
	Sample            *SampleInfo       `json:"sample_info,omitempty"`
	FillConfiguration *struct {
		FilledQuantityToSKU bool `json:"filled_quantity_to_sku"`
	} `json:"fill_configuration_info,omitempty"`
	FillConfigurationTags []string `json:"fill_configuration_tags,omitempty"`
}
type CostPrice struct {
	Price    string `json:"cost_price"`
	Currency string `json:"currency"`
}
type RetailPrice struct {
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
}
type SizeAttribute struct {
	AttributeID                 int64  `json:"attribute_id"`
	ExtraValue                  string `json:"attribute_extra_value"`
	RelatedSaleAttributeID      *int64 `json:"relate_sale_attribute_id,omitempty"`
	RelatedSaleAttributeValueID *int64 `json:"relate_sale_attribute_value_id,omitempty"`
}
type SiteDetailImages struct {
	Sites  []string      `json:"site_abbr_list"`
	Images []DetailImage `json:"image_info_list"`
}
type DetailImage struct {
	Sort int    `json:"image_sort"`
	URL  string `json:"image_url"`
}
type StockProof struct {
	Filename string `json:"file_name"`
	Type     string `json:"type"`
	URL      string `json:"url"`
}
type SampleInfo struct {
	Spec        SampleSpec `json:"sample_spec"`
	JudgeType   int        `json:"sample_judge_type"`
	ReserveFlag int        `json:"reserve_sample_flag"`
	SpotFlag    int        `json:"spot_flag"`
}
type SampleSpec struct {
	Main SampleMainSpec  `json:"main_spec"`
	Sub  []SampleSubSpec `json:"sub_spec_list"`
}
type SampleMainSpec struct {
	AttributeID int64 `json:"attribute_id"`
	ValueID     int64 `json:"attribute_value_id"`
}
type SampleSubSpec struct {
	AttributeID string `json:"attribute_id"`
	ValueID     string `json:"attribute_value_id"`
}

type Site struct {
	Abbreviation string `json:"site_abbr"`
	Name         string `json:"site_name"`
	Status       *int   `json:"site_status"`
	StoreType    *int   `json:"store_type"`
	Currency     string `json:"currency"`
}
type MainSite struct {
	ID    string `json:"main_site"`
	Name  string `json:"main_site_name"`
	Sites []Site `json:"sub_site_list"`
}

type Warehouse struct {
	Code          string   `json:"warehouseCode"`
	Name          string   `json:"warehouseName"`
	SaleCountries []string `json:"saleCountryList"`
	Type          int      `json:"warehouseType"`
}
type FillRule struct {
	Field    string `json:"field_key"`
	Module   string `json:"module"`
	Required *bool  `json:"required"`
	Show     *bool  `json:"show"`
}
type PictureRule struct {
	Field   string `json:"field_key"`
	Enabled *bool  `json:"is_true"`
}
type TitleLimit struct {
	Language string  `json:"language"`
	Maximum  float64 `json:"max_length"`
}
type FillStandards struct {
	Fields                             []FillRule    `json:"fill_in_standard_list"`
	Currency                           *string       `json:"currency"`
	DefaultLanguage                    string        `json:"default_language"`
	DefaultTitleMaximum                *int          `json:"default_language_title_max_length"`
	TitleLimits                        []TitleLimit  `json:"language_title_max_length_list"`
	Pictures                           []PictureRule `json:"picture_config_list"`
	SupplierCodeInSPU                  *bool         `json:"supplier_code_in_spu_dimension"`
	SupportDuplicateSaleAttributeValue *bool         `json:"support_duplicate_sale_attr_value"`
}

type PublishedSKU struct {
	SKUCode     string `json:"sku_code"`
	SupplierSKU string `json:"supplier_sku"`
}
type PublishedSKC struct {
	SKCName string         `json:"skc_name"`
	SKUs    []PublishedSKU `json:"sku_list"`
}
type PublishResult struct {
	SPUName      string         `json:"spu_name"`
	SKCs         []PublishedSKC `json:"skc_list"`
	Version      string         `json:"version,omitempty"`
	TraceID      string         `json:"traceId,omitempty"`
	ResponseHash string         `json:"-"`
}
type TransformImage struct {
	OriginalURL string `json:"original_url"`
	Type        int    `json:"image_type"`
}
type TransformedImage struct {
	Original      string `json:"original"`
	Transformed   string `json:"transformed"`
	FailureReason string `json:"failure_reason"`
	ResponseHash  string `json:"-"`
}
