// Package scheduler 提供TEMU价格同步相关类型定义
package scheduler

import (
	"context"
	"task-processor/internal/platforms/shein/api/product"
)

// PriceSyncService TEMU价格同步服务接口
type PriceSyncService interface {
	// SyncPrice 同步价格
	SyncPrice(ctx context.Context, productIDs []string) error

	// GetPriceStatus 获取价格状态
	GetPriceStatus(ctx context.Context, productID string) (*product.PriceInfo, error)

	// UpdatePriceBatch 批量更新价格
	UpdatePriceBatch(ctx context.Context, updates []PriceUpdate) error

	// CalculatePrice 计算价格
	CalculatePrice(ctx context.Context, productID string, strategy PricingStrategy) (*product.PriceInfo, error)
}

// PriceUpdate 价格更新信息
type PriceUpdate struct {
	ProductID   string  `json:"product_id"`
	SalePrice   float64 `json:"sale_price"`
	MarketPrice float64 `json:"market_price"`
	CostPrice   float64 `json:"cost_price"`
}

// PricingStrategy 定价策略
type PricingStrategy struct {
	Type      string  `json:"type"`       // "fixed", "markup", "competitive"
	Markup    float64 `json:"markup"`     // 加价率
	MinProfit float64 `json:"min_profit"` // 最小利润
	MaxPrice  float64 `json:"max_price"`  // 最高价格
}

// PriceSyncConfig 价格同步配置
type PriceSyncConfig struct {
	BatchSize       int             `json:"batch_size"`
	EnableAutoSync  bool            `json:"enable_auto_sync"`
	SyncInterval    int             `json:"sync_interval"` // 秒
	DefaultStrategy PricingStrategy `json:"default_strategy"`
}
