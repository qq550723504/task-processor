package types

import "encoding/json"

// Product TEMU商品提交请求结构体
type Product struct {
	CanSave                *bool          `json:"can_save"`
	Extra                  ExtraInfo      `json:"extra"`
	GoodsBasic             GoodsBasicInfo `json:"goods_basic"`
	GoodsSaleInfo          GoodsSaleInfo  `json:"goods_sale_info"`
	GoodsServicePromise    ServicePromise `json:"goods_service_promise"`
	GoodsExtensionInfo     ExtensionInfo  `json:"goods_extension_info"`
	PlatformExpressBill    *bool          `json:"platform_express_bill"`
	SupportMaxRetailPrice  *bool          `json:"support_max_retail_price"`
	ReplicateToRelateGoods *bool          `json:"replicate_to_relate_goods"`
	SkcList                []Skc          `json:"skc_list"`
	BatchSkuInfo           BatchSkuInfo   `json:"batch_sku_info"`
}

type Skc struct {
	SkuList []Sku `json:"sku_list"`
	// 以下字段在提交时可选，主要用于内部管理
	CarouselGallery  []ImageInfo `json:"carousel_gallery,omitempty"`
	ColorImageUrl    string      `json:"color_image_url,omitempty"`
	CommitDeleteType int         `json:"commit_delete_type,omitempty"`
	CommitDeleted    int         `json:"commit_deleted,omitempty"`
	Priority         int         `json:"priority,omitempty"`
	SkcComplete      bool        `json:"skc_complete,omitempty"`
	Spec             []SpecInfo  `json:"spec,omitempty"`
}

type Sku struct {
	// 必需字段
	Spec                     []SpecInfo         `json:"spec"`
	Currency                 string             `json:"currency"`
	UseEstimateSupplierPrice bool               `json:"use_estimate_supplier_price"`
	DimensionGallery         []ImageInfo        `json:"dimension_gallery"`
	CarouselGallery          []ImageInfo        `json:"carousel_gallery"`
	FoodIngredientGallery    []ImageInfo        `json:"food_ingredient_gallery"`
	Quantity                 string             `json:"quantity"`
	ProductExpressInfo       ProductExpressInfo `json:"product_express_info"`
	SupplierPriceStr         string             `json:"supplier_price_str"`
	OutSkuSN                 string             `json:"out_sku_sn"`
	MultiplePackage          MultiplePackage    `json:"multiple_package"`
	OriginNetContentNumber   string             `json:"origin_net_content_number,omitempty"`
	NetContentUnitCode       int                `json:"net_content_unit_code,omitempty"`
	MaxRetailPriceStr        string             `json:"max_retail_price_str"`
	SupplierPrice            int                `json:"supplier_price"`
	SkuPriceDocuments        map[string]any     `json:"sku_price_documents"`

	// 可选字段（用于内部管理）
	MarketPrice         int    `json:"market_price,omitempty"`     // 市场价（分），供货价*2
	MarketPriceStr      string `json:"market_price_str,omitempty"` // 市场价字符串（元），供货价*2
	MaxRetailPrice      int    `json:"max_retail_price,omitempty"`
	Price               int    `json:"price,omitempty"`
	PriceStr            string `json:"price_str,omitempty"`
	Priority            int    `json:"priority,omitempty"`
	RetailPriceCurrency string `json:"retail_price_currency,omitempty"`
	SkuComplete         bool   `json:"sku_complete,omitempty"`
	SkuDeleted          bool   `json:"sku_deleted,omitempty"`
}

// SpecInfo 规格信息
type SpecInfo struct {
	SpecID         string `json:"spec_id"`
	SpecName       string `json:"spec_name"`
	ParentSpecID   string `json:"parent_spec_id"`
	ParentSpecName string `json:"parent_spec_name,omitempty"`
	ParentID       string `json:"parent_id,omitempty"`
}

type SkuPriceDocument struct {
	Category1FileList []string `json:"category1_file_list"`
	Category2FileList []string `json:"category2_file_list"`
}

// ProductExpressInfo 产品物流信息
type ProductExpressInfo struct {
	WeightInfo WeightInfo `json:"weight_info"`
	VolumeInfo VolumeInfo `json:"volume_info"`
}

type VolumeInfo struct {
	Height string `json:"height"`
	Length string `json:"length"`
	//Unit   string `json:"unit"`
	Width string `json:"width"`
}

// WeightInfo 重量信息
type WeightInfo struct {
	Weight string `json:"weight"`
	//Unit   string `json:"unit"`
}

