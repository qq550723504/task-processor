// package sync 提供TEMU平台调度器相关服务的类型定义
package sync

import (
	"context"

	temuproduct "task-processor/internal/temu/api/product"
)

// ProductSyncService TEMU产品同步服务接口
type ProductSyncService interface {
	// FetchProductList 获取TEMU产品列表
	FetchProductList(ctx context.Context) ([]temuproduct.GoodsSearchItem, error)

	// ConvertProducts 转换TEMU产品格式为管理系统格式
	ConvertProducts(ctx context.Context, products []temuproduct.GoodsSearchItem, tenantID, storeID int64) ([]*TemuProductSnapshot, error)

	// SaveProducts 保存产品到管理系统
	SaveProducts(ctx context.Context, productDataList []*TemuProductSnapshot) (int, error)
}

// ProductSyncConfig 产品同步配置
type ProductSyncConfig struct {
	PageSize        int    `json:"page_size"`
	MaxPages        int    `json:"max_pages"`
	Language        string `json:"language"`
	IncludeInactive bool   `json:"include_inactive"`
}

// ProductSyncStats 产品同步统计
type ProductSyncStats struct {
	TotalFetched   int `json:"total_fetched"`
	TotalConverted int `json:"total_converted"`
	TotalSaved     int `json:"total_saved"`
	TotalFailed    int `json:"total_failed"`
}
