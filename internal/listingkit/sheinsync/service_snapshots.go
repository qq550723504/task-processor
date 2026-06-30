package sheinsync

import (
	"context"
	"encoding/json"

	sheinproduct "task-processor/internal/shein/api/product"
)

type sheinProductSnapshots struct {
	priceSnapshot         string
	priceSnapshotBySKC    map[string]string
	inventorySnapshot     string
	inventorySKUInfoBySKC map[string][]sheinSiteSnapshotSKUInfo
	priceLoaded           bool
	inventoryLoaded       bool
}

func (s *sheinSyncService) fetchSupplementalSnapshots(
	_ context.Context,
	productAPI sheinproduct.ProductAPI,
	product sheinproduct.ProductListItem,
) sheinProductSnapshots {
	snapshots := sheinProductSnapshots{}
	if productAPI == nil || product.SpuName == "" {
		return snapshots
	}

	if priceResp, err := productAPI.QueryPrice(product.SpuName); err == nil {
		snapshots.priceLoaded = true
		snapshots.priceSnapshot = buildSheinPriceSnapshot(priceResp, product)
		snapshots.priceSnapshotBySKC = buildSheinPriceSnapshotsBySKC(priceResp, product)
	}

	if inventoryResp, err := productAPI.QueryInventory(product.SpuName); err == nil {
		snapshots.inventoryLoaded = true
		snapshots.inventorySnapshot = buildSheinInventorySnapshot(inventoryResp, product)
		snapshots.inventorySKUInfoBySKC = buildSheinInventorySKUInfoBySKC(inventoryResp)
	}

	return snapshots
}

func (s sheinProductSnapshots) priceSnapshotForSKC(skcName string) string {
	if s.priceSnapshotBySKC != nil {
		return s.priceSnapshotBySKC[skcName]
	}
	return s.priceSnapshot
}

func buildSheinInventorySKUInfoBySKC(
	response *sheinproduct.InventoryQueryResponse,
) map[string][]sheinSiteSnapshotSKUInfo {
	if response == nil {
		return nil
	}
	out := make(map[string][]sheinSiteSnapshotSKUInfo, len(response.Info.SkcInfo))
	for _, skcInventory := range response.Info.SkcInfo {
		if skcInventory.SkcName == "" {
			continue
		}
		items := make([]sheinSiteSnapshotSKUInfo, 0, len(skcInventory.SkuInfo))
		for _, skuInventory := range skcInventory.SkuInfo {
			if skuInventory.SkuCode == "" {
				continue
			}
			items = append(items, sheinSiteSnapshotSKUInfo{
				SKUCode:      skuInventory.SkuCode,
				VariantLabel: sheinInventorySKUVariantLabel(skuInventory.SaleNameInfo),
				SaleNameInfo: sheinSiteSnapshotSaleNameInfoFromInventory(skuInventory.SaleNameInfo),
			})
		}
		if len(items) > 0 {
			out[skcInventory.SkcName] = items
		}
	}
	return out
}

func buildSheinPriceSnapshot(
	response *sheinproduct.PriceQueryResponse,
	product sheinproduct.ProductListItem,
) string {
	for _, skc := range product.SkcInfoList {
		if snapshot := buildSheinPriceSnapshotForSKC(response, skc.SkcName); snapshot != "" {
			return snapshot
		}
	}
	return ""
}

func buildSheinPriceSnapshotsBySKC(
	response *sheinproduct.PriceQueryResponse,
	product sheinproduct.ProductListItem,
) map[string]string {
	if response == nil {
		return nil
	}
	out := make(map[string]string, len(product.SkcInfoList))
	for _, skc := range product.SkcInfoList {
		if skc.SkcName == "" {
			continue
		}
		if snapshot := buildSheinPriceSnapshotForSKC(response, skc.SkcName); snapshot != "" {
			out[skc.SkcName] = snapshot
		}
	}
	return out
}

func buildSheinPriceSnapshotForSKC(
	response *sheinproduct.PriceQueryResponse,
	skcName string,
) string {
	if response == nil {
		return ""
	}

	skcByName := make(map[string]sheinproduct.SkcPriceData, len(response.Info.Data))
	for _, skcPrice := range response.Info.Data {
		skcByName[skcPrice.SkcName] = skcPrice
	}

	skcPrice, ok := skcByName[skcName]
	if !ok {
		return ""
	}

	skuPrices := make([]map[string]any, 0, len(skcPrice.SkuInfoList))
	var firstSalePrice float64
	firstCurrency := ""
	firstSubSite := ""
	for _, skuPrice := range skcPrice.SkuInfoList {
		for _, detail := range skuPrice.PriceInfoList {
			salePrice := detail.SpecialPrice
			if salePrice <= 0 {
				salePrice = detail.ShopPrice
			}
			if salePrice <= 0 {
				continue
			}
			if firstSalePrice <= 0 {
				firstSalePrice = salePrice
				firstCurrency = detail.Currency
				firstSubSite = detail.SubSite
			}
			skuPrices = append(skuPrices, map[string]any{
				"sku_code":   skuPrice.SkuCode,
				"sale_price": salePrice,
				"currency":   detail.Currency,
				"sub_site":   detail.SubSite,
			})
			break
		}
	}
	if firstSalePrice <= 0 {
		return ""
	}

	payload := map[string]any{
		"sale_price": firstSalePrice,
		"currency":   firstCurrency,
		"sub_site":   firstSubSite,
		"sku_prices": skuPrices,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func buildSheinInventorySnapshot(
	response *sheinproduct.InventoryQueryResponse,
	product sheinproduct.ProductListItem,
) string {
	if response == nil {
		return ""
	}

	skcByName := make(map[string]sheinproduct.SkcInventory, len(response.Info.SkcInfo))
	for _, skcInventory := range response.Info.SkcInfo {
		skcByName[skcInventory.SkcName] = skcInventory
	}

	for _, skc := range product.SkcInfoList {
		skcInventory, ok := skcByName[skc.SkcName]
		if !ok {
			continue
		}

		total := 0
		available := 0
		for _, skuInventory := range skcInventory.SkuInfo {
			for _, warehouse := range skuInventory.InventoryInfo {
				total += warehouse.InventoryQuantity
				available += warehouse.UsableInventory
			}
		}

		payload := map[string]any{
			"total":     total,
			"available": available,
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return ""
		}
		return string(encoded)
	}

	return ""
}
