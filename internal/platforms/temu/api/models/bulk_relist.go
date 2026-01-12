package models

// BulkRelistOptions 批量重新上架选项
type BulkRelistOptions struct {
	DelayBetweenRequests int             `json:"delay_between_requests"`  // 请求间隔（毫秒）
	SkipConditions       *SkipConditions `json:"skip_conditions"`         // 跳过条件
	MaxConcurrency       int             `json:"max_concurrency"`         // 最大并发数
	DryRun               bool            `json:"dry_run"`                 // 是否为试运行
	ProcessFirstPageOnly bool            `json:"process_first_page_only"` // 只处理第一页（循环处理）
	PrintProductData     bool            `json:"print_product_data"`      // 是否打印详细商品数据
}

// SkipConditions 跳过条件
type SkipConditions struct {
	SkipNeedRectification bool `json:"skip_need_rectification"` // 跳过需要整改的商品
	SkipSeverelyPunished  bool `json:"skip_severely_punished"`  // 跳过被严重惩罚的商品
	SkipLocked            bool `json:"skip_locked"`             // 跳过被锁定的商品
	SkipNoStock           bool `json:"skip_no_stock"`           // 跳过无库存的商品
	MinStock              int  `json:"min_stock"`               // 最小库存要求
}

// ProductFilter 产品过滤条件
type ProductFilter struct {
	IncludeCategories []string `json:"include_categories"` // 包含的分类列表
	ExcludeCategories []string `json:"exclude_categories"` // 排除的分类列表
	NameKeywords      []string `json:"name_keywords"`      // 商品名称关键词
	MinStock          int      `json:"min_stock"`          // 最小库存
	MaxStock          int      `json:"max_stock"`          // 最大库存
	MinPrice          float64  `json:"min_price"`          // 最小价格
	MaxPrice          float64  `json:"max_price"`          // 最大价格
	HasPunishTags     *bool    `json:"has_punish_tags"`    // 是否有惩罚标签（nil表示不限制）
	NeedRectification *bool    `json:"need_rectification"` // 是否需要整改（nil表示不限制）
}

// BulkRelistSummary 批量重新上架摘要
type BulkRelistSummary struct {
	StartTime         string                   `json:"start_time"`          // 开始时间
	EndTime           string                   `json:"end_time"`            // 结束时间
	Duration          string                   `json:"duration"`            // 耗时
	TotalOfflineCount int                      `json:"total_offline_count"` // 总下架数量
	ProcessedCount    int                      `json:"processed_count"`     // 处理数量
	SuccessCount      int                      `json:"success_count"`       // 成功数量
	FailCount         int                      `json:"fail_count"`          // 失败数量
	SkippedCount      int                      `json:"skipped_count"`       // 跳过数量
	SuccessRate       float64                  `json:"success_rate"`        // 成功率
	CategoryStats     map[string]CategoryStats `json:"category_stats"`      // 分类统计
	ErrorStats        map[string]int           `json:"error_stats"`         // 错误统计
}

// CategoryStats 分类统计
type CategoryStats struct {
	Total   int `json:"total"`   // 总数
	Success int `json:"success"` // 成功数
	Failed  int `json:"failed"`  // 失败数
	Skipped int `json:"skipped"` // 跳过数
}

// RelistProgress 重新上架进度
type RelistProgress struct {
	CurrentIndex      int     `json:"current_index"`       // 当前处理索引
	TotalCount        int     `json:"total_count"`         // 总数量
	ProcessedCount    int     `json:"processed_count"`     // 已处理数量
	SuccessCount      int     `json:"success_count"`       // 成功数量
	FailCount         int     `json:"fail_count"`          // 失败数量
	SkippedCount      int     `json:"skipped_count"`       // 跳过数量
	ProgressPercent   float64 `json:"progress_percent"`    // 进度百分比
	CurrentGoodsName  string  `json:"current_goods_name"`  // 当前处理的商品名称
	EstimatedTimeLeft string  `json:"estimated_time_left"` // 预计剩余时间
}

// OfflineProductPreview 已下架产品预览
type OfflineProductPreview struct {
	TotalOfflineCount int                     `json:"total_offline_count"` // 总下架数量
	FilteredCount     int                     `json:"filtered_count"`      // 符合条件的数量
	Products          []OfflineProductSummary `json:"products"`            // 产品摘要列表
}

// OfflineProductSummary 已下架产品摘要
type OfflineProductSummary struct {
	GoodsID           string   `json:"goods_id"`           // 商品ID
	GoodsName         string   `json:"goods_name"`         // 商品名称
	Categories        []string `json:"categories"`         // 分类列表
	Stock             int      `json:"stock"`              // 库存
	Price             float64  `json:"price"`              // 价格
	Currency          string   `json:"currency"`           // 货币
	NeedRectification bool     `json:"need_rectification"` // 是否需要整改
	PunishTags        int      `json:"punish_tags"`        // 惩罚标签
	IsLocked          bool     `json:"is_locked"`          // 是否被锁定
}
