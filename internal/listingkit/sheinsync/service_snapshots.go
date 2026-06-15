package sheinsync

import (
	"context"
	"encoding/json"

	sheinproduct "task-processor/internal/shein/api/product"
)

type sheinProductSnapshots struct {
	priceSnapshot     string
	inventorySnapshot string
	priceLoaded       bool
	inventoryLoaded   bool
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
	}

	if inventoryResp, err := productAPI.QueryInventory(product.SpuName); err == nil {
		snapshots.inventoryLoaded = true
		snapshots.inventorySnapshot = buildSheinInventorySnapshot(inventoryResp, product)
	}

	return snapshots
}

func buildSheinPriceSnapshot(
	response *sheinproduct.PriceQueryResponse,
	product sheinproduct.ProductListItem,
) string {
	if response == nil {
		return ""
	}

	skcByName := make(map[string]sheinproduct.SkcPriceData, len(response.Info.Data))
	for _, skcPrice := range response.Info.Data {
		skcByName[skcPrice.SkcName] = skcPrice
	}

	for _, skc := range product.SkcInfoList {
		skcPrice, ok := skcByName[skc.SkcName]
		if !ok {
			continue
		}
		for _, skuPrice := range skcPrice.SkuInfoList {
			for _, detail := range skuPrice.PriceInfoList {
				salePrice := detail.SpecialPrice
				if salePrice <= 0 {
					salePrice = detail.ShopPrice
				}
				if salePrice <= 0 {
					continue
				}

				payload := map[string]any{
					"sale_price": salePrice,
					"currency":   detail.Currency,
					"sub_site":   detail.SubSite,
				}
				encoded, err := json.Marshal(payload)
				if err != nil {
					return ""
				}
				return string(encoded)
			}
		}
	}

	return ""
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
