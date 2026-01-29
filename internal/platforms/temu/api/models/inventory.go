// Package models 提供TEMU库存管理相关数据模型
package models

// StockEditRequest TEMU库存修改请求
type StockEditRequest struct {
	GoodsID            string           `json:"goods_id"`              // 商品ID
	SkuStockChangeList []SkuStockChange `json:"sku_stock_change_list"` // SKU库存变更列表
}

// SkuStockChange SKU库存变更信息
type SkuStockChange struct {
	SkuID                 string `json:"sku_id"`                  // SKU ID
	CurrentShippingMode   int    `json:"current_shipping_mode"`   // 当前发货模式
	CurrentStockAvailable int    `json:"current_stock_available"` // 当前可用库存
	StockDiff             int    `json:"stock_diff"`              // 库存变化量
}

// StockEditResponse TEMU库存修改响应
type StockEditResponse struct {
	Success   bool            `json:"success"`    // 是否成功
	ErrorCode int             `json:"error_code"` // 错误码
	Result    StockEditResult `json:"result"`     // 结果
}

// StockEditResult 库存修改结果
type StockEditResult struct {
	Result bool `json:"result"` // 操作结果
}

// OfflineProductRequest TEMU下架产品请求
type OfflineProductRequest struct {
	GoodsID string   `json:"goods_id"` // 商品ID
	SkuIDs  []string `json:"sku_ids"`  // SKU ID列表
}

// OfflineProductResponse TEMU下架产品响应
type OfflineProductResponse struct {
	Success   bool                 `json:"success"`    // 是否成功
	ErrorCode int                  `json:"error_code"` // 错误码
	Result    OfflineProductResult `json:"result"`     // 结果
}

// OfflineProductResult 下架产品结果
type OfflineProductResult struct {
	Result bool `json:"result"` // 操作结果
}

// OnlineProductRequest TEMU上架产品请求
type OnlineProductRequest struct {
	GoodsID string   `json:"goods_id"` // 商品ID
	SkuIDs  []string `json:"sku_ids"`  // SKU ID列表
}

// OnlineProductResponse TEMU上架产品响应
type OnlineProductResponse struct {
	Success   bool                `json:"success"`    // 是否成功
	ErrorCode int                 `json:"error_code"` // 错误码
	Result    OnlineProductResult `json:"result"`     // 结果
}

// OnlineProductResult 上架产品结果
type OnlineProductResult struct {
	Result bool   `json:"result"` // 操作结果
	Msg    string `json:"msg"`    // 消息（可选，失败时返回）
}
