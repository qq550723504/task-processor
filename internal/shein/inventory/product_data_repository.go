package inventory

import (
	"context"
	"fmt"

	"task-processor/internal/listingadmin"
)

func (s *inventorySyncServiceImpl) saveInventoryProductSnapshot(ctx context.Context, prod *InventoryProductSnapshot) error {
	if prod == nil {
		return nil
	}
	if s.productDataRepo == nil {
		return fmt.Errorf("product data repository is not initialized")
	}
	if _, err := s.productDataRepo.UpsertProductDataBatch(ctx, []listingadmin.ProductData{inventoryProductDataFromSnapshot(prod)}); err != nil {
		return fmt.Errorf("通过产品数据仓储保存SHEIN产品失败: %w", err)
	}
	return nil
}

func (s *inventorySyncServiceImpl) updateInventoryProductAttributes(ctx context.Context, prod *InventoryProductSnapshot, attributes string) (int, error) {
	if prod == nil {
		return 0, nil
	}
	if s.productDataRepo == nil {
		return 0, fmt.Errorf("product data repository is not initialized")
	}
	count, err := s.productDataRepo.BatchUpdateAttributesByPlatformProductID(ctx, []listingadmin.ProductData{{
		TenantID:          prod.TenantID,
		StoreID:           sheinInvPtrInt64(prod.StoreID),
		Platform:          prod.Platform,
		PlatformProductID: prod.PlatformProductID,
		Attributes:        sheinInvRawJSONString(attributes),
	}})
	if err != nil {
		return 0, fmt.Errorf("通过产品数据仓储更新SHEIN attributes失败: %w", err)
	}
	return count, nil
}
