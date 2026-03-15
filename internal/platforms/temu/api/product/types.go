// Package product 提供TEMU平台产品相关的数据结构定义
package product

import "encoding/json"

// ImageInfo 图片信息
type ImageInfo struct {
	Type   *int   `json:"type"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// ProductExpressInfo 产品物流信息
type ProductExpressInfo struct {
	WeightInfo WeightInfo `json:"weight_info"`
	VolumeInfo VolumeInfo `json:"volume_info"`
}

// VolumeInfo 体积信息
type VolumeInfo struct {
	Height string `json:"height"`
	Length string `json:"length"`
	Width  string `json:"width"`
}

// WeightInfo 重量信息
type WeightInfo struct {
	Weight string `json:"weight"`
}

// SpecInfo 规格信息
type SpecInfo struct {
	SpecID         string `json:"spec_id"`
	SpecName       string `json:"spec_name"`
	ParentSpecID   string `json:"parent_spec_id"`
	ParentSpecName string `json:"parent_spec_name,omitempty"`
	ParentID       string `json:"parent_id,omitempty"`
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

func (g GoodsGallery) MarshalJSON() ([]byte, error) {
	if len(g.DetailImage) == 0 && len(g.CarouselVideo) == 0 && len(g.DetailVideo) == 0 {
		return []byte("{}"), nil
	}
	type Alias GoodsGallery
	return json.Marshal(Alias(g))
}

// GoodsTrademark 商品商标
type GoodsTrademark struct {
	NotSelectBrand bool `json:"not_select_brand"`
}

// GoodsOriginInfo 商品原产地信息
type GoodsOriginInfo struct {
	OriginRegionName1 string `json:"origin_region_name1"`
}

// Skc SKC商品结构体
type Skc struct {
	SkuList          []Sku       `json:"sku_list"`
	CarouselGallery  []ImageInfo `json:"carousel_gallery,omitempty"`
	ColorImageUrl    string      `json:"color_image_url,omitempty"`
	CommitDeleteType int         `json:"commit_delete_type,omitempty"`
	CommitDeleted    int         `json:"commit_deleted,omitempty"`
	Priority         int         `json:"priority,omitempty"`
	SkcComplete      bool        `json:"skc_complete,omitempty"`
	Spec             []SpecInfo  `json:"spec,omitempty"`
}

// Sku SKU商品结构体
type Sku struct {
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
	MarketPrice              int                `json:"market_price,omitempty"`
	MarketPriceStr           string             `json:"market_price_str,omitempty"`
	MaxRetailPrice           int                `json:"max_retail_price,omitempty"`
	Price                    int                `json:"price,omitempty"`
	PriceStr                 string             `json:"price_str,omitempty"`
	Priority                 int                `json:"priority,omitempty"`
	RetailPriceCurrency      string             `json:"retail_price_currency,omitempty"`
	SkuComplete              bool               `json:"sku_complete,omitempty"`
	SkuDeleted               bool               `json:"sku_deleted,omitempty"`
}

// BatchSkuInfo 批量SKU信息
type BatchSkuInfo struct {
	ProductExpressInfo ProductExpressInfo `json:"product_express_info"`
	SupplierPriceStr   string             `json:"supplier_price_str"`
	Quantity           string             `json:"quantity"`
	MultiplePackage    MultiplePackage    `json:"multiple_package"`
	OutSkuSN           string             `json:"out_sku_sn"`
}

// GoodsPropertys 商品属性集合
type GoodsPropertys struct {
	GoodsBrandProperties []interface{}      `json:"goods_brand_properties"`
	GoodsProperties      []PropertyItem     `json:"goods_properties"`
	GoodsSpecProperties  []GoodSpecProperty `json:"goods_spec_properties"`
}

func (g GoodsPropertys) MarshalJSON() ([]byte, error) {
	result := make(map[string]interface{})
	if len(g.GoodsBrandProperties) > 0 {
		result["goods_brand_properties"] = g.GoodsBrandProperties
	}
	if len(g.GoodsProperties) > 0 {
		result["goods_properties"] = g.GoodsProperties
	}
	result["goods_spec_properties"] = g.GoodsSpecProperties
	return json.Marshal(result)
}

// GoodSpecProperty 商品规格属性
type GoodSpecProperty struct {
	Value            string `json:"value"`
	SpecID           string `json:"spec_id"`
	ParentSpecID     string `json:"parent_spec_id"`
	ParentSpecName   string `json:"parent_spec_name"`
	Feature          int    `json:"feature,omitempty"`
	Checked          bool   `json:"checked"`
	ControlType      int    `json:"control_type"`
	Disabled         bool   `json:"disabled"`
	Name             string `json:"name"`
	IsCustomized     int    `json:"is_customized"`
	Vid              int    `json:"vid,omitempty"`
	TemplateModuleID int    `json:"template_module_id,omitempty"`
	TemplatePid      int    `json:"template_pid,omitempty"`
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

// ExtraInfo 额外信息
type ExtraInfo struct {
	CreateEmptyGoods bool        `json:"create_empty_goods"`
	VersionType      interface{} `json:"version_type"`
	Tab              int         `json:"tab"`
	MinSkuImageSize  int         `json:"min_sku_image_size"`
	MaxSkuImageSize  int         `json:"max_sku_image_size"`
	CurrentTab       int         `json:"current_tab"`
}

// ExtraTemplate 额外模板
type ExtraTemplate struct {
	ExtraTemplateDetailList []ExtraTemplateDetail `json:"extra_template_detail_list"`
}

// ExtraTemplateDetail 额外模板详情
type ExtraTemplateDetail struct {
	TemplateID int              `json:"template_id"`
	Properties map[string][]int `json:"properties"`
	InputText  map[string]any   `json:"input_text"`
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

// CertificationInfo 认证信息
type CertificationInfo struct {
	CertificateInfo map[string]any `json:"certificate_info"`
	ExtraTemplate   ExtraTemplate  `json:"extra_template"`
	ActualPhoto     ActualPhoto    `json:"actual_photo,omitempty"`
	GpsrInfo        GpsrInfo       `json:"gpsr_info,omitempty"`
	RepInfo         RepInfo        `json:"rep_info,omitempty"`
}

func (c CertificationInfo) MarshalJSON() ([]byte, error) {
	result := make(map[string]interface{})
	if len(c.CertificateInfo) > 0 {
		result["certificate_info"] = c.CertificateInfo
	}
	result["extra_template"] = c.ExtraTemplate
	if len(c.ActualPhoto.Ext) > 0 || len(c.ActualPhoto.ActualPhotoInfoList) > 0 {
		result["actual_photo"] = c.ActualPhoto
	}
	if len(c.GpsrInfo.Ext) > 0 {
		result["gpsr_info"] = c.GpsrInfo
	}
	if len(c.RepInfo.Ext) > 0 {
		result["rep_info"] = c.RepInfo
	}
	return json.Marshal(result)
}

// SizeChartImageInfo 尺寸图表信息
type SizeChartImageInfo struct {
	SizeChartImageList []ImageInfo `json:"size_chart_image_list"`
}

func (s *SizeChartImageInfo) MarshalJSON() ([]byte, error) {
	if s == nil || len(s.SizeChartImageList) == 0 {
		return []byte("null"), nil
	}
	type Alias SizeChartImageInfo
	return json.Marshal((*Alias)(s))
}

// ExtensionInfo 扩展信息
type ExtensionInfo struct {
	GoodsProperty             GoodsPropertys      `json:"goods_property"`
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

// ProductSaveResult 产品保存结果（旧版兼容）
type ProductSaveResult struct {
	GoodsID              *int `json:"goods_id"`
	ListingCommitID      int  `json:"listing_commit_id"`
	ListingCommitVersion int  `json:"listing_commit_version"`
	GoodsCommitID        int  `json:"goods_commit_id"`
}

// SaveRequest 产品保存请求
type SaveRequest struct {
	GoodsBasic            GoodsBasicInfo `json:"goods_basic"`
	GoodsSaleInfo         GoodsSaleInfo  `json:"goods_sale_info"`
	GoodsServicePromise   ServicePromise `json:"goods_service_promise"`
	GoodsExtensionInfo    ExtensionInfo  `json:"goods_extension_info"`
	Extra                 ExtraInfo      `json:"extra"`
	CanSave               bool           `json:"can_save"`
	SupportMaxRetailPrice bool           `json:"support_max_retail_price"`
	PlatformExpressBill   bool           `json:"platform_express_bill"`
	SkcList               []Skc          `json:"skc_list"`
	BatchSkuInfo          BatchSkuInfo   `json:"batch_sku_info"`
}

// SaveResponse 产品保存响应
type SaveResponse struct {
	Success   bool        `json:"success"`
	ErrorCode int         `json:"error_code"`
	Message   string      `json:"error_msg,omitempty"`
	Result    *SaveResult `json:"result,omitempty"`
}

// SaveResult 产品保存结果
type SaveResult struct {
	ListingCommitID      string `json:"listing_commit_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	GoodsCommitID        string `json:"goods_commit_id"`
}

