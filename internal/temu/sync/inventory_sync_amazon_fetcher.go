// package sync 提供TEMU平台Amazon数据获取逻辑
package sync

import (
	"context"
	"fmt"

	"task-processor/internal/model"
	"task-processor/internal/product"
)

// getAmazonProductData 获取Amazon产品数据（使用ProductFetcher，自动处理缓存）- 参考SHEIN实现
func (s *inventorySyncServiceImpl) getAmazonProductData(
	ctx context.Context,
	asin, region string,
	tenantID, storeID int64,
) (*model.Product, error) {
	s.logger.WithFields(map[string]any{
		"asin":      asin,
		"region":    region,
		"tenant_id": tenantID,
		"store_id":  storeID,
	}).Debug("开始获取Amazon产品数据")

	fetchReq := &product.FetchRequest{
		TenantID:  tenantID,
		Platform:  "Amazon",
		Region:    region,
		ProductID: asin,
		StoreID:   storeID,
		Creator:   "temu_monitor",
	}

	inventoryRawJsonClient := s.managementClient.GetRawJsonDataAdapter()
	productFetcher := product.NewProductFetcher(
		inventoryRawJsonClient,
		s.amazonConfig,
		s.amazonProcessor,
	)

	amazonProduct, err := productFetcher.FetchProduct(ctx, fetchReq)
	if err != nil {
		return nil, fmt.Errorf("获取Amazon产品失败: %w", err)
	}

	s.logger.WithFields(map[string]any{
		"asin":         asin,
		"region":       region,
		"title":        amazonProduct.Title,
		"final_price":  amazonProduct.FinalPrice,
		"availability": amazonProduct.Availability,
		"is_available": amazonProduct.IsAvailable,
	}).Debug("成功获取Amazon产品数据")

	return amazonProduct, nil
}
