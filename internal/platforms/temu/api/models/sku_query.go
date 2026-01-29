// Package models 提供TEMU平台SKU查询相关数据结构定义
package models

// SkuQueryRequest SKU查询请求参数
type SkuQueryRequest struct {
	CommitID             string `json:"commit_id"`                // 提交ID
	GoodsID              string `json:"goods_id"`                 // 商品ID
	SourceTypeOfSkuQuery int    `json:"source_type_of_sku_query"` // SKU查询来源类型
	Source               int    `json:"source"`                   // 来源
}

// SkuQueryResponse SKU查询响应
type SkuQueryResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		Total   int                  `json:"total"`
		SkuList []SkuQueryResultItem `json:"sku_list"`
	} `json:"result"`
}

// SkuQueryResultItem SKU查询结果项
type SkuQueryResultItem struct {
	GoodsName                            string                 `json:"goods_name"`
	SpecName                             string                 `json:"spec_name"`
	SpecList                             []SkuSpecInfo          `json:"spec_list"`
	ThumbURL                             string                 `json:"thumb_url"`
	GoodsID                              string                 `json:"goods_id"`
	GoodsCommitID                        string                 `json:"goods_commit_id"`
	ListingCommitID                      string                 `json:"listing_commit_id"`
	MallID                               string                 `json:"mall_id"`
	SkuID                                string                 `json:"sku_id"`
	SkuSN                                string                 `json:"sku_sn"`
	Stock                                int                    `json:"stock"`
	Price                                float64                `json:"price"`
	PriceVO                              PriceVO                `json:"price_vo"`
	CrtTime                              string                 `json:"crt_time"`
	Status4VO                            int                    `json:"status4_vo"`
	SubStatus4VO                         int                    `json:"sub_status4_vo"`
	ClosedTypeList                       []any                  `json:"closed_type_list"`
	SupplierPrice                        float64                `json:"supplier_price"`
	GoodsIsOnsale                        int                    `json:"goods_is_onsale"`
	Currency                             string                 `json:"currency"`
	ListPrice                            PriceVO                `json:"list_price"`
	ListPriceVO                          PriceVO                `json:"list_price_vo"`
	StatusUpdateTime                     string                 `json:"status_update_time"`
	CatType                              int                    `json:"cat_type"`
	LockInfo                             SkuLockInfo            `json:"lock_info"`
	MultiSiteGoods                       bool                   `json:"multi_site_goods"`
	SkcID                                string                 `json:"skc_id"`
	CheckPriceAuditInfo                  CheckPriceAuditInfo    `json:"check_price_audit_info"`
	AdjustPriceAuditInfo                 map[string]interface{} `json:"adjust_price_audit_info"`
	ShowSubStatus4VO                     int                    `json:"show_sub_status4_vo"`
	NewVersionApprovedLabelShow          bool                   `json:"new_version_approved_label_show"`
	CheckedTime                          int64                  `json:"checked_time"`
	PersonalizationStatus                int                    `json:"personalization_status"`
	PunishTags                           int                    `json:"punish_tags"`
	CatNameList                          []string               `json:"cat_name_list"`
	CatID                                int                    `json:"cat_id"`
	StockDisplayTag                      int                    `json:"stock_display_tag"`
	LowTrafficTag                        int                    `json:"low_traffic_tag"`
	GoodsToBeUpgradeInfo                 GoodsToBeUpgradeInfo   `json:"goods_to_be_upgrade_info"`
	RestrictedTrafficTag                 int                    `json:"restricted_traffic_tag"`
	PreSaleStockInfo                     SkuPreSaleStockInfo    `json:"pre_sale_stock_info"`
	OrdinaryStock                        int                    `json:"ordinary_stock"`
	LowTrafficSuggestTargetSupplierPrice float64                `json:"low_traffic_suggest_target_supplier_price"`
	LowTrafficShowAppealBubble           bool                   `json:"low_traffic_show_appeal_bubble"`
	ProposalStatus                       int                    `json:"proposal_status"`
	HighPriceSpreadRatio                 bool                   `json:"high_price_spread_ratio"`
	ShippingMode                         int                    `json:"shipping_mode"`
	OutGoodsSN                           string                 `json:"out_goods_sn"`
	EasyGainsTag                         int                    `json:"easy_gains_tag"`
	IsBooks                              bool                   `json:"is_books"`
	StockSearchTags                      []any                  `json:"stock_search_tags"`
	ShowSimilarReference                 bool                   `json:"show_similar_reference"`
}

// SkuSpecInfo SKU规格信息
type SkuSpecInfo struct {
	SpecID         string `json:"spec_id"`
	SpecName       string `json:"spec_name"`
	ParentSpecName string `json:"parent_spec_name"`
}

// SkuLockInfo SKU锁定信息
type SkuLockInfo struct {
	IsForbidEdit                 bool `json:"is_forbid_edit"`
	AllowEditPersonalizationInfo bool `json:"allow_edit_personalization_info"`
}

// CheckPriceAuditInfo 价格审核信息
type CheckPriceAuditInfo struct {
	Currency                string  `json:"currency"`
	PricingType             int     `json:"pricing_type"`
	PriceAuditOrderID       string  `json:"price_audit_order_id"`
	SuggestSupplierPrice    float64 `json:"suggest_supplier_price"`
	SourceSupplierPrice     float64 `json:"source_supplier_price"`
	TargetSupplierPrice     float64 `json:"target_supplier_price"`
	SupplierPrice           float64 `json:"supplier_price"`
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

// SkuPreSaleStockInfo SKU预售库存信息
type SkuPreSaleStockInfo struct {
	PreSaleStockShowStatus         int `json:"pre_sale_stock_show_status"`
	MaximumPreSaleStock            int `json:"maximum_pre_sale_stock"`
	Version                        int `json:"version"`
	PreSaleEnableDays              int `json:"pre_sale_enable_days"`
	PreSaleStockReopenIntervalDate int `json:"pre_sale_stock_reopen_interval_date"`
}