// SubmitRequest 产品提交请求
type SubmitRequest struct {
	GoodsBasic            GoodsBasicInfo `json:"goods_basic"`
	GoodsSaleInfo         GoodsSaleInfo  `json:"goods_sale_info"`
	GoodsServicePromise   ServicePromise `json:"goods_service_promise"`
	GoodsExtensionInfo    ExtensionInfo  `json:"goods_extension_info"`
	Extra                 ExtraInfo      `json:"extra"`
	CanSave               bool           `json:"can_save"`
	SupportMaxRetailPrice bool           `json:"support_max_retail_price"`
	PlatformExpressBill   bool           `json:"platform_express_bill"`
	SkcList               []Skc          `json:"skc_list"`
}

// SubmitResponse 产品提交响应
type SubmitResponse struct {
	Success   bool          `json:"success"`
	ErrorCode int           `json:"error_code"`
	Message   string        `json:"error_msg,omitempty"`
	Result    *SubmitResult `json:"result,omitempty"`
}

// SubmitResult 产品提交结果
type SubmitResult struct {
	ListingCommitID      string `json:"listing_commit_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	GoodsCommitID        string `json:"goods_commit_id"`
	Status               int    `json:"status"`
	Message              string `json:"message,omitempty"`
}

// CreateCommitRequest 创建提交请求
type CreateCommitRequest struct {
	CatIDs      []int  `json:"cat_ids"`
	CatID       int    `json:"cat_id"`
	GoodsName   string `json:"goods_name"`
	OperateType int    `json:"operate_type"`
}

// CreateCommitResponse 创建提交响应
type CreateCommitResponse struct {
	Success   bool                `json:"success"`
	ErrorCode int                 `json:"error_code"`
	Message   string              `json:"error_msg,omitempty"`
	Result    *CreateCommitResult `json:"result,omitempty"`
}

// CreateCommitResult 创建提交结果
type CreateCommitResult struct {
	GoodsID              string `json:"goods_id"`
	ListingCommitID      string `json:"listing_commit_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	GoodsCommitID        string `json:"goods_commit_id"`
}

