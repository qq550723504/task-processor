// Package bulkrelist 提供TEMU平台批量重新上架相关的业务类型定义
package bulkrelist

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

// ProductFilterOptions 产品过滤条件
type ProductFilterOptions struct {
	IncludeCategories []string `json:"include_categories"`
	ExcludeCategories []string `json:"exclude_categories"`
	NameKeywords      []string `json:"name_keywords"`
	MinStock          int      `json:"min_stock"`
	MaxStock          int      `json:"max_stock"`
	MinPrice          float64  `json:"min_price"`
	MaxPrice          float64  `json:"max_price"`
	HasPunishTags     *bool    `json:"has_punish_tags"`
	NeedRectification *bool    `json:"need_rectification"`
}

// RelistAllResult 全部重新上架结果
type RelistAllResult struct {
	TotalOfflineCount int                  `json:"total_offline_count"`
	ProcessedCount    int                  `json:"processed_count"`
	SuccessCount      int                  `json:"success_count"`
	FailCount         int                  `json:"fail_count"`
	SkippedCount      int                  `json:"skipped_count"`
	Results           []RelistDetailResult `json:"results"`
}

// RelistDetailResult 重新上架详细结果
type RelistDetailResult struct {
	GoodsID   string   `json:"goods_id"`
	GoodsName string   `json:"goods_name"`
	SkuIDs    []string `json:"sku_ids"`
	SkuCount  int      `json:"sku_count"`
	Success   bool     `json:"success"`
	Skipped   bool     `json:"skipped"`
	Error     string   `json:"error"`
}

// OfflineProductPreview 已下架产品预览
type OfflineProductPreview struct {
	TotalOfflineCount int                     `json:"total_offline_count"`
	FilteredCount     int                     `json:"filtered_count"`
	Products          []OfflineProductSummary `json:"products"`
}

// OfflineProductSummary 已下架产品摘要
type OfflineProductSummary struct {
	GoodsID           string   `json:"goods_id"`
	GoodsName         string   `json:"goods_name"`
	Categories        []string `json:"categories"`
	Stock             int      `json:"stock"`
	Price             float64  `json:"price"`
	Currency          string   `json:"currency"`
	NeedRectification bool     `json:"need_rectification"`
	PunishTags        int      `json:"punish_tags"`
	IsLocked          bool     `json:"is_locked"`
}
