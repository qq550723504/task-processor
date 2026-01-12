package models

// PendingPriceListRequest 待核价列表请求
type PendingPriceListRequest struct {
	PageSize int    `json:"page_size"`
	PageNo   int    `json:"page_no"`
	Scene    string `json:"scene"` // PRICING_HEALTH_SALES_BOOST 表示价格健康-销量提升场景
}

// PendingPriceListResponse 待核价列表响应
type PendingPriceListResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		PageNum             int               `json:"page_num"`
		Total               int               `json:"total"`
		SalesBoostGoodsList []SalesBoostGoods `json:"sales_boost_goods_list"`
	} `json:"result"`
}

// SalesBoostGoods 销量提升商品信息
type SalesBoostGoods struct {
	SalesBoostGoodsBasicInfo SalesBoostGoodsBasicInfo `json:"sales_boost_goods_basic_info"`
	SalesBoostSkuList        []SalesBoostSku          `json:"sales_boost_sku_list"`
	HoverInfo                HoverInfo                `json:"hover_info"`
}

// SalesBoostGoodsBasicInfo 商品基本信息
type SalesBoostGoodsBasicInfo struct {
	GoodsID     string   `json:"goods_id"`
	GoodsName   string   `json:"goods_name"`
	CatType     int      `json:"cat_type"`
	CatID       string   `json:"cat_id"`
	CatNameList []string `json:"cat_name_list"`
	HdThumbURL  string   `json:"hd_thumb_url"`
	IsBooks     bool     `json:"is_books"`
}

// SalesBoostSku 销量提升SKU信息
type SalesBoostSku struct {
	GoodsID               string                 `json:"goods_id"`
	SkcID                 string                 `json:"skc_id"`
	SkuID                 string                 `json:"sku_id"`
	SpecNames             []PricingSpecInfo      `json:"spec_names"`
	SkuThumbURL           string                 `json:"sku_thumb_url"`
	OutSkuSN              string                 `json:"out_sku_sn"`
	CurrentSupplierPrice  PriceVO                `json:"current_supplier_price"`
	TargetSupplierPrice   PriceVO                `json:"target_supplier_price"`
	TargetRetailPrice     PriceVO                `json:"target_retail_price"`
	CurrentRetailPrice    PriceVO                `json:"current_retail_price"`
	RecRetailPriceTipInfo map[string]interface{} `json:"rec_retail_price_tip_info"`
	CreateTime            int64                  `json:"create_time"`
	PriceSpreadRatio      string                 `json:"price_spread_ratio"`
	EasyGainsTag          int                    `json:"easy_gains_tag"`
	Active                int                    `json:"active"`
	HighPriceSpreadRatio  bool                   `json:"high_price_spread_ratio"`
	ProposalStatus        int                    `json:"proposal_status"`
	ActionInfo            ActionInfo             `json:"action_info"`
}

// ActionInfo 操作信息
type ActionInfo struct {
	AllowCreateAppealOrder bool `json:"allow_create_appeal_order"`
	AllowViewAppealOrder   bool `json:"allow_view_appeal_order"`
	ShowAppealBubble       bool `json:"show_appeal_bubble"`
}

// HoverInfo 悬浮信息
type HoverInfo struct {
	HoverSkuInfoList   []HoverSkuInfo `json:"hover_sku_info_list"`
	TotalSkuCount      int            `json:"total_sku_count"`
	NeedActionSkuCount int            `json:"need_action_sku_count"`
}

// HoverSkuInfo 悬浮SKU信息
type HoverSkuInfo struct {
	SkuID       string            `json:"sku_id"`
	SpecNames   []PricingSpecInfo `json:"spec_names"`
	SkuThumbURL string            `json:"sku_thumb_url"`
	OutSkuSN    string            `json:"out_sku_sn"`
	NeedAction  int               `json:"need_action"`
}

// PriceVO 价格对象
type PriceVO struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// SpecInfo 规格信息
type PricingSpecInfo struct {
	SpecID         string `json:"spec_id"`
	SpecName       string `json:"spec_name"`
	ParentSpecName string `json:"parent_spec_name"`
}