// GoodsSearchRequest 商品搜索请求
type GoodsSearchRequest struct {
	PageSize                 int    `json:"page_size"`
	PageNo                   int    `json:"page_no"`
	OrderType                int    `json:"order_type"`
	OrderField               string `json:"order_field"`
	EnableBatchSearchText    bool   `json:"enable_batch_search_text"`
	StatusFilterType         int    `json:"status_filter_type"`
	GoodsSearchType          int    `json:"goods_search_type"`
	GoodsSubStatusFilterType int    `json:"goods_sub_status_filter_type"`
}

// GoodsSearchResponse 商品搜索响应
type GoodsSearchResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		PageNum   int               `json:"page_num"`
		Total     int               `json:"total"`
		GoodsList []GoodsSearchItem `json:"goods_list"`
	} `json:"result"`
}

// GoodsSearchItem 商品搜索结果项
type GoodsSearchItem struct {
	ListingCommitID       string   `json:"listing_commit_id"`
	GoodsCommitID         string   `json:"goods_commit_id"`
	GoodsID               string   `json:"goods_id"`
	GoodsName             string   `json:"goods_name"`
	SpecName              string   `json:"spec_name"`
	ThumbURL              string   `json:"thumb_url"`
	MallID                string   `json:"mall_id"`
	OutGoodsSN            string   `json:"out_goods_sn"`
	Status4VO             int      `json:"status4_vo"`
	SubStatus4VO          int      `json:"sub_status4_vo"`
	ClosedTypeList        []any    `json:"closed_type_list"`
	Currency              string   `json:"currency"`
	OutSkuSNList          []string `json:"out_sku_sn_list"`
	SkuIDList             []string `json:"sku_id_list"`
	Price                 float64  `json:"price"`
	Quantity              int      `json:"quantity"`
	VariationsCount       int      `json:"variations_count"`
	CrtTime               string   `json:"crt_time"`
	StatusUpdateTime      string   `json:"status_update_time"`
	SupplierPrice         float64  `json:"supplier_price"`
	CatType               int      `json:"cat_type"`
	CatID                 int      `json:"cat_id"`
	CatNameList           []string `json:"cat_name_list"`
	MultiSiteGoods        bool     `json:"multi_site_goods"`
	ShowSubStatus4VO      int      `json:"show_sub_status4_vo"`
	PersonalizationStatus int      `json:"personalization_status"`
	PunishTags            int      `json:"punish_tags"`
	LowTrafficTag         int      `json:"low_traffic_tag"`
	RestrictedTrafficTag  int      `json:"restricted_traffic_tag"`
	IsBooks               bool     `json:"is_books"`
	StockSearchTags       []any    `json:"stock_search_tags"`
	HighPriceSpreadRatio  float64  `json:"high_price_spread_ratio"`
	CheckedTime           string   `json:"checked_time"`
	SkuPreviewURL         string   `json:"sku_preview_url"`
}

