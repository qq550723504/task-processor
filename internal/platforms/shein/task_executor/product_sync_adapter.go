// Package scheduler 提供SHEIN产品同步任务的适配器
package scheduler

import (
	"context"

	managementapi "task-processor/internal/pkg/management/api"
	commonscheduler "task-processor/internal/platforms/common/scheduler"
	"task-processor/internal/platforms/shein/api/product"
	sheinscheduler "task-processor/internal/platforms/shein/service/business_service"
)

// productSyncServiceAdapter 适配器，将SHEIN特定的ProductSyncService适配到通用接口
type productSyncServiceAdapter struct {
	sheinService sheinscheduler.ProductSyncService
}

// newProductSyncServiceAdapter 创建产品同步服务适配器
func newProductSyncServiceAdapter(sheinService sheinscheduler.ProductSyncService) commonscheduler.ProductSyncService {
	return &productSyncServiceAdapter{
		sheinService: sheinService,
	}
}

// FetchProductList 获取产品列表（适配到通用接口）
func (a *productSyncServiceAdapter) FetchProductList(ctx context.Context) ([]interface{}, error) {
	products, err := a.sheinService.FetchProductList(ctx)
	if err != nil {
		return nil, err
	}

	// 转换为interface{}切片
	result := make([]interface{}, len(products))
	for i, p := range products {
		result[i] = p
	}
	return result, nil
}

// ConvertProducts 转换产品格式（适配到通用接口）
func (a *productSyncServiceAdapter) ConvertProducts(ctx context.Context, products []interface{}, tenantID, storeID int64) ([]interface{}, error) {
	// 转换回SHEIN特定类型
	sheinProducts := make([]product.ProductListItem, len(products))
	for i, p := range products {
		if sp, ok := p.(product.ProductListItem); ok {
			sheinProducts[i] = sp
		}
	}

	// 调用SHEIN服务
	converted, err := a.sheinService.ConvertProducts(ctx, sheinProducts, tenantID, storeID)
	if err != nil {
		return nil, err
	}

	// 转换为interface{}切片
	result := make([]interface{}, len(converted))
	for i, p := range converted {
		result[i] = p
	}
	return result, nil
}

// SaveProducts 保存产品（适配到通用接口）
func (a *productSyncServiceAdapter) SaveProducts(ctx context.Context, products []interface{}) (int, error) {
	// 转换回SHEIN特定类型
	productDataList := make([]*managementapi.ProductDataDTO, len(products))
	for i, p := range products {
		if pd, ok := p.(*managementapi.ProductDataDTO); ok {
			productDataList[i] = pd
		}
	}

	// 调用SHEIN服务
	return a.sheinService.SaveProducts(ctx, productDataList)
}