// GoodsBasicInfo 商品基本信息
type GoodsBasicInfo struct {
	GoodsID                 string       `json:"goods_id"`
	ListingCommitID         string       `json:"listing_commit_id"`
	ListingCommitVersion    string       `json:"listing_commit_version"`
	GoodsName               string       `json:"goods_name"`
	GoodsCreateTime         int64        `json:"goods_create_time"`
	GoodsCommitID           string       `json:"goods_commit_id"`
	Lang                    string       `json:"lang"`
	AllowSite               []int        `json:"allow_site"`
	CatID                   int          `json:"cat_id"`
	CatIDs                  []int        `json:"cat_ids"`
	CategoryTree            CategoryTree `json:"category_tree"`
	CategoryDisclaimer      Disclaimer   `json:"category_disclaimer"`
	GoodsType               int          `json:"goods_type"`
	HdThumbURL              string       `json:"hd_thumb_url"`
	GoodsGallery            GoodsGallery `json:"goods_gallery,omitempty"`
	IsOnSale                int          `json:"is_on_sale"`
	CatType                 int          `json:"cat_type"`
	IsClothes               bool         `json:"is_clothes"`
	IsBooks                 bool         `json:"is_books"`
	CanSkipRequiredProperty bool         `json:"can_skip_required_property"`
	IsShop                  bool         `json:"is_shop"`
	FromCopy                bool         `json:"from_copy"`
	HasSubmitted            bool         `json:"has_submitted"`
	Source                  int          `json:"source"`
	OutGoodsSN              string       `json:"out_goods_sn"`
	ListPriceRequired       bool         `json:"list_price_required"`
	ListPriceDocuments      bool         `json:"list_price_documents"`
	NeedAccessoryInfo       bool         `json:"need_accessory_info"`
	AccessoryInfoRequired   bool         `json:"accessory_info_required"`
	Customized              bool         `json:"customized"`
	SecondHand              bool         `json:"second_hand"`
	SupportCustomizedGoods  bool         `json:"support_customized_goods"`
	RecommendURLPrice       bool         `json:"recommend_url_price"`
	AgreeMaxRetailPrice     bool         `json:"agree_max_retail_price"`
	CanEditSecondHand       bool         `json:"can_edit_second_hand"`
	MadeToOrder             bool         `json:"made_to_order"`
}

// GoodsSaleInfo 商品销售信息
type GoodsSaleInfo struct {
	GoodsPattern int `json:"goods_pattern"`
}

// ServicePromise 服务承诺
type ServicePromise struct {
	ShipmentLimitSecond int    `json:"shipment_limit_second"`
	FulfillmentType     int    `json:"fulfillment_type"`
	CostTemplateID      string `json:"cost_template_id"`
}

// ExtensionInfo 扩展信息
type ExtensionInfo struct {
	GoodsProperty             GoodsProperty       `json:"goods_property"`
	CertificationInfo         CertificationInfo   `json:"certification_info"`
	GoodsTrademark            GoodsTrademark      `json:"goods_trademark,omitempty"`
	GoodsProductTaxCodeDetail interface{}         `json:"goods_product_tax_code_detail,omitempty"`
	GoodsOriginInfo           GoodsOriginInfo     `json:"goods_origin_info"`
	GoodsDesc                 string              `json:"goods_desc,omitempty"`
	BulletPoints              []string            `json:"bullet_points,omitempty"`
	SecondHand                interface{}         `json:"second_hand,omitempty"`
	GuideFileInfo             interface{}         `json:"guide_file_info,omitempty"`
	SizeChartImageInfo        *SizeChartImageInfo `json:"size_chart_image_info,omitempty"`
}

type SizeChartImageInfo struct {
	SizeChartImageList []ImageInfo `json:"size_chart_image_list"`
}

// MarshalJSON 实现自定义JSON序列化
func (s *SizeChartImageInfo) MarshalJSON() ([]byte, error) {
	// 如果指针为 nil 或 SizeChartImageList 为空，返回 null
	if s == nil || len(s.SizeChartImageList) == 0 {
		return []byte("null"), nil
	}

	// 否则使用标准序列化
	type Alias SizeChartImageInfo
	return json.Marshal((*Alias)(s))
}

