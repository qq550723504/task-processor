package listingkit

import (
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
	attribute "task-processor/internal/shein/api/attribute"
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

func prepareSheinProductForNewSubmit(product *sheinproduct.Product) {
	prepareSheinProductForSubmit(product, SheinSettings{
		Site:          "US",
		WarehouseCode: "DEFAULT",
	})
}

func prepareSheinProductForSubmit(product *sheinproduct.Product, settings SheinSettings) {
	if product == nil {
		return
	}
	// SHEIN generates spu_name for new products. Sending a display title here
	// makes the product API reject the draft/publish request.
	product.SPUName = ""
	if strings.TrimSpace(product.PointKey) == "" {
		product.PointKey = uuid.NewString()
	}
	product.SourceSystem = "listingkit"
	product.SupplierCode = deriveSheinSubmitProductSupplierCode(product)
	normalizeSheinSubmitCollections(product)
	ensureSheinSubmitSites(product, settings)
	ensureSheinSubmitSKUs(product, settings)
	normalizeSheinSubmitImages(product)
	normalizeSheinSubmitExtra(product)
	finalizeSheinSubmitTransportFields(product)
}

func normalizeSheinSubmitCollections(product *sheinproduct.Product) {
	if product == nil {
		return
	}
	if product.BrandSeriesList == nil {
		product.BrandSeriesList = []string{}
	}
	if product.MultiLanguageMakeupIngredientList == nil {
		product.MultiLanguageMakeupIngredientList = []any{}
	}
	if product.ProductVideoList == nil {
		product.ProductVideoList = []sheinproduct.ProductVideo{}
	}
	if product.PartInfoList == nil {
		product.PartInfoList = []any{}
	}
	if product.PLMPatternIDList == nil {
		product.PLMPatternIDList = []any{}
	}
	if product.SizeAttributeList == nil {
		product.SizeAttributeList = []sheinproduct.SizeAttribute{}
	}
	if product.BackSizeAttributeList == nil {
		product.BackSizeAttributeList = []any{}
	}
	if product.ProductCertificateList == nil {
		product.ProductCertificateList = []int{}
	}
	if product.CertificateList == nil {
		product.CertificateList = []int{}
	}
	if product.DelOtherCertificateSNList == nil {
		product.DelOtherCertificateSNList = []string{}
	}
	if product.CustomAttributeRelation == nil {
		product.CustomAttributeRelation = []attribute.CustomAttributeRelation{}
	}
}

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

func normalizeSheinSubmitImages(product *sheinproduct.Product) {
	if product == nil {
		return
	}
	if product.ImageInfo != nil {
		product.ImageInfo.ImageInfoList = normalizeSheinSubmitSPUImages(product.ImageInfo.ImageInfoList)
		if product.ImageInfo.OriginalImageInfoList == nil {
			empty := []any{}
			product.ImageInfo.OriginalImageInfoList = &empty
		}
	}
	product.Extra.SwitchToSPUPic = false
	for skcIndex := range product.SKCList {
		skc := &product.SKCList[skcIndex]
		normalizeSheinSubmitSKCImages(skc)
		normalizeSheinSubmitSKUImages(skc)
	}
}

func normalizeSheinSubmitSPUImages(images []sheinproduct.ImageDetail) []sheinproduct.ImageDetail {
	normalized := normalizeSheinSubmitGalleryImages(images, false)
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeSheinSubmitExtra(product *sheinproduct.Product) {
	if product == nil {
		return
	}
	fromPageID := "product_publish"
	product.Extra.FromPageID = &fromPageID
	product.Extra.SwitchToSPUPic = false
	product.Extra.UseCVTransformImage = false
	product.Extra.TransformCVSizeImage = false
}

func finalizeSheinSubmitTransportFields(product *sheinproduct.Product) {
	if product == nil {
		return
	}
	if product.Extra.SPUTag == nil {
		product.Extra.SPUTag = []string{}
	}
	if product.Extra.ConfirmVolumeSKU == nil {
		product.Extra.ConfirmVolumeSKU = []string{}
	}
	if product.Extra.ConfirmWeightSKU == nil {
		product.Extra.ConfirmWeightSKU = []string{}
	}
	if product.Extra.ControlPriceData == nil {
		product.Extra.ControlPriceData = map[string]string{}
	}
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
		if skc.ProofOfStockList == nil {
			skc.ProofOfStockList = []any{}
		}
		for skuIndex := range skc.SKUS {
			sku := &skc.SKUS[skuIndex]
			if sku.CompetingCostPriceImages == nil {
				sku.CompetingCostPriceImages = []any{}
			}
		}
	}
}

func deriveSheinSubmitProductSupplierCode(product *sheinproduct.Product) string {
	if product == nil {
		return ""
	}
	if value := strings.TrimSpace(product.SupplierCode); value != "" && !looksLikeRawBaseSupplierCode(value) {
		return value
	}
	for _, skc := range product.SKCList {
		for _, sku := range skc.SKUS {
			if value := deriveSheinSubmitSupplierCodeFromSKU(sku.SupplierSKU); value != "" {
				return value
			}
		}
	}
	return strings.TrimSpace(product.SupplierCode)
}

func deriveSheinSubmitSupplierCodeFromSKU(supplierSKU string) string {
	supplierSKU = strings.TrimSpace(strings.ToUpper(supplierSKU))
	if supplierSKU == "" {
		return ""
	}
	parts := strings.Split(supplierSKU, "-")
	if len(parts) < 2 {
		return supplierSKU
	}
	styleSuffix := normalizeSheinSubmitStyleSuffix(parts[len(parts)-1])
	if styleSuffix == "" {
		return supplierSKU
	}
	baseSKU := strings.TrimSpace(parts[0])
	if baseSKU == "" {
		return ""
	}
	return baseSKU + "-" + styleSuffix
}

func normalizeSheinSubmitStyleSuffix(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
		if b.Len() >= 8 {
			break
		}
	}
	return b.String()
}

