package product

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
