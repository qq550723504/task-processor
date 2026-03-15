// Package product 提供TEMU平台SKU列表查询相关数据结构
package product

// PriceVO 价格对象
type PriceVO struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// SkuLockInfo SKU锁定信息
type SkuLockInfo struct {
	IsForbidEdit                 bool `json:"is_forbid_edit"`
	AllowEditPersonalizationInfo bool `json:"allow_edit_personalization_info"`
}

// SkuPreSaleStockInfo SKU预售库存信息
type SkuPreSaleStockInfo struct {
	PreSaleStockShowStatus         int `json:"pre_sale_stock_show_status"`
	MaximumPreSaleStock            int `json:"maximum_pre_sale_stock"`
	Version                        int `json:"version"`
	PreSaleEnableDays              int `json:"pre_sale_enable_days"`
	PreSaleStockReopenIntervalDate int `json:"pre_sale_stock_reopen_interval_date"`
}

// SkuListRequest SKU列表查询请求
type SkuListRequest struct {
	PageSize              int    `json:"page_size"`
	PageNo                int    `json:"page_no"`
	OrderType             int    `json:"order_type"`
	OrderField            string `json:"order_field"`
	EnableBatchSearchText bool   `json:"enable_batch_search_text"`
	SkuSearchType         int    `json:"sku_search_type,omitempty"`
}

// SkuListResponse SKU列表查询响应
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
	GoodsName             string              `json:"goods_name"`
	SpecName              string              `json:"spec_name"`
	SpecList              []SpecInfo          `json:"spec_list"`
	ThumbURL              string              `json:"thumb_url"`
	GoodsID               string              `json:"goods_id"`
	GoodsCommitID         string              `json:"goods_commit_id"`
	ListingCommitID       string              `json:"listing_commit_id"`
	MallID                string              `json:"mall_id"`
	SkuID                 string              `json:"sku_id"`
	SkuSN                 string              `json:"sku_sn"`
	Stock                 int                 `json:"stock"`
	Price                 float64             `json:"price"`
	PriceVO               PriceVO             `json:"price_vo"`
	CrtTime               string              `json:"crt_time"`
	Status4VO             int                 `json:"status4_vo"`
	SubStatus4VO          int                 `json:"sub_status4_vo"`
	ClosedTypeList        []any               `json:"closed_type_list"`
	SupplierPrice         float64             `json:"supplier_price"`
	GoodsIsOnsale         int                 `json:"goods_is_onsale"`
	Currency              string              `json:"currency"`
	ListPrice             PriceVO             `json:"list_price"`
	ListPriceVO           PriceVO             `json:"list_price_vo"`
	StatusUpdateTime      string              `json:"status_update_time"`
	CatType               int                 `json:"cat_type"`
	LockInfo              SkuLockInfo         `json:"lock_info"`
	MultiSiteGoods        bool                `json:"multi_site_goods"`
	ShowSubStatus4VO      int                 `json:"show_sub_status4_vo"`
	PersonalizationStatus int                 `json:"personalization_status"`
	PunishTags            int                 `json:"punish_tags"`
	CatNameList           []string            `json:"cat_name_list"`
	CatID                 int                 `json:"cat_id"`
	StockDisplayTag       int                 `json:"stock_display_tag"`
	LowTrafficTag         int                 `json:"low_traffic_tag"`
	RestrictedTrafficTag  int                 `json:"restricted_traffic_tag"`
	PreSaleStockInfo      SkuPreSaleStockInfo `json:"pre_sale_stock_info"`
	OrdinaryStock         int                 `json:"ordinary_stock"`
	ShippingMode          int                 `json:"shipping_mode"`
	OutGoodsSN            string              `json:"out_goods_sn"`
	EasyGainsTag          int                 `json:"easy_gains_tag"`
	IsBooks               bool                `json:"is_books"`
	StockSearchTags       []any               `json:"stock_search_tags"`
}