func looksLikeRawBaseSupplierCode(value string) bool {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return false
	}
	parts := strings.Split(value, "-")
	return len(parts) == 1
}

func normalizeSheinSubmitSKUImages(skc *sheinproduct.SKC) {
	if skc == nil {
		return
	}
	var fallback sheinproduct.ImageDetail
	hasFallback := false
	if len(skc.ImageInfo.ImageInfoList) > 0 {
		fallback = skc.ImageInfo.ImageInfoList[0]
		hasFallback = strings.TrimSpace(fallback.ImageURL) != ""
	}
	for skuIndex := range skc.SKUS {
		info := skc.SKUS[skuIndex].ImageInfo
		if info == nil || len(info.ImageInfoList) == 0 {
			if !hasFallback {
				continue
			}
			skc.SKUS[skuIndex].ImageInfo = &sheinproduct.ImageInfo{
				ImageInfoList: []sheinproduct.ImageDetail{normalizeSheinSubmitSKUImageDetail(fallback)},
			}
			empty := []any{}
			skc.SKUS[skuIndex].ImageInfo.OriginalImageInfoList = &empty
			continue
		}
		info.ImageInfoList = dedupeSheinImagesByURL(info.ImageInfoList)
		if len(info.ImageInfoList) > 0 {
			info.ImageInfoList = []sheinproduct.ImageDetail{normalizeSheinSubmitSKUImageDetail(info.ImageInfoList[0])}
		}
		if info.OriginalImageInfoList == nil {
			empty := []any{}
			info.OriginalImageInfoList = &empty
		}
	}
}

func normalizeSheinSubmitSKUImageDetail(image sheinproduct.ImageDetail) sheinproduct.ImageDetail {
	image.ImageType = 1
	image.ImageSort = 1
	image.MarketingMainImage = false
	image.SizeImgFlag = false
	image.TransformCVSizeImage = false
	if image.PSTypes == nil {
		image.PSTypes = []string{}
	}
	return image
}

