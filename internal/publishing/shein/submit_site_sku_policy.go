package shein

import (
	"math"
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
	sheinstore "task-processor/internal/shein/store"
)

const (
	defaultSubmitSubSite       = "shein-us"
	defaultSubmitSKCShelfWay   = 1
	minSubmitWeightGrams       = 0.01
	maxSubmitWeightGrams       = 50000000
	defaultSubmitInventory     = 1000
	defaultSubmitWarehouseCode = "DEFAULT"
)

// SubmitPayloadSettings carries marketplace payload defaults needed during submit normalization.
type SubmitPayloadSettings struct {
	Site          string
	WarehouseCode string
}

// EnsureSubmitSites normalizes SHEIN submit site fields for a product payload.
func EnsureSubmitSites(product *sheinproduct.Product, settings SubmitPayloadSettings) {
	if product == nil {
		return
	}
	defaultSites := submitDefaultSites(settings)
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

// EnsureSubmitSKUs normalizes SKU, stock, quantity, and dimensional fields for submit.
func EnsureSubmitSKUs(product *sheinproduct.Product, settings SubmitPayloadSettings) {
	if product == nil {
		return
	}
	preferredWarehouseCode := SubmitPreferredWarehouseCode(settings)
	for skcIndex := range product.SKCList {
		skc := &product.SKCList[skcIndex]
		if skc.SiteDetailImageInfoList == nil {
			skc.SiteDetailImageInfoList = []sheinproduct.SiteDetailImageInfo{}
		}
		if skc.SiteSpecImageInfoList == nil {
			skc.SiteSpecImageInfoList = []any{}
		}
		if skc.SKCScopeAttributeList == nil {
			skc.SKCScopeAttributeList = []any{}
		}
		skc.SupplierCode = nil
		skc.SaleAttribute.PreFillSpec = false
		skc.SaleAttribute.IsSPPSaleAttribute = false
		if skc.ShelfWay == 0 {
			skc.ShelfWay = defaultSubmitSKCShelfWay
		}
		for skuIndex := range skc.SKUS {
			sku := &skc.SKUS[skuIndex]
			if sku.SaleAttributeList == nil {
				sku.SaleAttributeList = []sheinproduct.SaleAttribute{}
			}
			ensureSubmitStockInfo(sku, preferredWarehouseCode)
			ensureSubmitQuantityInfo(sku)
			if sku.PackageType == 0 {
				sku.PackageType = 3
			}
			if sku.CompetingCostPriceImages == nil {
				sku.CompetingCostPriceImages = []any{}
			}
			sku.StopPurchase = 1
			for priceIndex := range sku.PriceInfoList {
				if strings.EqualFold(strings.TrimSpace(sku.PriceInfoList[priceIndex].SubSite), "US") {
					sku.PriceInfoList[priceIndex].SubSite = defaultSubmitSubSite
				}
			}
			ensureSubmitDimensions(sku)
		}
	}
}

// SubmitPreferredWarehouseCode returns the first configured warehouse code or the SHEIN default sentinel.
func SubmitPreferredWarehouseCode(settings SubmitPayloadSettings) string {
	for _, item := range strings.Split(settings.WarehouseCode, ",") {
		value := strings.TrimSpace(item)
		if value != "" {
			return value
		}
	}
	return defaultSubmitWarehouseCode
}

// NormalizeSubmitWeight converts SKU weight to grams and clamps it to SHEIN submit bounds.
func NormalizeSubmitWeight(sku *sheinproduct.SKU) {
	if sku == nil {
		return
	}
	weight := convertSubmitWeightToGrams(sku.Weight, sku.WeightUnit)
	if weight <= 0 {
		weight = minSubmitWeightGrams
	}
	if weight < minSubmitWeightGrams {
		weight = minSubmitWeightGrams
	}
	if weight > maxSubmitWeightGrams {
		weight = maxSubmitWeightGrams
	}
	sku.Weight = roundSubmitWeightGrams(weight)
	sku.WeightUnit = "g"
}

func submitDefaultSites(settings SubmitPayloadSettings) []sheinproduct.SiteInfo {
	region := strings.ToUpper(strings.TrimSpace(settings.Site))
	if region == "" {
		region = "US"
	}
	return sheinstore.GetSiteListByRegion(region)
}

func ensureSubmitStockInfo(sku *sheinproduct.SKU, preferredWarehouseCode string) {
	if len(sku.StockInfoList) == 0 {
		inventory := defaultSubmitInventory
		if sku.StockCount != nil && *sku.StockCount > 0 {
			inventory = *sku.StockCount
		}
		sku.StockInfoList = []sheinproduct.StockInfo{{
			InventoryNum:          inventory,
			MerchantWarehouseCode: preferredWarehouseCode,
		}}
		sku.StockCount = nil
		return
	}
	for stockIndex := range sku.StockInfoList {
		if strings.TrimSpace(sku.StockInfoList[stockIndex].MerchantWarehouseCode) == "" ||
			strings.EqualFold(sku.StockInfoList[stockIndex].MerchantWarehouseCode, defaultSubmitWarehouseCode) ||
			strings.EqualFold(sku.StockInfoList[stockIndex].MerchantWarehouseCode, "US") {
			sku.StockInfoList[stockIndex].MerchantWarehouseCode = preferredWarehouseCode
		}
		if sku.StockInfoList[stockIndex].InventoryNum <= 0 {
			sku.StockInfoList[stockIndex].InventoryNum = defaultSubmitInventory
		}
	}
	sku.StockCount = nil
}

func ensureSubmitQuantityInfo(sku *sheinproduct.SKU) {
	if sku.QuantityInfo != nil && sku.QuantityInfo.Quantity != nil && sku.QuantityInfo.QuantityType != nil && sku.QuantityInfo.QuantityUnit != nil {
		return
	}
	quantity := 1
	quantityType := 1
	quantityUnit := 1
	sku.QuantityInfo = &sheinproduct.QuantityInfo{
		Quantity:     &quantity,
		QuantityType: &quantityType,
		QuantityUnit: &quantityUnit,
	}
}

func ensureSubmitDimensions(sku *sheinproduct.SKU) {
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
	NormalizeSubmitWeight(sku)
}

func convertSubmitWeightToGrams(value float64, unit string) float64 {
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

func roundSubmitWeightGrams(value float64) float64 {
	return math.Round(value*100) / 100
}
