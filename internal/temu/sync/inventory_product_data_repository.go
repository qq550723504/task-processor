// package sync 提供 TEMU 库存同步相关的产品数据仓储写入能力
package sync

import (
	"context"
	"fmt"

	"task-processor/internal/listingadmin"
)

func (s *inventorySyncServiceImpl) saveInventoryProductSnapshot(ctx context.Context, prod *TemuInventoryProductSnapshot) error {
	if prod == nil {
		return nil
	}
	if s.productDataRepo != nil {
		if _, err := s.productDataRepo.UpsertProductDataBatch(ctx, []listingadmin.ProductData{temuInventoryProductDataFromSnapshot(prod)}); err != nil {
			return fmt.Errorf("通过产品数据仓储保存TEMU产品失败: %w", err)
		}
		return nil
	}
	if s.runtime == nil {
		return fmt.Errorf("inventory sync runtime is not initialized")
	}
	productDataAPI := s.runtime.GetProductDataClient(prod.StoreID)
	if productDataAPI == nil {
		return fmt.Errorf("product data client is not initialized for store %d", prod.StoreID)
	}
	if _, err := productDataAPI.BatchCreateOrUpdate(prod.toBatchSaveReq()); err != nil {
		return fmt.Errorf("通过管理客户端保存TEMU产品失败: %w", err)
	}
	return nil
}

func (s *inventorySyncServiceImpl) updateInventoryProductAttributes(ctx context.Context, prod *TemuInventoryProductSnapshot, attributes string) (int, error) {
	if prod == nil {
		return 0, nil
	}
	if s.productDataRepo != nil {
		count, err := s.productDataRepo.BatchUpdateAttributesByPlatformProductID(ctx, []listingadmin.ProductData{{
			TenantID:          prod.TenantID,
			StoreID:           temuSyncPtrInt64(prod.StoreID),
			Platform:          prod.Platform,
			PlatformProductID: prod.PlatformProductID,
			Attributes:        temuSyncRawJSONString(attributes),
		}})
		if err != nil {
			return 0, fmt.Errorf("通过产品数据仓储更新TEMU attributes失败: %w", err)
		}
		return count, nil
	}
	if s.runtime == nil {
		return 0, fmt.Errorf("inventory sync runtime is not initialized")
	}
	productDataAPI := s.runtime.GetProductDataClient(prod.StoreID)
	if productDataAPI == nil {
		return 0, fmt.Errorf("product data client is not initialized for store %d", prod.StoreID)
	}
	updateReq := prod.toBatchUpdateAttributesReq(attributes)
	count, err := productDataAPI.BatchUpdateAttributes(updateReq)
	if err != nil {
		return 0, fmt.Errorf("通过管理客户端更新TEMU attributes失败: %w", err)
	}
	return count, nil
}
