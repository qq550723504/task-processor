package models

// OfflineProductSearchRequest 已下架产品搜索请求
type OfflineProductSearchRequest struct {
	PageSize              int    `json:"page_size"`                // 每页数量，最大200
	PageNo                int    `json:"page_no"`                  // 页码，从1开始
	OrderType             int    `json:"order_type"`               // 排序类型：0-降序，1-升序
	OrderField            string `json:"order_field"`              // 排序字段，如"gmt_create"
	EnableBatchSearchText bool   `json:"enable_batch_search_text"` // 是否启用批量搜索文本
	SkuSearchType         int    `json:"sku_search_type"`          // SKU搜索类型：3-已下架产品
}

// OfflineProductSearchResponse 已下架产品搜索响应
type OfflineProductSearchResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		PageNum int                  `json:"page_num"` // 当前页码
		Total   int                  `json:"total"`    // 总数量
		SkuList []OfflineProductItem `json:"sku_list"` // 已下架产品列表
	} `json:"result"`
}

// OfflineProductItem 已下架产品项
type OfflineProductItem struct {
	GoodsName                   string                    `json:"goods_name"`                      // 商品名称
	SpecName                    string                    `json:"spec_name"`                       // 规格名称
	SpecList                    []SpecInfo                `json:"spec_list"`                       // 规格列表
	ThumbURL                    string                    `json:"thumb_url"`                       // 缩略图URL
	GoodsID                     string                    `json:"goods_id"`                        // 商品ID
	GoodsCommitID               string                    `json:"goods_commit_id"`                 // 商品提交ID
	ListingCommitID             string                    `json:"listing_commit_id"`               // 上架提交ID
	MallID                      string                    `json:"mall_id"`                         // 店铺ID
	SkuID                       string                    `json:"sku_id"`                          // SKU ID
	SkuSN                       string                    `json:"sku_sn"`                          // SKU编号
	Stock                       int                       `json:"stock"`                           // 库存
	Price                       float64                   `json:"price"`                           // 价格
	PriceVO                     PriceVO                   `json:"price_vo"`                        // 价格对象
	CrtTime                     string                    `json:"crt_time"`                        // 创建时间
	Status4VO                   int                       `json:"status4_vo"`                      // 状态4
	SubStatus4VO                int                       `json:"sub_status4_vo"`                  // 子状态4
	ClosedTypeList              []any                     `json:"closed_type_list"`                // 关闭类型列表
	SupplierPrice               float64                   `json:"supplier_price"`                  // 供应商价格
	GoodsIsOnsale               int                       `json:"goods_is_onsale"`                 // 商品是否在售
	Currency                    string                    `json:"currency"`                        // 货币
	ListPrice                   PriceVO                   `json:"list_price"`                      // 标价
	ListPriceVO                 PriceVO                   `json:"list_price_vo"`                   // 标价对象
	StatusUpdateTime            string                    `json:"status_update_time"`              // 状态更新时间
	CatType                     int                       `json:"cat_type"`                        // 分类类型
	LockInfo                    LockInfo                  `json:"lock_info"`                       // 锁定信息
	MultiSiteGoods              bool                      `json:"multi_site_goods"`                // 多站点商品
	CheckPriceAuditInfo         map[string]any            `json:"check_price_audit_info"`          // 核价审核信息
	AdjustPriceAuditInfo        AdjustPriceAuditInfo      `json:"adjust_price_audit_info"`         // 调价审核信息
	ShowSubStatus4VO            int                       `json:"show_sub_status4_vo"`             // 显示子状态4
	NewVersionApprovedLabelShow bool                      `json:"new_version_approved_label_show"` // 新版本审核标签显示
	PersonalizationStatus       int                       `json:"personalization_status"`          // 个性化状态
	PunishTags                  int                       `json:"punish_tags"`                     // 惩罚标签
	CatNameList                 []string                  `json:"cat_name_list"`                   // 分类名称列表
	CatID                       int                       `json:"cat_id"`                          // 分类ID
	CategoryRectificationInfo   CategoryRectificationInfo `json:"category_rectification_info"`     // 分类整改信息
	StockDisplayTag             int                       `json:"stock_display_tag"`               // 库存显示标签
	LowTrafficTag               int                       `json:"low_traffic_tag"`                 // 低流量标签
	RestrictedTrafficTag        int                       `json:"restricted_traffic_tag"`          // 限制流量标签
	PreSaleStockInfo            PreSaleStockInfo          `json:"pre_sale_stock_info"`             // 预售库存信息
	OrdinaryStock               int                       `json:"ordinary_stock"`                  // 普通库存
	ShippingMode                int                       `json:"shipping_mode"`                   // 发货模式
	OutGoodsSN                  string                    `json:"out_goods_sn"`                    // 外部商品编号
	EasyGainsTag                int                       `json:"easy_gains_tag"`                  // 易获利标签
	IsBooks                     bool                      `json:"is_books"`                        // 是否为书籍
	StockSearchTags             []any                     `json:"stock_search_tags"`               // 库存搜索标签
}

// CategoryRectificationInfo 分类整改信息
type CategoryRectificationInfo struct {
	NeedRectification bool  `json:"need_rectification"` // 是否需要整改
	ExpectedTime      int64 `json:"expected_time"`      // 预期时间
}

// OfflineProductStatistics 已下架产品统计信息
type OfflineProductStatistics struct {
	TotalCount             int            `json:"total_count"`              // 总数量
	NeedRectificationCount int            `json:"need_rectification_count"` // 需要整改的数量
	HasStockCount          int            `json:"has_stock_count"`          // 有库存的数量
	PunishedCount          int            `json:"punished_count"`           // 被惩罚的数量
	CategoryCount          map[string]int `json:"category_count"`           // 按分类统计
	StatusCount            map[int]int    `json:"status_count"`             // 按状态统计
}
