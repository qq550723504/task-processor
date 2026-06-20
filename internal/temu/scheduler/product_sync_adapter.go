// Package scheduler 提供TEMU产品同步任务的适配器
package scheduler

import (
	"context"
	"fmt"

	platformtask "task-processor/internal/platformtask"
	models "task-processor/internal/temu/api/product"
	temuscheduler "task-processor/internal/temu/sync"
)

// productSyncServiceAdapter 适配器，将TEMU特定的ProductSyncService适配到通用接口
type productSyncServiceAdapter struct {
	temuService temuscheduler.ProductSyncService
}

// newProductSyncServiceAdapter 创建产品同步服务适配器
func newProductSyncServiceAdapter(temuService temuscheduler.ProductSyncService) platformtask.ProductSyncService {
	return &productSyncServiceAdapter{
		temuService: temuService,
	}
}

// FetchProductList 获取产品列表（适配到通用接口）
func (a *productSyncServiceAdapter) FetchProductList(ctx context.Context) ([]any, error) {
	products, err := a.temuService.FetchProductList(ctx)
	if err != nil {
		return nil, err
	}

	// 转换为interface{}切片
	result := make([]any, len(products))
	for i, p := range products {
		result[i] = p
	}
	return result, nil
}

// ConvertProducts 转换产品格式（适配到通用接口）
func (a *productSyncServiceAdapter) ConvertProducts(ctx context.Context, products []any, tenantID, storeID int64) ([]any, error) {
	// 转换回TEMU特定类型
	temuProducts := make([]models.GoodsSearchItem, len(products))
	for i, p := range products {
		if tp, ok := p.(models.GoodsSearchItem); ok {
			temuProducts[i] = tp
		}
	}

	// 调用TEMU服务
	converted, err := a.temuService.ConvertProducts(ctx, temuProducts, tenantID, storeID)
	if err != nil {
		return nil, err
	}

	// 转换为interface{}切片
	result := make([]any, len(converted))
	for i, p := range converted {
		result[i] = p
	}
	return result, nil
}

// SaveProducts 保存产品（适配到通用接口）
func (a *productSyncServiceAdapter) SaveProducts(ctx context.Context, products []any) (int, error) {
	// 转换回TEMU特定类型
	productDataList := make([]*temuscheduler.TemuProductSnapshot, len(products))
	for i, p := range products {
		pd, ok := p.(*temuscheduler.TemuProductSnapshot)
		if !ok {
			return 0, fmt.Errorf("products[%d] 类型断言失败: 期望 *TemuProductSnapshot, 实际 %T", i, p)
		}
		productDataList[i] = pd
	}

	// 调用TEMU服务
	return a.temuService.SaveProducts(ctx, productDataList)
}
