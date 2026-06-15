package listingkit

import (
	"math"
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
	sheinstore "task-processor/internal/shein/store"
)

const (
	defaultSheinMainSite      = "shein"
	defaultSheinSubSite       = "shein-us"
	defaultSheinWarehouseCode = "PS4833059103"
	defaultSheinSKCShelfWay   = 1
	minSheinWeightGrams       = 0.01
	maxSheinWeightGrams       = 50000000
)

func ensureSheinSubmitSites(product *sheinproduct.Product, settings SheinSettings) {
	if product == nil {
		return
	}
	defaultSites := sheinSubmitDefaultSites(settings)
	if len(product.SiteList) == 0 {
		product.SiteList = defaultSites
		return
	}
	for index := range product.SiteList {
		if strings.TrimSpace(product.SiteList[index].MainSite) == "" || strings.EqualFold(product.SiteList[index].MainSite, "US") {
			product.SiteList[index].MainSite = defaultSites[0].MainSite
		}
		if len(product.SiteList[index].SubSiteList) == 0 {
			product.SiteList[index].SubSiteList = append([]string(nil), defaultSites[0].SubSiteList...)
		}
		for subIndex, subSite := range product.SiteList[index].SubSiteList {
			if strings.EqualFold(strings.TrimSpace(subSite), "US") {
				product.SiteList[index].SubSiteList[subIndex] = defaultSites[0].SubSiteList[0]
			}
		}
	}
}

func sheinSubmitDefaultSites(settings SheinSettings) []sheinproduct.SiteInfo {
	region := strings.ToUpper(strings.TrimSpace(settings.Site))
	if region == "" {
		region = "US"
	}
	return sheinstore.GetSiteListByRegion(region)
}

func sheinSubmitPreferredWarehouseCode(settings SheinSettings) string {
	for _, item := range strings.Split(settings.WarehouseCode, ",") {
		value := strings.TrimSpace(item)
		if value != "" {
			return value
		}
	}
	return "DEFAULT"
}

func ensureSheinSubmitSKUs(product *sheinproduct.Product, settings SheinSettings) {
	if product == nil {
		return
	}
	preferredWarehouseCode := sheinSubmitPreferredWarehouseCode(settings)
	for skcIndex := range product.SKCList {
		if product.SKCList[skcIndex].SiteDetailImageInfoList == nil {
			product.SKCList[skcIndex].SiteDetailImageInfoList = []sheinproduct.SiteDetailImageInfo{}
		}
		if product.SKCList[skcIndex].SiteSpecImageInfoList == nil {
			product.SKCList[skcIndex].SiteSpecImageInfoList = []any{}
		}
		if product.SKCList[skcIndex].SKCScopeAttributeList == nil {
			product.SKCList[skcIndex].SKCScopeAttributeList = []any{}
		}
		product.SKCList[skcIndex].SupplierCode = nil
		product.SKCList[skcIndex].SaleAttribute.PreFillSpec = false
		product.SKCList[skcIndex].SaleAttribute.IsSPPSaleAttribute = false
		if product.SKCList[skcIndex].ShelfWay == 0 {
			product.SKCList[skcIndex].ShelfWay = defaultSheinSKCShelfWay
		}
		for skuIndex := range product.SKCList[skcIndex].SKUS {
			sku := &product.SKCList[skcIndex].SKUS[skuIndex]
			if sku.SaleAttributeList == nil {
				sku.SaleAttributeList = []sheinproduct.SaleAttribute{}
			}
			if len(sku.StockInfoList) == 0 {
				inventory := 1000
				if sku.StockCount != nil && *sku.StockCount > 0 {
					inventory = *sku.StockCount
				}
				sku.StockInfoList = []sheinproduct.StockInfo{{
					InventoryNum:          inventory,
					MerchantWarehouseCode: preferredWarehouseCode,
				}}
				sku.StockCount = nil
			} else {
				for stockIndex := range sku.StockInfoList {
					if strings.TrimSpace(sku.StockInfoList[stockIndex].MerchantWarehouseCode) == "" ||
						strings.EqualFold(sku.StockInfoList[stockIndex].MerchantWarehouseCode, "DEFAULT") ||
						strings.EqualFold(sku.StockInfoList[stockIndex].MerchantWarehouseCode, "US") {
						sku.StockInfoList[stockIndex].MerchantWarehouseCode = preferredWarehouseCode
					}
					if sku.StockInfoList[stockIndex].InventoryNum <= 0 {
						sku.StockInfoList[stockIndex].InventoryNum = 1000
					}
				}
				sku.StockCount = nil
			}
			if sku.QuantityInfo == nil || sku.QuantityInfo.Quantity == nil || sku.QuantityInfo.QuantityType == nil || sku.QuantityInfo.QuantityUnit == nil {
				quantity := 1
				quantityType := 1
				quantityUnit := 1
				sku.QuantityInfo = &sheinproduct.QuantityInfo{
					Quantity:     &quantity,
					QuantityType: &quantityType,
					QuantityUnit: &quantityUnit,
				}
			}
			if sku.PackageType == 0 {
				sku.PackageType = 3
			}
			if sku.CompetingCostPriceImages == nil {
				sku.CompetingCostPriceImages = []any{}
			}
			sku.StopPurchase = 1
			for priceIndex := range sku.PriceInfoList {
				if strings.EqualFold(strings.TrimSpace(sku.PriceInfoList[priceIndex].SubSite), "US") {
					sku.PriceInfoList[priceIndex].SubSite = defaultSheinSubSite
				}
			}
			ensureSheinSubmitDimensions(sku)
		}
	}
}

func ensureSheinSubmitDimensions(sku *sheinproduct.SKU) {
	if sku == nil {
		return
	}
	if strings.TrimSpace(sku.LengthUnit) == "" {
		sku.LengthUnit = "Inch"
	}
	if strings.TrimSpace(sku.Length) == "" {
		sku.Length = "1"
	}
	if strings.TrimSpace(sku.Width) == "" {
		sku.Width = "1"
	}
	if strings.TrimSpace(sku.Height) == "" {
		sku.Height = "1"
	}
	if strings.TrimSpace(sku.WeightUnit) == "" {
		sku.WeightUnit = "g"
	}
	normalizeSheinSubmitWeight(sku)
}

func normalizeSheinSubmitWeight(sku *sheinproduct.SKU) {
	if sku == nil {
		return
	}
	weight := convertSheinWeightToGrams(sku.Weight, sku.WeightUnit)
	if weight <= 0 {
		weight = minSheinWeightGrams
	}
	if weight < minSheinWeightGrams {
		weight = minSheinWeightGrams
	}
	if weight > maxSheinWeightGrams {
		weight = maxSheinWeightGrams
	}
	sku.Weight = roundSheinWeightGrams(weight)
	sku.WeightUnit = "g"
}

func convertSheinWeightToGrams(value float64, unit string) float64 {
	if value <= 0 {
		return 0
	}
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "g", "gram", "grams":
		return value
	case "kg", "kilogram", "kilograms":
		return value * 1000
	case "lb", "lbs", "pound", "pounds":
		return value * 453.59237
	case "oz", "ounce", "ounces":
		return value * 28.349523125
	case "mg", "milligram", "milligrams":
		return value / 1000
	default:
		return value
	}
}

func roundSheinWeightGrams(value float64) float64 {
	return math.Round(value*100) / 100
}
