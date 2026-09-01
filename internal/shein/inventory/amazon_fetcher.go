// Package inventory 提供 SHEIN 平台库存同步功能
package inventory

import (
	"context"
	"fmt"

	"task-processor/internal/marketplace/sourceproduct"
	"task-processor/internal/model"
)

// getAmazonProductData 获取Amazon产品数据（使用分布式或本地ProductFetcher，自动处理缓存）
func (s *inventorySyncServiceImpl) getAmazonProductData(
	ctx context.Context,
	asin, region string,
	tenantID, storeID int64,
) (*model.Product, error) {
	fetchReq := &sourceproduct.FetchRequest{
		TenantID:  tenantID,
		Platform:  "Amazon",
		Region:    region,
		ProductID: asin,
		StoreID:   storeID,
		Creator:   "monitor",
	}

	p, err := s.productFetcher.FetchProduct(ctx, fetchReq)
	if err != nil {
		return nil, fmt.Errorf("获取Amazon产品失败: %w", err)
	}
	return p, nil
}
