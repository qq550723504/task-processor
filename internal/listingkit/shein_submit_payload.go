package listingkit

import (
	"fmt"
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
)

const (
	defaultSheinMainSite      = "shein"
	defaultSheinSubSite       = "shein-us"
	defaultSheinWarehouseCode = "PS4833059103"
)

func prepareSheinProductForNewSubmit(product *sheinproduct.Product) {
	if product == nil {
		return
	}
	// SHEIN generates spu_name for new products. Sending a display title here
	// makes the product API reject the draft/publish request.
	product.SPUName = ""
	ensureSheinSubmitSites(product)
	ensureSheinSubmitSKUs(product)
	normalizeSheinSubmitImages(product)
}

func ensureSheinSubmitSites(product *sheinproduct.Product) {
	if product == nil {
		return
	}
	if len(product.SiteList) == 0 {
		product.SiteList = []sheinproduct.SiteInfo{{
			MainSite:    defaultSheinMainSite,
			SubSiteList: []string{defaultSheinSubSite},
		}}
		return
	}
	for index := range product.SiteList {
		if strings.TrimSpace(product.SiteList[index].MainSite) == "" || strings.EqualFold(product.SiteList[index].MainSite, "US") {
			product.SiteList[index].MainSite = defaultSheinMainSite
		}
		if len(product.SiteList[index].SubSiteList) == 0 {
			product.SiteList[index].SubSiteList = []string{defaultSheinSubSite}
		}
		for subIndex, subSite := range product.SiteList[index].SubSiteList {
			if strings.EqualFold(strings.TrimSpace(subSite), "US") {
				product.SiteList[index].SubSiteList[subIndex] = defaultSheinSubSite
			}
		}
	}
}

func ensureSheinSubmitSKUs(product *sheinproduct.Product) {
	if product == nil {
		return
	}
	for skcIndex := range product.SKCList {
		for skuIndex := range product.SKCList[skcIndex].SKUS {
			sku := &product.SKCList[skcIndex].SKUS[skuIndex]
			if len(sku.StockInfoList) == 0 {
				inventory := 1000
				if sku.StockCount != nil && *sku.StockCount > 0 {
					inventory = *sku.StockCount
				}
				sku.StockInfoList = []sheinproduct.StockInfo{{
					InventoryNum:          inventory,
					MerchantWarehouseCode: defaultSheinWarehouseCode,
				}}
				sku.StockCount = nil
			} else {
				for stockIndex := range sku.StockInfoList {
					if strings.TrimSpace(sku.StockInfoList[stockIndex].MerchantWarehouseCode) == "" ||
						strings.EqualFold(sku.StockInfoList[stockIndex].MerchantWarehouseCode, "DEFAULT") ||
						strings.EqualFold(sku.StockInfoList[stockIndex].MerchantWarehouseCode, "US") {
						sku.StockInfoList[stockIndex].MerchantWarehouseCode = defaultSheinWarehouseCode
					}
					if sku.StockInfoList[stockIndex].InventoryNum <= 0 {
						sku.StockInfoList[stockIndex].InventoryNum = 1000
					}
				}
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
}

func normalizeSheinSubmitImages(product *sheinproduct.Product) {
	if product == nil {
		return
	}
	product.ImageInfo = &sheinproduct.ImageInfo{}
	product.Extra.SwitchToSPUPic = false
	product.Extra.TransformCVSizeImage = false
	product.Extra.UseCVTransformImage = false
	for skcIndex := range product.SKCList {
		skc := &product.SKCList[skcIndex]
		normalizeSheinSubmitSKCImages(skc)
		for skuIndex := range skc.SKUS {
			skc.SKUS[skuIndex].ImageInfo = &sheinproduct.ImageInfo{}
		}
	}
}

func normalizeSheinSubmitSKCImages(skc *sheinproduct.SKC) {
	if skc == nil || len(skc.ImageInfo.ImageInfoList) == 0 {
		return
	}
	source := dedupeSheinImagesByURL(skc.ImageInfo.ImageInfoList)
	if len(source) == 0 {
		return
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
	limit := len(gallerySource)
	normalized := make([]sheinproduct.ImageDetail, 0, limit+2)
	for index := 0; index < limit; index++ {
		image := gallerySource[index]
		image.ImageType = 2
		if index == 0 {
			image.ImageType = 1
		}
		image.ImageSort = index + 1
		image.MarketingMainImage = false
		image.SizeImgFlag = false
		image.TransformCVSizeImage = false
		normalized = append(normalized, image)
	}
	square := gallerySource[0]
	square.ImageType = 5
	square.ImageSort = len(normalized) + 1
	square.MarketingMainImage = false
	square.SizeImgFlag = false
	square.TransformCVSizeImage = false
	normalized = append(normalized, square)
	colorBlock := colorBlockSource
	colorBlock.ImageType = 6
	colorBlock.ImageSort = len(normalized) + 1
	colorBlock.MarketingMainImage = false
	colorBlock.SizeImgFlag = false
	colorBlock.TransformCVSizeImage = false
	normalized = append(normalized, colorBlock)
	skc.ImageInfo.ImageInfoList = normalized
	if skc.ImageInfo.OriginalImageInfoList == nil {
		empty := []any{}
		skc.ImageInfo.OriginalImageInfoList = &empty
	}
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