// LockInfo 锁定信息
type LockInfo struct {
	CloseListingMMS struct {
		AllowOperate       bool   `json:"allow_operate"`
		AllowShow          bool   `json:"allow_show"`
		AllowOperateReason string `json:"allow_operate_reason"`
	} `json:"close_listing_mms"`
	AllowEditPersonalizationInfo bool `json:"allow_edit_personalization_info"`
}

// AdjustPriceAuditInfo 调价审核信息
type AdjustPriceAuditInfo struct {
	Currency                string  `json:"currency"`
	PricingType             int     `json:"pricing_type"`
	PriceAuditOrderID       string  `json:"price_audit_order_id"`
	SuggestSupplierPrice    float64 `json:"suggest_supplier_price"`
	SourceSupplierPrice     int     `json:"source_supplier_price"`
	TargetSupplierPrice     int     `json:"target_supplier_price"`
	SupplierPrice           int     `json:"supplier_price"`
	CrtTime                 int64   `json:"crt_time"`
	UptTime                 int64   `json:"upt_time"`
	Status                  int     `json:"status"`
	PriceCommitVersion      int     `json:"price_commit_version"`
	GoodsCommitEdit         int     `json:"goods_commit_edit"`
	SuggestSupplierPriceStr string  `json:"suggest_supplier_price_str"`
	SourceSupplierPriceStr  string  `json:"source_supplier_price_str"`
	TargetSupplierPriceStr  string  `json:"target_supplier_price_str"`
	SupplierPriceStr        string  `json:"supplier_price_str"`
}

// PreSaleStockInfo 预售库存信息
type PreSaleStockInfo struct {
	PreSaleStockShowStatus         int `json:"pre_sale_stock_show_status"`
	MaximumPreSaleStock            int `json:"maximum_pre_sale_stock"`
	Version                        int `json:"version"`
	PreSaleEnableDays              int `json:"pre_sale_enable_days"`
	PreSaleStockReopenIntervalDate int `json:"pre_sale_stock_reopen_interval_date"`
}

// Sku 待核价商品项
type PricingSku struct {
	GoodsName                   string                 `json:"goods_name"`
	SpecName                    string                 `json:"spec_name"`
	SpecList                    []PricingSpecInfo      `json:"spec_list"`
	ThumbURL                    string                 `json:"thumb_url"`
	GoodsID                     string                 `json:"goods_id"`
	GoodsCommitID               string                 `json:"goods_commit_id"`
	ListingCommitID             string                 `json:"listing_commit_id"`
	MallID                      string                 `json:"mall_id"`
	SkuID                       string                 `json:"sku_id"`
	SkuSN                       string                 `json:"sku_sn"`
	Stock                       int                    `json:"stock"`
	Price                       float64                `json:"price"`
	PriceVO                     PriceVO                `json:"price_vo"`
	CrtTime                     string                 `json:"crt_time"`
	Status4VO                   int                    `json:"status4_vo"`
	SubStatus4VO                int                    `json:"sub_status4_vo"`
	ClosedTypeList              []interface{}          `json:"closed_type_list"`
	SupplierPrice               float64                `json:"supplier_price"`
	GoodsIsOnsale               int                    `json:"goods_is_onsale"`
	Currency                    string                 `json:"currency"`
	ListPrice                   PriceVO                `json:"list_price"`
	ListPriceVO                 PriceVO                `json:"list_price_vo"`
	StatusUpdateTime            string                 `json:"status_update_time"`
	CatType                     int                    `json:"cat_type"`
	LockInfo                    LockInfo               `json:"lock_info"`
	MultiSiteGoods              bool                   `json:"multi_site_goods"`
	CheckPriceAuditInfo         map[string]interface{} `json:"check_price_audit_info"`
	AdjustPriceAuditInfo        AdjustPriceAuditInfo   `json:"adjust_price_audit_info"`
	ShowSubStatus4VO            int                    `json:"show_sub_status4_vo"`
	NewVersionApprovedLabelShow bool                   `json:"new_version_approved_label_show"`
	CheckedTime                 int64                  `json:"checked_time"`
	PersonalizationStatus       int                    `json:"personalization_status"`
	PunishTags                  int                    `json:"punish_tags"`
	CatNameList                 []string               `json:"cat_name_list"`
	CatID                       int                    `json:"cat_id"`
	StockDisplayTag             int                    `json:"stock_display_tag"`
	LowTrafficTag               int                    `json:"low_traffic_tag"`
	RestrictedTrafficTag        int                    `json:"restricted_traffic_tag"`
	PreSaleStockInfo            PreSaleStockInfo       `json:"pre_sale_stock_info"`
	OrdinaryStock               int                    `json:"ordinary_stock"`
	ShippingMode                int                    `json:"shipping_mode"`
	OutGoodsSN                  string                 `json:"out_goods_sn"`
	EasyGainsTag                int                    `json:"easy_gains_tag"`
	IsBooks                     bool                   `json:"is_books"`
}

