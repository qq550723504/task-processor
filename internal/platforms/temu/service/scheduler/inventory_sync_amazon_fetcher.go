// Package scheduler 提供TEMU平台Amazon数据获取逻辑
package scheduler

import (
	"context"
	"fmt"

	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
)

// getAmazonProductData 获取Amazon产品数据（使用ProductFetcher，自动处理缓存）- 参考SHEIN实现
func (s *inventorySyncServiceImpl) getAmazonProductData(
	ctx context.Context,
	asin, region string,
	tenantID, storeID int64,
) (*model.Product, error) {
	s.logger.WithFields(map[string]interface{}{
		"asin":      asin,
		"region":    region,
		"tenant_id": tenantID,
		"store_id":  storeID,
	}).Debug("开始获取Amazon产品数据")

	// 使用 ProductFetcher 获取产品（自动处理缓存和爬取）
	fetchReq := &product.FetchRequest{
		TenantID:  tenantID,
		Platform:  "Amazon",
		Region:    region,
		ProductID: asin,
		StoreID:   storeID,
		Creator:   "temu_monitor", // TEMU库存监控标识
	}

	// 为TEMU库存监控创建专用的 rawJsonDataClient，设置24小时数据新鲜度
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
				resultChan <- fetchResult{nil, fmt.Errorf("获取Amazon产品时发生panic: %v", r)}
			}
		}()

		s.logger.WithField("asin", asin).Debug("开始调用ProductFetcher获取Amazon产品")
		amazonProduct, err := productFetcher.FetchProduct(fetchReq)
		if err != nil {
			s.logger.WithError(err).WithField("asin", asin).Warn("ProductFetcher获取Amazon产品失败")
		} else {
			s.logger.WithFields(map[string]interface{}{
				"asin":  asin,
				"title": amazonProduct.Title,
				"price": amazonProduct.FinalPrice,
			}).Debug("ProductFetcher成功获取Amazon产品")
		}
		resultChan <- fetchResult{amazonProduct, err}
	}()

	// 等待结果或超时
	select {
	case <-ctx.Done():
		s.logger.WithFields(map[string]interface{}{
			"asin":   asin,
			"region": region,
			"error":  ctx.Err(),
		}).Warn("获取Amazon产品超时")
		return nil, fmt.Errorf("获取Amazon产品超时: %w", ctx.Err())
	case result := <-resultChan:
		if result.err != nil {
			return nil, fmt.Errorf("获取Amazon产品失败: %w", result.err)
		}

		s.logger.WithFields(map[string]interface{}{
			"asin":         asin,
			"region":       region,
			"title":        result.product.Title,
			"final_price":  result.product.FinalPrice,
			"availability": result.product.Availability,
			"is_available": result.product.IsAvailable,
		}).Debug("成功获取Amazon产品数据")

		return result.product, nil
	}
}