// PriceQueryRequest 价格查询请求
type PriceQueryRequest struct {
	GoodsID                      string                    `json:"goods_id"`
	MmsSkuMaxRetailPriceQryItems []MaxRetailPriceQueryItem `json:"mms_sku_max_retail_price_qry_items"`
}

// MaxRetailPriceQueryItem 最大零售价查询项
type MaxRetailPriceQueryItem struct {
	BasePriceStr string `json:"base_price_str"`
	Currency     string `json:"currency"`
}

// PriceQueryResponse 价格查询响应
type PriceQueryResponse struct {
	Success   bool              `json:"success"`
	ErrorCode int               `json:"error_code"`
	ErrorMsg  string            `json:"error_msg,omitempty"`
	Result    *PriceQueryResult `json:"result,omitempty"`
}

// PriceQueryResult 价格查询结果
type PriceQueryResult struct {
	MmsSkuMaxRetailPriceItems []MaxRetailPriceResultItem `json:"mms_sku_max_retail_price_items"`
}

// MaxRetailPriceResultItem 最大零售价结果项
type MaxRetailPriceResultItem struct {
	BasePriceStr        string `json:"base_price_str"`
	Currency            string `json:"currency"`
	MaxRetailPriceStr   string `json:"max_retail_price_str"`
	RetailPriceCurrency string `json:"retail_price_currency"`
}

// BulkRelistOptions 批量重新上架选项
type BulkRelistOptions struct {
	DelayBetweenRequests int             `json:"delay_between_requests"`
	SkipConditions       *SkipConditions `json:"skip_conditions"`
	MaxConcurrency       int             `json:"max_concurrency"`
	DryRun               bool            `json:"dry_run"`
	ProcessFirstPageOnly bool            `json:"process_first_page_only"`
	PrintProductData     bool            `json:"print_product_data"`
}

// SkipConditions 跳过条件
type SkipConditions struct {
	SkipNeedRectification bool `json:"skip_need_rectification"`
	SkipSeverelyPunished  bool `json:"skip_severely_punished"`
	SkipLocked            bool `json:"skip_locked"`
	SkipNoStock           bool `json:"skip_no_stock"`
	MinStock              int  `json:"min_stock"`
}

// BulkRelistSummary 批量重新上架摘要
type BulkRelistSummary struct {
	StartTime         string                   `json:"start_time"`
	EndTime           string                   `json:"end_time"`
	Duration          string                   `json:"duration"`
	TotalOfflineCount int                      `json:"total_offline_count"`
	ProcessedCount    int                      `json:"processed_count"`
	SuccessCount      int                      `json:"success_count"`
	FailCount         int                      `json:"fail_count"`
	SkippedCount      int                      `json:"skipped_count"`
	SuccessRate       float64                  `json:"success_rate"`
	CategoryStats     map[string]CategoryStats `json:"category_stats"`
	ErrorStats        map[string]int           `json:"error_stats"`
}

// CategoryStats 分类统计
type CategoryStats struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}
