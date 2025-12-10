// Package product SKC和SKU数据结构
package product

// SKC 库存单位
type SKC struct {
	FromPreProduct          *string               `json:"from_pre_product"`
	SaleAttribute           SaleAttribute         `json:"sale_attribute"`
	SKUS                    []SKU                 `json:"sku_list"`
	ImageInfo               ImageInfo             `json:"image_info,omitempty"`
	IsMaternalStyle         *bool                 `json:"is_maternal_style"`
	MultiLanguageName       LanguageContent       `json:"multi_language_name"`
	MultiLanguageNameList   []LanguageContent     `json:"multi_language_name_list"`
	SiteDetailImageInfoList []SiteDetailImageInfo `json:"site_detail_image_info_list"`
	SppRelateSkcName        *string               `json:"spp_relate_skc_name"`
	HopeOnSaleDate          string                `json:"hope_on_sale_date"`
	ShelfWay                int                   `json:"shelf_way"`
	ShelfRequire            int                   `json:"shelf_require"`
	ProofOfStockList        []interface{}         `json:"proof_of_stock_list"`
	Sort                    int                   `json:"sort"`
	PartCodeRelList         *interface{}          `json:"part_code_rel_list"`
	PriceFillingTips        *string               `json:"price_filling_tips"`
	SKCMaterial             *interface{}          `json:"skc_material"`
	SiteSpecImageInfoList   []interface{}         `json:"site_spec_image_info_list"`
	SKCScopeAttributeList   []interface{}         `json:"skc_scope_attribute_list"`
	SampleInfo              *interface{}          `json:"sample_info"`
	IsFirstPublish          bool                  `json:"is_first_publish"`
	QualitySN               *string               `json:"quality_sn"`
	ReQuoteFileList         *[]interface{}        `json:"re_quote_file_list"`
	ReQuoteReason           *string               `json:"re_quote_reason"`
	SupplierCode            *string               `json:"supplier_code"`
	Extra                   SkcExtra              `json:"extra"`
}

// SkcExtra SKC扩展信息
type SkcExtra struct {
	IsSourceEmbryo             bool `json:"isSourceEmbryo"`
	IsEmbryoInclude            bool `json:"isEmbryoInclude"`
	PhysicalPriceReviewSuccess bool `json:"physical_price_review_success"`
}

// SaleAttribute 销售属性
type SaleAttribute struct {
	AttributeID        int  `json:"attribute_id"`
	AttributeValueID   int  `json:"attribute_value_id"`
	PreFillSpec        bool `json:"pre_fill_spec"`
	IsSPPSaleAttribute bool `json:"is_spp_sale_attribute"`
}

// SKU 具体规格
type SKU struct {
	SaleAttributeList        []SaleAttribute `json:"sale_attribute_list"`
	SppRelateSkuCode         *string         `json:"spp_relate_sku_code"`
	CostInfo                 *CostInfo       `json:"cost_info"`
	Height                   string          `json:"height"`
	Length                   string          `json:"length"`
	Width                    string          `json:"width"`
	LengthUnit               string          `json:"length_unit"`
	StockInfoList            []StockInfo     `json:"stock_info_list"`
	Weight                   float64         `json:"weight"`
	WeightUnit               string          `json:"weight_unit"`
	StockCount               *int            `json:"stock_count"`
	SupplierSKU              string          `json:"supplier_sku"`
	StopPurchase             int             `json:"stop_purchase"`
	MallState                int             `json:"mall_state"`
	PriceInfoList            []PriceInfo     `json:"price_info_list"`
	ImageInfo                *ImageInfo      `json:"image_info"`
	CompetingProductLink     *any            `json:"competing_product_link"`
	CompetingCostPriceImages []any           `json:"competing_cost_price_images"`
	SuggestedRetailPrice     *any            `json:"suggested_retail_price"`
	QuantityInfo             *QuantityInfo   `json:"quantity_info"`
	Extra                    SkuExtra        `json:"extra"`
}

// SkuExtra SKU扩展信息
type SkuExtra struct {
	FieldDisabledInfo FieldDisabledInfo `json:"fieldDisabledInfo"`
}

// FieldDisabledInfo 字段禁用信息
type FieldDisabledInfo struct {
	SkuQuantity     *int `json:"skuQuantity"`
	SkuQuantityType *int `json:"skuQuantityType"`
	SkuQuantityUnit *int `json:"skuQuantityUnit"`
}

// QuantityInfo 数量信息
type QuantityInfo struct {
	Quantity     *int `json:"quantity"`
	QuantityType *int `json:"quantity_type"`
	QuantityUnit *int `json:"quantity_unit"`
}

// CostInfo 成本信息
type CostInfo struct {
	CostPrice string `json:"cost_price"`
	Currency  string `json:"currency"`
}

// PriceInfo 价格信息
type PriceInfo struct {
	SubSite      string   `json:"sub_site"`
	BasePrice    float64  `json:"base_price"`
	SpecialPrice *float64 `json:"special_price"`
	Currency     string   `json:"currency"`
}

// StockInfo 库存信息
type StockInfo struct {
	InventoryNum          int    `json:"inventory_num"`
	MerchantWarehouseCode string `json:"merchant_warehouse_code"`
}

// ImageInfo 图片信息
type ImageInfo struct {
	ImageGroupCode        *string        `json:"image_group_code"`
	ImageInfoList         []ImageDetail  `json:"image_info_list,omitempty"`
	OriginalImageInfoList *[]interface{} `json:"original_image_info_list,omitempty"`
}

// ImageDetail 图片详情
type ImageDetail struct {
	ImageType             int      `json:"image_type"`
	ImageSort             int      `json:"image_sort"`
	ImageURL              string   `json:"image_url"`
	ImageItemID           *string  `json:"image_item_id"`
	SizeImgFlag           bool     `json:"size_img_flag"`
	TransformCVSizeImage  bool     `json:"transformCvSizeImage"`
	AISStatus             int      `json:"ai_status"`
	PSTypes               []string `json:"ps_types"`
	MarketingMainImage    bool     `json:"marketing_main_image"`
	CommodityCategoryFlag *string  `json:"commodity_category_flag"`
}

// SiteDetailImageInfo 站点详情图片信息
type SiteDetailImageInfo struct {
	SiteAbbrList   []string      `json:"site_abbr_list"`
	ImageGroupCode *string       `json:"image_group_code"`
	ImageInfoList  []DetailImage `json:"image_info_list"`
}

// DetailImage 详情图片
type DetailImage struct {
	ImageSort   int     `json:"image_sort"`
	ImageURL    string  `json:"image_url"`
	ImageItemID *string `json:"image_item_id"`
}
