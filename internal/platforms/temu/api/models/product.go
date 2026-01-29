// Package types 提供TEMU平台的产品数据结构定义
package models

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

// Extra 额外信息（用于提交请求）
type Extra struct {
	Tab              int  `json:"tab"`
	MinSkuImageSize  int  `json:"min_sku_image_size"`
	MaxSkuImageSize  int  `json:"max_sku_image_size"`
	CreateEmptyGoods bool `json:"create_empty_goods"`
}

// ProductListRequest 产品列表请求参数
type ProductListRequest struct {
	PageSize              int    `json:"page_size"`
	PageNo                int    `json:"page_no"`
	OrderType             int    `json:"order_type"`  // 0=降序, 1=升序
	OrderField            string `json:"order_field"` // gmt_create=创建时间
	EnableBatchSearchText bool   `json:"enable_batch_search_text"`
	SkuSearchType         int    `json:"sku_search_type"`             // 2=在售，3=不在售
	GoodsSearchType       int    `json:"goods_search_type,omitempty"` // 1=全部
}

// ProductListRequest 产品列表请求参数
type SkuListRequest struct {
	PageSize              int    `json:"page_size"`
	PageNo                int    `json:"page_no"`
	OrderType             int    `json:"order_type"`  // 0=降序, 1=升序
	OrderField            string `json:"order_field"` // gmt_create=创建时间
	EnableBatchSearchText bool   `json:"enable_batch_search_text"`
	SkuSearchType         int    `json:"sku_search_type,omitempty"` // 2=在售，3=不在售
}

// ProductListResponse 产品列表响应
type SkuListResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		PageNum int           `json:"page_num"`
		Total   int           `json:"total"`
		SkuList []SkuResponse `json:"sku_list"`
	} `json:"result"`
}

// SkuResponse TEMU SKU响应结构体
type SkuResponse struct {
	GoodsName                   string                    `json:"goods_name"`
	SpecName                    string                    `json:"spec_name"`
	SpecList                    []SpecInfo                `json:"spec_list"`
	ThumbURL                    string                    `json:"thumb_url"`
	GoodsID                     string                    `json:"goods_id"`
	GoodsCommitID               string                    `json:"goods_commit_id"`
	ListingCommitID             string                    `json:"listing_commit_id"`
	MallID                      string                    `json:"mall_id"`
	SkuID                       string                    `json:"sku_id"`
	SkuSN                       string                    `json:"sku_sn"`
	Stock                       int                       `json:"stock"`
	Price                       float64                   `json:"price"`
	PriceVO                     PriceVO                   `json:"price_vo"`
	CrtTime                     string                    `json:"crt_time"`
	Status4VO                   int                       `json:"status4_vo"`
	SubStatus4VO                int                       `json:"sub_status4_vo"`
	ClosedTypeList              []any                     `json:"closed_type_list"`
	SupplierPrice               float64                   `json:"supplier_price"`
	GoodsIsOnsale               int                       `json:"goods_is_onsale"`
	Currency                    string                    `json:"currency"`
	ListPrice                   PriceVO                   `json:"list_price"`
	ListPriceVO                 PriceVO                   `json:"list_price_vo"`
	StatusUpdateTime            string                    `json:"status_update_time"`
	CatType                     int                       `json:"cat_type"`
	LockInfo                    LockInfo                  `json:"lock_info"`
	MultiSiteGoods              bool                      `json:"multi_site_goods"`
	CheckPriceAuditInfo         map[string]interface{}    `json:"check_price_audit_info"`
	AdjustPriceAuditInfo        map[string]interface{}    `json:"adjust_price_audit_info"`
	ShowSubStatus4VO            int                       `json:"show_sub_status4_vo"`
	NewVersionApprovedLabelShow bool                      `json:"new_version_approved_label_show"`
	PersonalizationStatus       int                       `json:"personalization_status"`
	PunishTags                  int                       `json:"punish_tags"`
	CatNameList                 []string                  `json:"cat_name_list"`
	CatID                       int                       `json:"cat_id"`
	CategoryRectificationInfo   CategoryRectificationInfo `json:"category_rectification_info"`
	StockDisplayTag             int                       `json:"stock_display_tag"`
	LowTrafficTag               int                       `json:"low_traffic_tag"`
	RestrictedTrafficTag        int                       `json:"restricted_traffic_tag"`
	PreSaleStockInfo            PreSaleStockInfo          `json:"pre_sale_stock_info"`
	OrdinaryStock               int                       `json:"ordinary_stock"`
	ShippingMode                int                       `json:"shipping_mode"`
	OutGoodsSN                  string                    `json:"out_goods_sn"`
	EasyGainsTag                int                       `json:"easy_gains_tag"`
	IsBooks                     bool                      `json:"is_books"`
	StockSearchTags             []any                     `json:"stock_search_tags"`
}

