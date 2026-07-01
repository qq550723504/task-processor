package sheinsync

import (
	"context"
	"encoding/json"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	sheinproduct "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/productsync"
)

func (s *sheinSyncService) buildInventorySyncAttributes(ctx context.Context, tenantID, storeID int64, skc sheinproduct.SkcInfoItem, snapshots sheinProductSnapshots) string {
	enrichedSKC := productsync.EnrichedSkcInfo{
		SkcName:               skc.SkcName,
		SkcCode:               skc.SkcCode,
		SaleName:              skc.SaleName,
		MainImageThumbnailURL: skc.MainImageThumbnailURL,
		SupplierCode:          skc.SupplierCode,
		BusinessModel:         skc.BusinessModel,
		IsSaleAttribute:       skc.IsSaleAttribute,
		SupplierID:            skc.SupplierID,
		SkuInfo:               make([]productsync.EnrichedSkuInfo, 0, len(skc.SkuInfo)),
		MallSellStatus:        skc.MallSellStatus,
		Abandoned:             skc.Abandoned,
		TagInfoList:           skc.TagInfoList,
		ShelfFailReason:       skc.ShelfFailReason,
		HasOriginalImage:      skc.HasOriginalImage,
	}
	inventoryBySKU := snapshots.inventoryInfoBySKCAndSKU(skc.SkcName)
	for _, sku := range skc.SkuInfo {
		enrichedSKU := productsync.EnrichedSkuInfo{SkuInfo: sku}
		if mapping := s.findInventoryMapping(ctx, tenantID, storeID, sku.SkuCode); mapping != nil {
			enrichedSKU.MappingInfo = mapping
		}
		if inventoryInfo, ok := inventoryBySKU[sku.SkuCode]; ok {
			enrichedSKU.InventoryInfo = inventoryInfo
			totalUsable := 0
			totalInventory := 0
			for _, warehouse := range inventoryInfo {
				totalUsable += warehouse.UsableInventory
				totalInventory += warehouse.InventoryQuantity
			}
			enrichedSKU.UsableInventory = &totalUsable
			enrichedSKU.InventoryQuantity = &totalInventory
		}
		enrichedSKC.SkuInfo = append(enrichedSKC.SkuInfo, enrichedSKU)
	}
	if len(enrichedSKC.SkuInfo) == 0 {
		return ""
	}
	payload, err := json.Marshal([]productsync.EnrichedSkcInfo{enrichedSKC})
	if err != nil {
		return ""
	}
	return string(payload)
}

func (s *sheinSyncService) findInventoryMapping(ctx context.Context, tenantID, storeID int64, skuCode string) *listingruntime.ProductImportMapping {
	if s.inventoryMappingSource == nil || skuCode == "" {
		return nil
	}
	mapping, err := s.inventoryMappingSource.FindLatest(ctx, listingadmin.ProductImportMappingQuery{
		TenantID:          tenantID,
		StoreID:           &storeID,
		Platform:          "shein",
		PlatformProductID: skuCode,
	})
	if err != nil || mapping == nil {
		return nil
	}
	return listingRuntimeMappingFromAdminMapping(mapping)
}

func listingRuntimeMappingFromAdminMapping(mapping *listingadmin.ProductImportMapping) *listingruntime.ProductImportMapping {
	if mapping == nil {
		return nil
	}
	return &listingruntime.ProductImportMapping{
		ID:                      mapping.ID,
		ImportTaskID:            mapping.ImportTaskID,
		StoreID:                 mapping.StoreID,
		Platform:                mapping.Platform,
		Region:                  mapping.Region,
		ProductID:               mapping.ProductID,
		ParentProductID:         sheinSyncStringPtr(mapping.ParentProductID),
		SKU:                     sheinSyncStringPtr(mapping.SKU),
		PlatformProductID:       sheinSyncStringPtr(mapping.PlatformProductID),
		PlatformParentProductID: sheinSyncStringPtr(mapping.PlatformParentProductID),
		CostPrice:               sheinSyncFloat64Value(mapping.CostPrice),
		FilterRuleID:            sheinSyncInt64Value(mapping.FilterRuleID),
		FilterRuleRange:         sheinSyncStringPtr(mapping.FilterRuleRange),
		ProfitRuleID:            sheinSyncInt64Value(mapping.ProfitRuleID),
		SalePriceMultiplier:     sheinSyncFloat64Ptr(mapping.SalePriceMultiplier),
		DiscountPriceMultiplier: sheinSyncFloat64Ptr(mapping.DiscountPriceMultiplier),
		Status:                  mapping.Status,
		Remark:                  sheinSyncStringPtr(mapping.Remark),
		TenantID:                mapping.TenantID,
	}
}

func sheinSyncStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	out := value
	return &out
}

func sheinSyncFloat64Ptr(value float64) *float64 {
	if value == 0 {
		return nil
	}
	out := value
	return &out
}

func sheinSyncFloat64Value(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func sheinSyncInt64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