// RejectPriceRequest 拒绝平台报价请求
type RejectPriceRequest struct {
	GoodsID         string   `json:"goods_id"`
	SkuIDs          []string `json:"sku_ids"`
	OperationSource int      `json:"operation_source"` // 1005表示价格健康页面
}

// RejectPriceResponse 拒绝平台报价响应
type RejectPriceResponse struct {
	Success   bool     `json:"success"`
	ErrorCode int      `json:"error_code"`
	Result    struct{} `json:"result"`
}

// ReappealPriceRequest 重新报价请求
type ReappealPriceRequest struct {
	GoodsID              string            `json:"goods_id"`
	AppealSource         int               `json:"appeal_source"` // 100表示价格健康页面
	MerchantAppealReason []string          `json:"merchant_appeal_reason"`
	SkuInfoList          []ReappealSkuInfo `json:"sku_info_list"`
}

// ReappealSkuInfo 重新报价SKU信息
type ReappealSkuInfo struct {
	SkuID                       string `json:"sku_id"`
	SupplierPriceStr            string `json:"supplier_price_str"`             // 当前供应商价格
	RecommendedSupplierPriceStr string `json:"recommended_supplier_price_str"` // 平台推荐价格
	TargetSupplierPriceStr      string `json:"target_supplier_price_str"`      // 目标报价
	Currency                    string `json:"currency"`                       // 货币单位
}

// ReappealPriceResponse 重新报价响应
type ReappealPriceResponse struct {
	Success   bool     `json:"success"`
	ErrorCode int      `json:"error_code"`
	Result    struct{} `json:"result"`
}

// AcceptPriceRequest 接受平台报价请求
type AcceptPriceRequest struct {
	Scene   int                  `json:"scene"` // 2表示价格健康页面
	GoodsID string               `json:"goods_id"`
	SkuList []AcceptPriceSkuInfo `json:"sku_list"`
}

// AcceptPriceSkuInfo 接受报价SKU信息
type AcceptPriceSkuInfo struct {
	SkuID                  string `json:"sku_id"`
	Currency               string `json:"currency"`
	TargetSupplierPriceStr string `json:"target_supplier_price_str"` // 目标价格，空字符串表示接受平台推荐价格
}

// AcceptPriceResponse 接受平台报价响应
type AcceptPriceResponse struct {
	Success   bool     `json:"success"`
	ErrorCode int      `json:"error_code"`
	Result    struct{} `json:"result"`
}

// PricingDecisionAction 核价决策动作
type PricingDecisionAction string

const (
	DecisionAccept   PricingDecisionAction = "accept"   // 接受平台报价
	DecisionReject   PricingDecisionAction = "reject"   // 拒绝报价
	DecisionReappeal PricingDecisionAction = "reappeal" // 重新报价
	DecisionSkip     PricingDecisionAction = "skip"     // 跳过（数据不足等）
)

// PricingDecision 核价决策结果
type PricingDecision struct {
	Sku             *PricingSku           // 待核价商品
	Action          PricingDecisionAction // 决策动作
	Reason          string                // 决策原因
	ProfitMargin    float64               // 计算出的利润率
	TargetPrice     float64               // 目标价格（重新报价时使用）
	TargetMargin    float64               // 目标利润率
	MinMargin       float64               // 最低利润率
	AcceptablePrice float64               // 重新报价价格
}

// PricingStatistics 核价统计信息
type PricingStatistics struct {
	TotalProcessed int // 总处理数
	AcceptCount    int // 接受数
	RejectCount    int // 拒绝数
	ReappealCount  int // 重新报价数
	SkipCount      int // 跳过数
	SuccessCount   int // 成功数
	FailCount      int // 失败数
}