// CategoryTree 分类树
type CategoryTree struct {
	Level        int      `json:"level"`
	CateType     int      `json:"cate_type"`
	CatID        int      `json:"cat_id"`
	Cate1ID      int      `json:"cate1_id"`
	Cate1Name    string   `json:"cate1_name"`
	Cate2ID      int      `json:"cate2_id"`
	Cate2Name    string   `json:"cate2_name"`
	Cate3ID      int      `json:"cate3_id"`
	Cate3Name    string   `json:"cate3_name"`
	Cate4ID      int      `json:"cate4_id"`
	Cate4Name    string   `json:"cate4_name"`
	Cate5ID      int      `json:"cate5_id,omitempty"`
	Cate5Name    string   `json:"cate5_name,omitempty"`
	CateNameList []string `json:"cate_name_list"`
}

// Disclaimer 免责声明
type Disclaimer struct {
	PromptList []string `json:"prompt_list"`
}

// GoodsGallery 商品图库
type GoodsGallery struct {
	DetailImage   []ImageInfo   `json:"detail_image"`
	CarouselVideo []interface{} `json:"carousel_video"`
	DetailVideo   []interface{} `json:"detail_video"`
}

// MarshalJSON 实现自定义JSON序列化
func (g GoodsGallery) MarshalJSON() ([]byte, error) {
	// 检查所有字段是否都为空
	if len(g.DetailImage) == 0 &&
		len(g.CarouselVideo) == 0 &&
		len(g.DetailVideo) == 0 {
		// 如果所有字段都为空，返回空对象
		return []byte("{}"), nil
	}

	// 否则使用标准序列化
	type Alias GoodsGallery
	return json.Marshal(Alias(g))
}

// GoodsSpecProperty 商品规格属性
type GoodsSpecProperty struct {
	Value          string `json:"value"`
	SpecID         string `json:"spec_id"`
	ParentSpecID   string `json:"parent_spec_id"`
	ParentSpecName string `json:"parent_spec_name"`
	Feature        int    `json:"feature,omitempty"`
	Checked        bool   `json:"checked"`
	ControlType    int    `json:"control_type"`
	Disabled       bool   `json:"disabled"`
	Name           string `json:"name"`
	IsCustomized   int    `json:"is_customized"`
}

// GoodsProperty 商品属性
type GoodsProperty struct {
	GoodsBrandProperties []interface{}       `json:"goods_brand_properties"`
	GoodsProperties      []PropertyItem      `json:"goods_properties"`
	GoodsSpecProperties  []GoodsSpecProperty `json:"goods_spec_properties"`
}

// MarshalJSON 实现自定义JSON序列化
func (g GoodsProperty) MarshalJSON() ([]byte, error) {
	result := make(map[string]interface{})

	// 只添加非空的字段
	if len(g.GoodsBrandProperties) > 0 {
		result["goods_brand_properties"] = g.GoodsBrandProperties
	}

	if len(g.GoodsProperties) > 0 {
		result["goods_properties"] = g.GoodsProperties
	}

	// GoodsSpecProperties 总是包含
	result["goods_spec_properties"] = g.GoodsSpecProperties

	return json.Marshal(result)
}

// PropertyItem 属性项
type PropertyItem struct {
	RefPid           int    `json:"ref_pid"`
	Pid              int    `json:"pid"`
	TemplatePid      int    `json:"template_pid"`
	Value            string `json:"value"`
	Vid              int    `json:"vid"`
	ValueUnit        string `json:"value_unit,omitempty"`
	TemplateModuleID int    `json:"template_module_id,omitempty"`
	NumberInputValue string `json:"number_input_value,omitempty"`
}

// CertificationInfo 认证信息
type CertificationInfo struct {
	CertificateInfo map[string]any `json:"certificate_info"`
	ExtraTemplate   ExtraTemplate  `json:"extra_template"`
	ActualPhoto     ActualPhoto    `json:"actual_photo,omitempty"`
	GpsrInfo        GpsrInfo       `json:"gpsr_info,omitempty"`
	RepInfo         RepInfo        `json:"rep_info,omitempty"`
}

// MarshalJSON 实现自定义JSON序列化
func (c CertificationInfo) MarshalJSON() ([]byte, error) {
	result := make(map[string]interface{})

	// 只添加非空的字段
	if len(c.CertificateInfo) > 0 {
		result["certificate_info"] = c.CertificateInfo
	}

	// ExtraTemplate 总是包含（因为它是必需的）
	result["extra_template"] = c.ExtraTemplate

	// 检查 ActualPhoto 是否为空
	if !c.isActualPhotoEmpty() {
		result["actual_photo"] = c.ActualPhoto
	}

	// 检查 GpsrInfo 是否为空
	if !c.isGpsrInfoEmpty() {
		result["gpsr_info"] = c.GpsrInfo
	}

	// 检查 RepInfo 是否为空
	if !c.isRepInfoEmpty() {
		result["rep_info"] = c.RepInfo
	}

	return json.Marshal(result)
}

