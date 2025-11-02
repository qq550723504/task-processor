package types

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
}

type Skc struct {
	CarouselGallery  []ImageInfo `json:"carousel_gallery"`
	ColorImageUrl    string      `json:"color_image_url"`
	CommitDeleteType int         `json:"commit_delete_type"`
	CommitDeleted    int         `json:"commit_deleted"`
	Priority         int         `json:"priority"`
	SkcComplete      bool        `json:"skc_complete"`
	SkcID            string      `json:"skc_id"`
	SkuList          []Sku       `json:"sku_list"`
	Spec             []SpecInfo  `json:"spec"`
}

type Sku struct {
	CarouselGallery          []ImageInfo        `json:"carousel_gallery"`
	Currency                 string             `json:"currency"`
	MaxRetailPrice           int                `json:"max_retail_price"`
	MaxRetailPriceStr        string             `json:"max_retail_price_str"`
	OutSkuSN                 string             `json:"out_sku_sn"`
	Price                    int                `json:"price"`
	PriceStr                 string             `json:"price_str"`
	Priority                 int                `json:"priority"`
	ProductExpressInfo       ProductExpressInfo `json:"product_express_info"`
	Quantity                 int                `json:"quantity"`
	RetailPriceCurrency      string             `json:"retail_price_currency"`
	SkcId                    string             `json:"skc_id"`
	SkuComplete              bool               `json:"sku_complete"`
	SkuDeleted               bool               `json:"sku_deleted"`
	SkuID                    string             `json:"sku_id"`
	SkuPriceDocuments        []SkuPriceDocument `json:"sku_price_documents"`
	Spec                     []SpecInfo         `json:"spec"`
	SupplierPrice            int                `json:"supplier_price"`
	SupplierPriceStr         string             `json:"supplier_price_str"`
	UseEstimateSupplierPrice bool               `json:"use_estimate_supplier_price"`
}

// SpecInfo 规格信息
type SpecInfo struct {
	ParentSpecID string `json:"parent_spec_id"`
	SpecID       string `json:"spec_id"`
	SpecName     string `json:"spec_name"`
	ParentID     string `json:"parent_id"`
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
	Unit   string `json:"unit"`
	Width  string `json:"width"`
}

// WeightInfo 重量信息
type WeightInfo struct {
	Weight string `json:"weight"`
	Unit   string `json:"unit"`
}

// GoodsBasicInfo 商品基本信息
type GoodsBasicInfo struct {
	GoodsID                 string        `json:"goods_id"`
	ListingCommitID         string        `json:"listing_commit_id"`
	ListingCommitVersion    string        `json:"listing_commit_version"`
	GoodsName               string        `json:"goods_name"`
	GoodsCreateTime         int64         `json:"goods_create_time"`
	GoodsCommitID           string        `json:"goods_commit_id"`
	Lang                    string        `json:"lang"`
	AllowSite               []int         `json:"allow_site"`
	CatID                   int           `json:"cat_id"`
	CatIDs                  []int         `json:"cat_ids"`
	CategoryTree            CategoryTree  `json:"category_tree"`
	CategoryDisclaimer      Disclaimer    `json:"category_disclaimer"`
	GoodsType               int           `json:"goods_type"`
	HdThumbURL              string        `json:"hd_thumb_url"`
	GoodsGallery            GoodsGallery  `json:"goods_gallery"`
	IsOnSale                int           `json:"is_on_sale"`
	CatType                 int           `json:"cat_type"`
	IsClothes               bool          `json:"is_clothes"`
	IsBooks                 bool          `json:"is_books"`
	CanSkipRequiredProperty bool          `json:"can_skip_required_property"`
	IsShop                  bool          `json:"is_shop"`
	FromCopy                bool          `json:"from_copy"`
	HasSubmitted            bool          `json:"has_submitted"`
	Source                  int           `json:"source"`
	OutGoodsSN              string        `json:"out_goods_sn"`
	ListPriceRequired       bool          `json:"list_price_required"`
	ListPriceDocuments      bool          `json:"list_price_documents"`
	NeedAccessoryInfo       bool          `json:"need_accessory_info"`
	AccessoryInfoRequired   bool          `json:"accessory_info_required"`
	Customized              bool          `json:"customized"`
	SecondHand              bool          `json:"second_hand"`
	SupportCustomizedGoods  bool          `json:"support_customized_goods"`
	RecommendURLPrice       bool          `json:"recommend_url_price"`
	AgreeMaxRetailPrice     bool          `json:"agree_max_retail_price"`
	CanEditSecondHand       bool          `json:"can_edit_second_hand"`
	MadeToOrder             bool          `json:"made_to_order"`
	Spec                    []interface{} `json:"spec"`
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
	GoodsProperty             GoodsProperty      `json:"goods_property"`
	CertificationInfo         CertificationInfo  `json:"certification_info"`
	GoodsTrademark            GoodsTrademark     `json:"goods_trademark"`
	GoodsProductTaxCodeDetail interface{}        `json:"goods_product_tax_code_detail"`
	GoodsOriginInfo           GoodsOriginInfo    `json:"goods_origin_info"`
	GoodsDesc                 string             `json:"goods_desc"`
	BulletPoints              []string           `json:"bullet_points"`
	SecondHand                interface{}        `json:"second_hand"`
	GuideFileInfo             interface{}        `json:"guide_file_info"`
	SizeChartImageInfo        SizeChartImageInfo `json:"size_chart_image_info"`
}

type SizeChartImageInfo struct {
	SizeChartImageList []ImageInfo `json:"size_chart_image_list"`
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
	Cate5ID      int      `json:"cate5_id"`
	Cate5Name    string   `json:"cate5_name"`
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

// GoodsProperty 商品属性
type GoodsProperty struct {
	GoodsBrandProperties []interface{}  `json:"goods_brand_properties"`
	GoodsProperties      []PropertyItem `json:"goods_properties"`
	GoodsSpecProperties  []interface{}  `json:"goods_spec_properties"`
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
	CertificateInfo interface{}   `json:"certificate_info"`
	ExtraTemplate   ExtraTemplate `json:"extra_template"`
	ActualPhoto     ActualPhoto   `json:"actual_photo"`
	GpsrInfo        GpsrInfo      `json:"gpsr_info"`
	RepInfo         RepInfo       `json:"rep_info"`
}

// ExtraTemplate 额外模板
type ExtraTemplate struct {
	ExtraTemplateDetailList []interface{} `json:"extra_template_detail_list"`
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
	Ext                 interface{} `json:"ext"`
	ActualPhotoInfoList interface{} `json:"actual_photo_info_list"`
}

// GpsrInfo GPSR信息
type GpsrInfo struct {
	Ext interface{} `json:"ext"`
}

// RepInfo REP信息
type RepInfo struct {
	Ext interface{} `json:"ext"`
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