// ProductSaveRequest TEMU产品保存请求结构体
type ProductSaveRequest struct {
	GoodsBasic            GoodsBasicInfo `json:"goods_basic"`
	GoodsSaleInfo         GoodsSaleInfo  `json:"goods_sale_info"`
	GoodsServicePromise   ServicePromise `json:"goods_service_promise"`
	GoodsExtensionInfo    ExtensionInfo  `json:"goods_extension_info"`
	Extra                 Extra          `json:"extra"`
	CanSave               bool           `json:"can_save"`
	SupportMaxRetailPrice bool           `json:"support_max_retail_price"`
	PlatformExpressBill   bool           `json:"platform_express_bill"`
	SkcList               []Skc          `json:"skc_list"`
	BatchSkuInfo          BatchSkuInfo   `json:"batch_sku_info"`
}

// ProductSaveResponse TEMU产品保存响应结构体
type ProductSaveResponse struct {
	Success   bool                `json:"success"`
	ErrorCode int                 `json:"error_code"`
	Message   string              `json:"error_msg,omitempty"`
	Result    *ProductSaveResult2 `json:"result,omitempty"`
}

// ProductSaveResult2 产品保存结果（避免与现有结构体冲突）
type ProductSaveResult2 struct {
	ListingCommitID      string `json:"listing_commit_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	GoodsCommitID        string `json:"goods_commit_id"`
}

// GoodsSearchRequest 商品搜索请求参数
type GoodsSearchRequest struct {
	PageSize                 int    `json:"page_size"`                    // 每页数量
	PageNo                   int    `json:"page_no"`                      // 页码
	OrderType                int    `json:"order_type"`                   // 排序类型：0=降序
	OrderField               string `json:"order_field"`                  // 排序字段：gmt_create=创建时间
	EnableBatchSearchText    bool   `json:"enable_batch_search_text"`     // 启用批量搜索文本
	StatusFilterType         int    `json:"status_filter_type"`           // 状态过滤类型：2001
	GoodsSearchType          int    `json:"goods_search_type"`            // 商品搜索类型：1=全部
	GoodsSubStatusFilterType int    `json:"goods_sub_status_filter_type"` // 商品子状态过滤类型：2001
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
	ListingCommitID        string                 `json:"listing_commit_id"`
	GoodsCommitID          string                 `json:"goods_commit_id"`
	GoodsID                string                 `json:"goods_id"`
	GoodsName              string                 `json:"goods_name"`
	SpecName               string                 `json:"spec_name"`
	ThumbURL               string                 `json:"thumb_url"`
	SkuPreviewURL          string                 `json:"sku_preview_url"`
	MallID                 string                 `json:"mall_id"`
	OutGoodsSN             string                 `json:"out_goods_sn"`
	Status4VO              int                    `json:"status4_vo"`
	SubStatus4VO           int                    `json:"sub_status4_vo"`
	ClosedTypeList         []any                  `json:"closed_type_list"`
	Currency               string                 `json:"currency"`
	OutSkuSNList           []string               `json:"out_sku_sn_list"`
	SkuIDList              []string               `json:"sku_id_list"`
	Price                  float64                `json:"price"`
	PriceVO                PriceVO                `json:"price_vo"`
	Quantity               int                    `json:"quantity"`
	VariationsCount        int                    `json:"variations_count"`
	CrtTime                string                 `json:"crt_time"`
	StatusUpdateTime       string                 `json:"status_update_time"`
	SupplierPrice          float64                `json:"supplier_price"`
	GoodsAllowSiteList     []string               `json:"goods_allow_site_list"`
	CatType                int                    `json:"cat_type"`
	CatID                  int                    `json:"cat_id"`
	CatNameList            []string               `json:"cat_name_list"`
	LockInfo               LockInfo               `json:"lock_info"`
	MultiSiteGoods         bool                   `json:"multi_site_goods"`
	CheckPriceAuditList    []CheckPriceAuditItem  `json:"check_price_audit_list"`
	AdjustPriceAuditList   []any                  `json:"adjust_price_audit_list"`
	ShowSubStatus4VO       int                    `json:"show_sub_status4_vo"`
	CheckedTime            int64                  `json:"checked_time"`
	PersonalizationStatus  int                    `json:"personalization_status"`
	PunishTags             int                    `json:"punish_tags"`
	HighPriceSpreadRatio   bool                   `json:"high_price_spread_ratio"`
	MultiSkusGoodsStatusVO MultiSkusGoodsStatusVO `json:"multi_skus_goods_status_vo"`
	GoodsOriginInfo        GoodsOriginInfo        `json:"goods_origin_info"`
	LowTrafficTag          int                    `json:"low_traffic_tag"`
	GoodsToBeUpgradeInfo   GoodsToBeUpgradeInfo   `json:"goods_to_be_upgrade_info"`
	RestrictedTrafficTag   int                    `json:"restricted_traffic_tag"`
	IsBooks                bool                   `json:"is_books"`
	StockSearchTags        []any                  `json:"stock_search_tags"`
	AdInfo                 AdInfo                 `json:"ad_info"`
}

// CheckPriceAuditItem 价格审核项
type CheckPriceAuditItem struct {
	Currency                string  `json:"currency"`
	PricingType             int     `json:"pricing_type"`
	SkuID                   string  `json:"sku_id"`
	PriceAuditOrderID       string  `json:"price_audit_order_id"`
	SuggestSupplierPrice    float64 `json:"suggest_supplier_price"`
	SourceSupplierPrice     float64 `json:"source_supplier_price"`
	TargetSupplierPrice     float64 `json:"target_supplier_price"`
	SupplierPrice           float64 `json:"supplier_price"`
	Status                  int     `json:"status"`
	PriceCommitVersion      int     `json:"price_commit_version"`
	RejectReason            string  `json:"reject_reason"`
	GoodsCommitEdit         int     `json:"goods_commit_edit"`
	SuggestSupplierPriceStr string  `json:"suggest_supplier_price_str"`
	SourceSupplierPriceStr  string  `json:"source_supplier_price_str"`
	TargetSupplierPriceStr  string  `json:"target_supplier_price_str"`
	SupplierPriceStr        string  `json:"supplier_price_str"`
}

// MultiSkusGoodsStatusVO 多SKU商品状态
type MultiSkusGoodsStatusVO struct {
	GoodsOfMultiSkuDisplayID       string  `json:"goods_of_multi_sku_display_id"`
	PriceReviewOfMultiSkuDisplayID string  `json:"price_review_of_multi_sku_display_id"`
	SkuMinSupplierPrice            float64 `json:"sku_min_supplier_price"`
	SkuMaxSupplierPrice            float64 `json:"sku_max_supplier_price"`
}

// GoodsToBeUpgradeInfo 商品待升级信息
type GoodsToBeUpgradeInfo struct {
	ToBeUpgradeTypeList []int `json:"to_be_upgrade_type_list"`
}

// AdInfo 广告信息
type AdInfo struct {
	GoodsAdStatus int `json:"goods_ad_status"`
}