// isActualPhotoEmpty 检查 ActualPhoto 是否为空
func (c CertificationInfo) isActualPhotoEmpty() bool {
	return len(c.ActualPhoto.Ext) == 0 &&
		len(c.ActualPhoto.ActualPhotoInfoList) == 0
}

// isGpsrInfoEmpty 检查 GpsrInfo 是否为空
func (c CertificationInfo) isGpsrInfoEmpty() bool {
	return len(c.GpsrInfo.Ext) == 0
}

// isRepInfoEmpty 检查 RepInfo 是否为空
func (c CertificationInfo) isRepInfoEmpty() bool {
	return len(c.RepInfo.Ext) == 0
}

// ExtraTemplate 额外模板
type ExtraTemplate struct {
	ExtraTemplateDetailList []ExtraTemplateDetail `json:"extra_template_detail_list"`
}

type ExtraTemplateDetail struct {
	TemplateID int              `json:"template_id"`
	Properties map[string][]int `json:"properties"`
	InputText  map[string]any   `json:"input_text"`
}

// ExtraInfo 额外信息
type ExtraInfo struct {
	CreateEmptyGoods bool        `json:"create_empty_goods"`
	VersionType      interface{} `json:"version_type"`
	Tab              int         `json:"tab"`
	MinSkuImageSize  int         `json:"min_sku_image_size"`
	MaxSkuImageSize  int         `json:"max_sku_image_size"`
	CurrentTab       int         `json:"current_tab"`
}

// ImageInfo 图片信息
type ImageInfo struct {
	Type   *int   `json:"type"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// ActualPhoto 实际照片
type ActualPhoto struct {
	Ext                 map[string]interface{} `json:"ext"`
	ActualPhotoInfoList map[string]interface{} `json:"actual_photo_info_list"`
}

// GpsrInfo GPSR信息
type GpsrInfo struct {
	Ext map[string]interface{} `json:"ext"`
}

// RepInfo REP信息
type RepInfo struct {
	Ext map[string]interface{} `json:"ext"`
}

// GoodsTrademark 商品商标
type GoodsTrademark struct {
	NotSelectBrand bool `json:"not_select_brand"`
}

// GoodsOriginInfo 商品原产地信息
type GoodsOriginInfo struct {
	OriginRegionName1 string `json:"origin_region_name1"`
}

// ProductSaveResult 产品保存结果
type ProductSaveResult struct {
	GoodsID              *int `json:"goods_id"`
	ListingCommitID      int  `json:"listing_commit_id"`
	ListingCommitVersion int  `json:"listing_commit_version"`
	GoodsCommitID        int  `json:"goods_commit_id"`
}

// SourceProduct 源产品数据
type SourceProduct struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Images      []string               `json:"images"`
	Price       float64                `json:"price"`
	Currency    string                 `json:"currency"`
	Attributes  map[string]interface{} `json:"attributes"`
}

// BatchSkuInfo 批量SKU信息
type BatchSkuInfo struct {
	ProductExpressInfo ProductExpressInfo `json:"product_express_info"`
	SupplierPriceStr   string             `json:"supplier_price_str"`
	Quantity           string             `json:"quantity"`
	MultiplePackage    MultiplePackage    `json:"multiple_package"`
	OutSkuSN           string             `json:"out_sku_sn"`
}

// MultiplePackage 多包装信息
type MultiplePackage struct {
	SkuClassification  int    `json:"sku_classification"`
	NumberOfPieces     int    `json:"number_of_pieces"`
	IndividuallyPacked int    `json:"individually_packed"`
	NumberOfPiecesNew  string `json:"number_of_pieces_new"`
	PieceUnitCode      int    `json:"piece_unit_code"`
	PieceNewUnitCode   int    `json:"piece_new_unit_code"`
}

// Extra 额外信息（用于提交请求）
type Extra struct {
	Tab              int  `json:"tab"`
	MinSkuImageSize  int  `json:"min_sku_image_size"`
	MaxSkuImageSize  int  `json:"max_sku_image_size"`
	CreateEmptyGoods bool `json:"create_empty_goods"`
}

// 为了兼容性，添加类型别名
type GoodsBasic = GoodsBasicInfo
type GoodsExtensionInfo = ExtensionInfo
type GoodsServicePromise = ServicePromise
