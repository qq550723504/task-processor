package sku

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"task-processor/internal/core/logger"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/api/attribute"
	"task-processor/internal/shein/api/product"
	"task-processor/internal/shein/validation"
)

type SKUUtils struct{}

const (
	minSheinWeightGrams = 0.01
	maxSheinWeightGrams = 50000000
)

var sheinWeightPattern = regexp.MustCompile(`^\s*([0-9]+(?:\.[0-9]+)?)\s*([a-z]+)?\s*$`)

func NewSKUUtils() *SKUUtils {
	return &SKUUtils{}
}

func (u *SKUUtils) GetAttributeName(attrID int, attributeTemplates []attribute.AttributeTemplate) string {
	for _, template := range attributeTemplates {
		for _, attrInfo := range template.AttributeInfos {
			if attrInfo.AttributeID == attrID {
				return attrInfo.AttributeNameEn
			}
		}
	}
	return ""
}

func (u *SKUUtils) GetAttributeNameAlternatives(attrID int, attributeTemplates []attribute.AttributeTemplate) []string {
	var alternatives []string
	attrName := u.GetAttributeName(attrID, attributeTemplates)
	if attrName != "" {
		alternatives = []string{strings.ToLower(attrName), strings.ToUpper(attrName)}
	}
	return alternatives
}

func (u *SKUUtils) ParseWeight(weightStr string) float64 {
	if weightStr == "" {
		return 0
	}

	normalized := strings.ToLower(strings.TrimSpace(weightStr))
	normalized = strings.ReplaceAll(normalized, ",", "")

	matches := sheinWeightPattern.FindStringSubmatch(normalized)
	if len(matches) != 3 {
		return 0
	}

	weight, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}

	unit := matches[2]
	switch unit {
	case "", "g", "gram", "grams":
		return math.Round(weight*100) / 100
	case "kg", "kilogram", "kilograms":
		weight *= 1000
	case "lb", "lbs", "pound", "pounds":
		weight *= 453.59237
	case "oz", "ounce", "ounces":
		weight *= 28.349523125
	case "mg", "milligram", "milligrams":
		weight /= 1000
	default:
		return 0
	}
	return math.Round(weight*100) / 100
}

func (u *SKUUtils) NormalizeWeightForShein(weight float64) float64 {
	if weight <= 0 {
		return minSheinWeightGrams
	}
	if weight < minSheinWeightGrams {
		weight = minSheinWeightGrams
	}
	if weight > maxSheinWeightGrams {
		weight = maxSheinWeightGrams
	}
	return math.Round(weight*100) / 100
}

func (u *SKUUtils) FormatPriceByCurrency(price float64, currency string) float64 {
	switch currency {
	case "JPY", "KRW":
		return float64(int(price))
	default:
		return price
	}
}

func (u *SKUUtils) BuildStockInfoList(stockCount int, warehouseCode string) []product.StockInfo {
	return []product.StockInfo{{
		InventoryNum:          stockCount,
		MerchantWarehouseCode: warehouseCode,
	}}
}

func (u *SKUUtils) BuildQuantityInfo(params shein.SKUCreationParams) *product.QuantityInfo {
	quantity := 1
	quantityType := 1
	quantityUnit := 1

	if params.Variant.Quantity > 0 {
		quantity = params.Variant.Quantity
	}
	if params.Variant.QuantityType > 0 {
		quantityType = params.Variant.QuantityType
	}
	if params.Variant.UnitType > 0 {
		quantityUnit = params.Variant.UnitType
	}

	quantityType, quantity = u.CorrectQuantityTypeAndValue(quantityType, quantity, params.ASIN)
	validator := validation.NewQuantityValidator()

	correctUnit, err := validator.GetCorrectQuantityUnit(quantityType)
	if err != nil {
		logger.GetGlobalLogger("shein/product").Warnf("failed to get correct quantity unit: %v, keep unit %d", err, quantityUnit)
	} else if quantityUnit != correctUnit {
		logger.GetGlobalLogger("shein/product").Warnf("quantity unit mismatched rule, quantityType=%d should use %d, current=%d, auto-corrected", quantityType, correctUnit, quantityUnit)
		quantityUnit = correctUnit
	}

	if err := validator.ValidateQuantityMapping(quantityType, quantityUnit); err != nil {
		logger.GetGlobalLogger("shein/product").Errorf("quantity mapping validation failed: %v", err)
	}
	if err := validator.ValidateQuantity(quantity, quantityType); err != nil {
		logger.GetGlobalLogger("shein/product").Errorf("quantity validation failed: %v", err)
	}

	return &product.QuantityInfo{
		Quantity:     &quantity,
		QuantityType: &quantityType,
		QuantityUnit: &quantityUnit,
	}
}

