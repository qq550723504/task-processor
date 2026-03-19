// Package inventory 提供 SHEIN 平台库存同步功能
package inventory

import (
	"context"
	"fmt"

	"task-processor/internal/model"
	domainproduct "task-processor/internal/product"
)

// getAmazonProductData 获取Amazon产品数据（使用ProductFetcher，自动处理缓存）
func (s *inventorySyncServiceImpl) getAmazonProductData(
	ctx context.Context,
	asin, region string,
	tenantID, storeID int64,
) (*model.Product, error) {
	fetchReq := &domainproduct.FetchRequest{
		TenantID:  tenantID,
		Platform:  "Amazon",
		Region:    region,
		ProductID: asin,
		StoreID:   storeID,
		Creator:   "monitor",
	}

	inventoryRawJsonClient := s.managementClient.GetRawJsonDataAdapter()
	productFetcher := domainproduct.NewProductFetcher(
		inventoryRawJsonClient,
		s.amazonConfig,
		s.amazonProcessor,
	)

	product, err := productFetcher.FetchProduct(ctx, fetchReq)
	if err != nil {
		return nil, fmt.Errorf("获取Amazon产品失败: %w", err)
	}
	return product, nil
}
