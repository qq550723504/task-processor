package models

// RelistProductRequest 重新上架产品请求
type RelistProductRequest struct {
	GoodsID string   `json:"goods_id"` // 商品ID
	SkuIDs  []string `json:"sku_ids"`  // SKU ID列表
}

// RelistProductResponse 重新上架产品响应
type RelistProductResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		Result bool `json:"result"` // true表示上架成功，false表示上架失败或有限制
	} `json:"result"`
}

// DelistProductRequest 下架产品请求
type DelistProductRequest struct {
	GoodsID         string   `json:"goods_id"`         // 商品ID
	SkuIDs          []string `json:"sku_ids"`          // SKU ID列表
	OperationSource int      `json:"operation_source"` // 操作来源，如1005表示价格健康页面
}

// DelistProductResponse 下架产品响应
type DelistProductResponse struct {
	Success   bool     `json:"success"`
	ErrorCode int      `json:"error_code"`
	Result    struct{} `json:"result"`
}

// ProductListingInfo 产品上架信息
type ProductListingInfo struct {
	GoodsID string   `json:"goods_id"` // 商品ID
	SkuIDs  []string `json:"sku_ids"`  // SKU ID列表
}

// ListingOperationResult 上架操作结果
type ListingOperationResult struct {
	GoodsID string   `json:"goods_id"` // 商品ID
	SkuIDs  []string `json:"sku_ids"`  // SKU ID列表
	Success bool     `json:"success"`  // 操作是否成功
	Error   string   `json:"error"`    // 错误信息（如果失败）
}

// BatchListingResult 批量上架结果
type BatchListingResult struct {
	TotalCount   int                      `json:"total_count"`   // 总数量
	SuccessCount int                      `json:"success_count"` // 成功数量
	FailCount    int                      `json:"fail_count"`    // 失败数量
	Results      []ListingOperationResult `json:"results"`       // 详细结果
}

// RelistConditions 重新上架条件
type RelistConditions struct {
	RequireStock             bool     `json:"require_stock"`              // 是否要求有库存
	ExcludeNeedRectification bool     `json:"exclude_need_rectification"` // 是否排除需要整改的产品
	ExcludePunished          bool     `json:"exclude_punished"`           // 是否排除被惩罚的产品
	IncludeCategories        []string `json:"include_categories"`         // 包含的分类列表（为空表示不限制）
	ExcludeCategories        []string `json:"exclude_categories"`         // 排除的分类列表
	MinStock                 int      `json:"min_stock"`                  // 最小库存要求
	MaxDaysOffline           int      `json:"max_days_offline"`           // 最大下架天数
}

// ListingOperationType 上架操作类型
type ListingOperationType string

const (
	ListingOperationRelist ListingOperationType = "relist" // 重新上架
	ListingOperationDelist ListingOperationType = "delist" // 下架
)

// ListingOperationRequest 上架操作请求
type ListingOperationRequest struct {
	Operation ListingOperationType     `json:"operation"` // 操作类型
	Products  []ProductListingInfo     `json:"products"`  // 产品列表
	Options   *ListingOperationOptions `json:"options"`   // 操作选项
}

// ListingOperationOptions 上架操作选项
type ListingOperationOptions struct {
	OperationSource int    `json:"operation_source"` // 操作来源
	BatchSize       int    `json:"batch_size"`       // 批处理大小
	DelayMs         int    `json:"delay_ms"`         // 操作间隔（毫秒）
	Reason          string `json:"reason"`           // 操作原因
}

// RelistAllResult 全部重新上架结果
type RelistAllResult struct {
	TotalOfflineCount int                  `json:"total_offline_count"` // 总的已下架产品数量
	ProcessedCount    int                  `json:"processed_count"`     // 已处理的商品数量
	SuccessCount      int                  `json:"success_count"`       // 成功上架数量
	FailCount         int                  `json:"fail_count"`          // 失败数量
	SkippedCount      int                  `json:"skipped_count"`       // 跳过数量
	Results           []RelistDetailResult `json:"results"`             // 详细结果列表
}

// RelistDetailResult 重新上架详细结果
type RelistDetailResult struct {
	GoodsID   string   `json:"goods_id"`   // 商品ID
	GoodsName string   `json:"goods_name"` // 商品名称
	SkuIDs    []string `json:"sku_ids"`    // SKU ID列表
	SkuCount  int      `json:"sku_count"`  // SKU数量
	Success   bool     `json:"success"`    // 是否成功
	Skipped   bool     `json:"skipped"`    // 是否跳过
	Error     string   `json:"error"`      // 错误信息或跳过原因
}
