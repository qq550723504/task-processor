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
	SkuSearchType         int    `json:"sku_search_type"` // 2=全部
}

// ProductListResponse 产品列表响应
type ProductListResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		PageNum   int                   `json:"page_num"`
		Total     int                   `json:"total"`
		GoodsList []TemuProductResponse `json:"goods_list"`
	} `json:"result"`
}

// TemuProductResponse TEMU 产品响应
type TemuProductResponse struct {
	ListingCommitID       string   `json:"listing_commit_id"`
	GoodsCommitID         string   `json:"goods_commit_id"`
	GoodsID               string   `json:"goods_id"`
	GoodsName             string   `json:"goods_name"`
	SpecName              string   `json:"spec_name"`
	ThumbURL              string   `json:"thumb_url"`
	SkuPreviewURL         string   `json:"sku_preview_url"`
	MallID                string   `json:"mall_id"`
	OutGoodsSn            string   `json:"out_goods_sn"`
	Status4Vo             int      `json:"status4_vo"`     // 3=已上架
	SubStatus4Vo          int      `json:"sub_status4_vo"` // 子状态
	ClosedTypeList        []any    `json:"closed_type_list"`
	Currency              string   `json:"currency"`
	MarketPrice           string   `json:"market_price"`
	MarketPriceVo         PriceVO  `json:"market_price_vo"`
	ListPrice             PriceVO  `json:"list_price"`
	ListPriceVo           PriceVO  `json:"list_price_vo"`
	OutSkuSnList          []string `json:"out_sku_sn_list"`
	SkuIDList             []string `json:"sku_id_list"`
	Price                 float64  `json:"price"`
	PriceVo               PriceVO  `json:"price_vo"`
	Quantity              int      `json:"quantity"`
	VariationsCount       int      `json:"variations_count"`
	CrtTime               string   `json:"crt_time"`
	StatusUpdateTime      string   `json:"status_update_time"`
	SupplierPrice         float64  `json:"supplier_price"`
	GoodsAllowSiteList    []any    `json:"goods_allow_site_list"`
	CatType               int      `json:"cat_type"`
	CatID                 int64    `json:"cat_id"`
	CatNameList           []string `json:"cat_name_list"`
	MultiSiteGoods        bool     `json:"multi_site_goods"`
	ShowSubStatus4Vo      int      `json:"show_sub_status4_vo"`
	PersonalizationStatus int      `json:"personalization_status"`
	PunishTags            int      `json:"punish_tags"`
	StockDisplayTag       int      `json:"stock_display_tag"`
	LowTrafficTag         int      `json:"low_traffic_tag"`
	RestrictedTrafficTag  int      `json:"restricted_traffic_tag"`
	OrdinaryStock         int      `json:"ordinary_stock"`
	ShippingMode          int      `json:"shipping_mode"`
	EasyGainsTag          int      `json:"easy_gains_tag"`
	IsBooks               bool     `json:"is_books"`
}
