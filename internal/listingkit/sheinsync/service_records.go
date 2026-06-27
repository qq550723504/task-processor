package sheinsync

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sheinproduct "task-processor/internal/shein/api/product"
)

type sheinSiteSnapshotSKUInfo struct {
	SKUCode      string                          `json:"sku_code,omitempty"`
	SupplierSKU  string                          `json:"supplier_sku,omitempty"`
	VariantLabel string                          `json:"variant_label,omitempty"`
	SaleNameInfo []sheinSiteSnapshotSaleNameInfo `json:"sale_name_info,omitempty"`
}

type sheinSiteSnapshotSaleNameInfo struct {
	SaleAttrName string `json:"sale_attr_name,omitempty"`
	SaleName     string `json:"sale_name,omitempty"`
}

func buildSyncedProductRecord(
	tenantID, storeID int64,
	product sheinproduct.ProductListItem,
	skc sheinproduct.SkcInfoItem,
	snapshots sheinProductSnapshots,
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
		SiteSnapshot:     buildSheinSiteSnapshot(product, skc, snapshots),
		LastSyncAt:       &now,
		IsActive:         true,
	}
	return record
}

func buildSheinSiteSnapshot(product sheinproduct.ProductListItem, skc sheinproduct.SkcInfoItem, snapshots sheinProductSnapshots) string {
	skuInfo := make([]sheinSiteSnapshotSKUInfo, 0, len(skc.SkuInfo))
	bySKUCode := make(map[string]int)
	appendInfo := func(info sheinSiteSnapshotSKUInfo) {
		info.SKUCode = strings.TrimSpace(info.SKUCode)
		info.SupplierSKU = strings.TrimSpace(info.SupplierSKU)
		info.VariantLabel = strings.TrimSpace(info.VariantLabel)
		if info.SKUCode == "" && info.SupplierSKU == "" && info.VariantLabel == "" {
			return
		}
		key := strings.ToUpper(info.SKUCode)
		if key != "" {
			if index, ok := bySKUCode[key]; ok {
				if skuInfo[index].SupplierSKU == "" {
					skuInfo[index].SupplierSKU = info.SupplierSKU
				}
				if skuInfo[index].VariantLabel == "" {
					skuInfo[index].VariantLabel = info.VariantLabel
				}
				if len(skuInfo[index].SaleNameInfo) == 0 {
					skuInfo[index].SaleNameInfo = info.SaleNameInfo
				}
				return
			}
			bySKUCode[key] = len(skuInfo)
		}
		skuInfo = append(skuInfo, info)
	}
	for _, sku := range skc.SkuInfo {
		appendInfo(sheinSiteSnapshotSKUInfo{SKUCode: sku.SkuCode, SupplierSKU: sku.SupplierSKU})
	}
	for _, info := range snapshots.inventorySKUInfoBySKC[skc.SkcName] {
		appendInfo(info)
	}
	skuCodes := make([]string, 0, len(skuInfo))
	for _, info := range skuInfo {
		if info.SKUCode != "" {
			skuCodes = append(skuCodes, info.SKUCode)
		}
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
		"sku_info":           skuInfo,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func sheinInventorySKUVariantLabel(saleNameInfo []sheinproduct.SkuSaleNameInfo) string {
	if len(saleNameInfo) == 0 {
		return ""
	}
	labels := make([]string, 0, len(saleNameInfo))
	fallback := make([]string, 0, len(saleNameInfo))
	for _, item := range saleNameInfo {
		name := strings.TrimSpace(item.SaleName)
		if name == "" {
			continue
		}
		fallback = append(fallback, name)
		if sheinInventorySaleAttrIsColor(item.SaleAttrName) {
			continue
		}
		labels = append(labels, name)
	}
	if len(labels) == 0 {
		labels = fallback
	}
	return strings.Join(labels, " / ")
}

func sheinSiteSnapshotSaleNameInfoFromInventory(items []sheinproduct.SkuSaleNameInfo) []sheinSiteSnapshotSaleNameInfo {
	out := make([]sheinSiteSnapshotSaleNameInfo, 0, len(items))
	for _, item := range items {
		saleName := strings.TrimSpace(item.SaleName)
		saleAttrName := strings.TrimSpace(item.SaleAttrName)
		if saleName == "" && saleAttrName == "" {
			continue
		}
		out = append(out, sheinSiteSnapshotSaleNameInfo{
			SaleAttrName: saleAttrName,
			SaleName:     saleName,
		})
	}
	return out
}

func sheinInventorySaleAttrIsColor(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "color", "colour", "颜色", "顏色":
		return true
	default:
		return false
	}
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