func (u *SKUUtils) BuildSKUImageInfoForMultiPiece(ctx *shein.TaskContext, params shein.SKUCreationParams) *product.ImageInfo {
	return u.BuildSKUImageInfoForMultiPieceWithRuntime(newRuntimeInput(ctx), params)
}

func (u *SKUUtils) BuildSKUImageInfoForMultiPieceWithRuntime(runtime *RuntimeInput, params shein.SKUCreationParams) *product.ImageInfo {
	if runtime == nil || runtime.ImageAPI == nil {
		return nil
	}

	var skuImages []product.ImageDetail
	var sourceImages []string

	if imageURL, exists := params.Variant.Attributes["image"]; exists && imageURL != "" {
		sourceImages = append(sourceImages, imageURL)
	}

	if len(sourceImages) == 0 && params.ProductInfo != nil && len(params.ProductInfo.Images) > 0 {
		maxImages := 1
		if len(params.ProductInfo.Images) < maxImages {
			maxImages = len(params.ProductInfo.Images)
		}
		sourceImages = params.ProductInfo.Images[:maxImages]
	}

	if len(sourceImages) == 0 {
		return nil
	}

	for _, imageURL := range sourceImages {
		if imageURL == "" {
			continue
		}

		uploadedURL, err := runtime.ImageAPI.DownloadAndUploadImage(imageURL)
		if err != nil {
			logger.GetGlobalLogger("shein/product").Warnf("failed to upload multipart SKU image, ASIN=%s, url=%s, err=%v", params.ASIN, imageURL, err)
			continue
		}

		if uploadedURL != "" {
			skuImages = append(skuImages, product.ImageDetail{
				ImageURL:  uploadedURL,
				ImageSort: 1,
				ImageType: 1,
			})
			break
		}
	}

	if len(skuImages) == 0 {
		logger.GetGlobalLogger("shein/product").Warnf("all multipart SKU images failed to upload, ASIN=%s", params.ASIN)
		return nil
	}

	logger.GetGlobalLogger("shein/product").Infof("built %d SKU images for multipart ASIN=%s", len(skuImages), params.ASIN)
	return &product.ImageInfo{ImageInfoList: skuImages}
}

func (u *SKUUtils) CorrectQuantityTypeAndValue(quantityType, quantity int, asin string) (int, int) {
	validator := validation.NewQuantityValidator()
	if err := validator.ValidateQuantity(quantity, quantityType); err == nil {
		return quantityType, quantity
	}

	logger.GetGlobalLogger("shein/product").Warnf("ASIN %s quantity rule mismatch: quantityType=%d quantity=%d", asin, quantityType, quantity)

	if quantityType == 2 && quantity == 1 {
		logger.GetGlobalLogger("shein/product").Infof("ASIN %s corrected multipart-single to single item", asin)
		return 1, 1
	}
	if quantityType == 4 && quantity == 1 {
		logger.GetGlobalLogger("shein/product").Infof("ASIN %s corrected multi-set-single to single set", asin)
		return 3, 1
	}
	if (quantityType == 1 || quantityType == 3) && quantity > 1 {
		if quantityType == 1 {
			logger.GetGlobalLogger("shein/product").Infof("ASIN %s corrected single item to multipart, quantity=%d", asin, quantity)
			return 2, quantity
		}
		logger.GetGlobalLogger("shein/product").Infof("ASIN %s corrected single set to multi-set, quantity=%d", asin, quantity)
		return 4, quantity
	}
	if (quantityType == 2 || quantityType == 4) && quantity < 2 {
		logger.GetGlobalLogger("shein/product").Infof("ASIN %s corrected quantity from %d to 2 for quantityType=%d", asin, quantity, quantityType)
		return quantityType, 2
	}

	logger.GetGlobalLogger("shein/product").Warnf("ASIN %s fallback to default quantity setting", asin)
	return 1, 1
}