func normalizeSheinSubmitSKCImages(skc *sheinproduct.SKC) {
	if skc == nil || len(skc.ImageInfo.ImageInfoList) == 0 {
		return
	}
	skc.ImageInfo.ImageInfoList = normalizeSheinSubmitGalleryImages(skc.ImageInfo.ImageInfoList, true)
	if skc.ImageInfo.OriginalImageInfoList == nil {
		empty := []any{}
		skc.ImageInfo.OriginalImageInfoList = &empty
	}
}

func normalizeSheinSubmitGalleryImages(images []sheinproduct.ImageDetail, includeColorBlock bool) []sheinproduct.ImageDetail {
	source := dedupeSheinImagesByURL(images)
	if len(source) == 0 {
		return nil
	}
	colorBlockSource := source[0]
	for _, image := range source {
		if image.ImageType == 6 && !image.SizeImgFlag && strings.TrimSpace(image.ImageURL) != "" {
			colorBlockSource = image
			break
		}
	}
	gallerySource := make([]sheinproduct.ImageDetail, 0, len(source))
	for _, image := range source {
		if image.ImageType == 6 && !image.SizeImgFlag {
			continue
		}
		gallerySource = append(gallerySource, image)
	}
	if len(gallerySource) == 0 {
		gallerySource = []sheinproduct.ImageDetail{source[0]}
	}
	extraCapacity := 1
	if includeColorBlock {
		extraCapacity = 2
	}
	normalized := make([]sheinproduct.ImageDetail, 0, len(gallerySource)+extraCapacity)
	for index, image := range gallerySource {
		image.ImageType = 2
		if index == 0 {
			image.ImageType = 1
		}
		image.ImageSort = index + 1
		image.MarketingMainImage = false
		image.SizeImgFlag = false
		image.TransformCVSizeImage = false
		if image.PSTypes == nil {
			image.PSTypes = []string{}
		}
		normalized = append(normalized, image)
	}
	square := gallerySource[0]
	square.ImageType = 5
	square.ImageSort = len(normalized) + 1
	square.MarketingMainImage = false
	square.SizeImgFlag = false
	square.TransformCVSizeImage = false
	if square.PSTypes == nil {
		square.PSTypes = []string{}
	}
	normalized = append(normalized, square)
	if !includeColorBlock {
		return normalized
	}
	colorBlock := colorBlockSource
	colorBlock.ImageType = 6
	colorBlock.ImageSort = len(normalized) + 1
	colorBlock.MarketingMainImage = false
	colorBlock.SizeImgFlag = false
	colorBlock.TransformCVSizeImage = false
	if colorBlock.PSTypes == nil {
		colorBlock.PSTypes = []string{}
	}
	normalized = append(normalized, colorBlock)
	return normalized
}

func dedupeSheinImagesByURL(images []sheinproduct.ImageDetail) []sheinproduct.ImageDetail {
	seen := map[string]bool{}
	result := make([]sheinproduct.ImageDetail, 0, len(images))
	for _, image := range images {
		url := strings.TrimSpace(image.ImageURL)
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		result = append(result, image)
	}
	return result
}

func validateSheinProductPublishPayload(product *sheinproduct.Product) error {
	if product == nil {
		return fmt.Errorf("SHEIN publish payload is empty")
	}
	for skcIndex, skc := range product.SKCList {
		if len(skc.ImageInfo.ImageInfoList) == 0 {
			return fmt.Errorf("SHEIN publish blocked: SKC[%d] has no images", skcIndex)
		}
		hasSquare := false
		hasColorBlock := false
		for _, image := range skc.ImageInfo.ImageInfoList {
			switch image.ImageType {
			case 5:
				hasSquare = true
			case 6:
				hasColorBlock = true
			}
		}
		if !hasSquare {
			return fmt.Errorf("SHEIN publish blocked: SKC[%d] is missing required square image", skcIndex)
		}
		if !hasColorBlock {
			return fmt.Errorf("SHEIN publish blocked: SKC[%d] is missing required color block image", skcIndex)
		}
	}
	return nil
}
