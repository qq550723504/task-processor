// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"context"
	"fmt"

	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
)

// getAmazonProductData 获取Amazon产品数据（使用ProductFetcher，自动处理缓存）
func (s *inventorySyncServiceImpl) getAmazonProductData(
	ctx context.Context,
	asin, region string,
	tenantID, storeID int64,
) (*model.Product, error) {
	// 使用 ProductFetcher 获取产品（自动处理缓存和爬取）
	fetchReq := &product.FetchRequest{
		TenantID:  tenantID,
		Platform:  "Amazon",
		Region:    region,
		ProductID: asin,
		StoreID:   storeID,
		Creator:   "monitor",
	}

	// 为库存监控创建专用的 rawJsonDataClient，设置24小时数据新鲜度
	inventoryRawJsonClient := s.managementClient.GetRawJsonDataClient()
	inventoryRawJsonClient.SetDataFreshnessDays(1) // 24小时 = 1天

	productFetcher := product.NewProductFetcher(
		inventoryRawJsonClient,
		s.amazonConfig,
		s.amazonProcessor,
	)

	// 使用 channel 实现超时控制
	type fetchResult struct {
		product *model.Product
		err     error
	}
	resultChan := make(chan fetchResult, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.WithField("panic", r).Error("获取Amazon产品时发生panic")
				resultChan <- fetchResult{nil, fmt.Errorf("获取产品时发生panic: %v", r)}
			}
		}()

		amazonProduct, err := productFetcher.FetchProduct(fetchReq)
		resultChan <- fetchResult{amazonProduct, err}
	}()

	// 等待结果或超时
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("获取Amazon产品超时: %w", ctx.Err())
	case result := <-resultChan:
		if result.err != nil {
			return nil, fmt.Errorf("获取Amazon产品失败: %w", result.err)
		}
		return result.product, nil
	}
}
