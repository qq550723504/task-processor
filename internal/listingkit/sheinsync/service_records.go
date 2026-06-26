package sheinsync

import (
	"encoding/json"
	"fmt"
	"time"

	sheinproduct "task-processor/internal/shein/api/product"
)

func buildSyncedProductRecord(
	tenantID, storeID int64,
	product sheinproduct.ProductListItem,
	skc sheinproduct.SkcInfoItem,
) *SheinSyncedProductRecord {
	now := time.Now().UTC()
	publishTime, _ := parseSheinSyncTime(product.PublishTime)
	firstShelfTime, _ := parseSheinSyncTime(product.FirstShelfTime)

	record := &SheinSyncedProductRecord{
		TenantID:         tenantID,
		StoreID:          storeID,
		SPUName:          product.SpuName,
		SPUCode:          product.SpuCode,
		SKCName:          skc.SkcName,
		SKCCode:          skc.SkcCode,
		SupplierCode:     skc.SupplierCode,
		CategoryID:       product.CategoryID,
		BrandName:        product.BrandName,
		ProductNameMulti: product.ProductNameMulti,
		MainImageURL:     skc.MainImageThumbnailURL,
		SaleName:         skc.SaleName,
		BusinessModel:    skc.BusinessModel,
		ShelfStatus:      product.ShelfStatus,
		PublishTime:      publishTime,
		FirstShelfTime:   firstShelfTime,
		SiteSnapshot:     buildSheinSiteSnapshot(product, skc),
		LastSyncAt:       &now,
		IsActive:         true,
	}
	return record
}

func buildSheinSiteSnapshot(product sheinproduct.ProductListItem, skc sheinproduct.SkcInfoItem) string {
	skuCodes := make([]string, 0, len(skc.SkuInfo))
	for _, sku := range skc.SkuInfo {
		if sku.SkuCode == "" {
			continue
		}
		skuCodes = append(skuCodes, sku.SkuCode)
	}
	payload := map[string]any{
		"spu_name":           product.SpuName,
		"spu_code":           product.SpuCode,
		"shelf_status":       product.ShelfStatus,
		"publish_time":       product.PublishTime,
		"first_shelf_time":   product.FirstShelfTime,
		"product_name_multi": product.ProductNameMulti,
		"skc_name":           skc.SkcName,
		"skc_code":           skc.SkcCode,
		"sale_name":          skc.SaleName,
		"supplier_code":      skc.SupplierCode,
		"sku_codes":          skuCodes,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func parseSheinSyncTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		time.RFC3339,
	}
	for _, format := range formats {
		parsed, err := time.Parse(format, value)
		if err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("parse SHEIN time %q", value)
}

func countUpsertedProducts(existingProducts map[string]SheinSyncedProductRecord, records []*SheinSyncedProductRecord) (int, int) {
	insertedCount := 0
	updatedCount := 0
	for _, record := range records {
		if record == nil {
			continue
		}
		if _, exists := existingProducts[record.SKCName]; exists {
			updatedCount++
			continue
		}
		insertedCount++
	}
	return insertedCount, updatedCount
}

func countDeactivatedProducts(existingProducts map[string]SheinSyncedProductRecord, activeSKCNames []string) int {
	activeSet := make(map[string]struct{}, len(activeSKCNames))
	for _, skcName := range activeSKCNames {
		activeSet[skcName] = struct{}{}
	}

	deactivatedCount := 0
	for skcName, row := range existingProducts {
		if !row.IsActive {
			continue
		}
		if _, stillActive := activeSet[skcName]; stillActive {
			continue
		}
		deactivatedCount++
	}
	return deactivatedCount
}

func cloneSheinSyncFloat64(v *float64) *float64 {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}
