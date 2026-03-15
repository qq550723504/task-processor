// Package modules 提供SHEIN平台SKU构建工具方法
package sku

import (
	"strconv"
	"strings"
	"task-processor/internal/platforms/shein/api/attribute"
	"task-processor/internal/platforms/shein/api/product"
	"task-processor/internal/platforms/shein/model"
	"task-processor/internal/platforms/shein/validation"

	"github.com/sirupsen/logrus"
)

// SKUUtils SKU工具类
type SKUUtils struct{}

// NewSKUUtils 创建SKU工具类
func NewSKUUtils() *SKUUtils {
	return &SKUUtils{}
}

// GetAttributeName 获取属性名称
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

// GetAttributeNameAlternatives 获取属性名称的替代形式
func (u *SKUUtils) GetAttributeNameAlternatives(attrID int, attributeTemplates []attribute.AttributeTemplate) []string {
	var alternatives []string

	// 从模板中获取原始名称的变体
	attrName := u.GetAttributeName(attrID, attributeTemplates)
	if attrName != "" {
		alternatives = []string{
			strings.ToLower(attrName),
			strings.ToUpper(attrName),
		}
	}

	return alternatives
}

// ParseWeight 解析重量字符串
func (u *SKUUtils) ParseWeight(weightStr string) float64 {
	if weightStr == "" {
		return 0
	}

	// 移除单位并解析数字
	weightStr = strings.TrimSpace(weightStr)
	weightStr = strings.ReplaceAll(weightStr, "g", "")
	weightStr = strings.ReplaceAll(weightStr, "kg", "")
	weightStr = strings.ReplaceAll(weightStr, "lb", "")
	weightStr = strings.ReplaceAll(weightStr, "oz", "")
	weightStr = strings.TrimSpace(weightStr)

	if weight, err := strconv.ParseFloat(weightStr, 64); err == nil {
		return weight
	}

	return 0
}

// FormatPriceByCurrency 根据货币格式化价格
func (u *SKUUtils) FormatPriceByCurrency(price float64, currency string) float64 {
	switch currency {
	case "JPY", "KRW": // 日元、韩元不使用小数
		return float64(int(price))
	default:
		return price
	}
}

// BuildStockInfoList 构建库存信息列表
func (u *SKUUtils) BuildStockInfoList(ctx *model.TaskContext, stockCount int, warehouseCode string) []product.StockInfo {
	return []product.StockInfo{
		{
			InventoryNum:          stockCount,
			MerchantWarehouseCode: warehouseCode,
		},
	}
}

// BuildQuantityInfo 构建数量信息
func (u *SKUUtils) BuildQuantityInfo(params model.SKUCreationParams) *product.QuantityInfo {
	// 默认值：数量=1，类型=单品(1)，单位=件(1)
	quantity := 1
	quantityType := 1
	quantityUnit := 1

	// 从变体数据中获取数量信息（如果存在）
	if params.Variant.Quantity > 0 {
		quantity = params.Variant.Quantity
	}
	if params.Variant.QuantityType > 0 {
		quantityType = params.Variant.QuantityType
	}
	if params.Variant.UnitType > 0 {
		quantityUnit = params.Variant.UnitType
	}

	// 智能修正：检查数量类型和数量值的匹配性
	quantityType, quantity = u.CorrectQuantityTypeAndValue(quantityType, quantity, params.ASIN)

	// 创建验证器并修正数量单位
	validator := validation.NewQuantityValidator()

	// 根据数量类型自动修正单位类型
	correctUnit, err := validator.GetCorrectQuantityUnit(quantityType)
	if err != nil {
		logrus.Warnf("获取正确的数量单位失败: %v，使用原始值: %d", err, quantityUnit)
	} else {
		// 如果用户提供的单位与规则不符，使用正确的单位
		if quantityUnit != correctUnit {
			logrus.Warnf("数量单位不符合规则，quantityType=%d 应使用单位=%d，当前单位=%d，已自动修正",
				quantityType, correctUnit, quantityUnit)
			quantityUnit = correctUnit
		}
	}

	// 验证数量类型和单位类型的映射关系
	if err := validator.ValidateQuantityMapping(quantityType, quantityUnit); err != nil {
		logrus.Errorf("数量信息验证失败: %v", err)
	}

	// 验证数量值的合理性
	if err := validator.ValidateQuantity(quantity, quantityType); err != nil {
		logrus.Errorf("数量值验证失败: %v", err)
	}

	return &product.QuantityInfo{
		Quantity:     &quantity,
		QuantityType: &quantityType,
		QuantityUnit: &quantityUnit,
	}
}

// BuildSKUImageInfoForMultiPiece 为多件商品构建SKU图片信息
func (u *SKUUtils) BuildSKUImageInfoForMultiPiece(ctx *model.TaskContext, params model.SKUCreationParams) *product.ImageInfo {

	// 为多件商品构建SKU级别的图片信息
	var skuImages []product.ImageDetail
	var sourceImages []string

	// 从变体的属性中获取图片URL（如果有的话）
	if imageURL, exists := params.Variant.Attributes["image"]; exists && imageURL != "" {
		sourceImages = append(sourceImages, imageURL)
	}

	// 如果变体没有图片，使用主产品图片
	if len(sourceImages) == 0 && params.ProductInfo != nil && len(params.ProductInfo.Images) > 0 {
		// 对于主产品图片，限制数量
		maxImages := 1
		if len(params.ProductInfo.Images) < maxImages {
			maxImages = len(params.ProductInfo.Images)
		}
		sourceImages = params.ProductInfo.Images[:maxImages]
	}

	if len(sourceImages) == 0 {
		return nil
	}

	// 上传图片到SHEIN平台 - SKU只能有一张图片
	for _, imageURL := range sourceImages {
		if imageURL == "" {
			continue
		}

		// 使用ShopClient上传图片
		uploadedURL, err := ctx.ImageAPI.DownloadAndUploadImage(imageURL)
		if err != nil {
			logrus.Warnf("上传多件商品SKU图片失败，ASIN: %s, 原始URL: %s, 错误: %v", params.ASIN, imageURL, err)
			continue
		}

		if uploadedURL != "" {
			// SKU只能有一张图片，且必须是ImageType: 1，ImageSort: 1
			skuImages = append(skuImages, product.ImageDetail{
				ImageURL:  uploadedURL,
				ImageSort: 1,
				ImageType: 1, // SKU图片类型
			})

			// SKU只能有一张图片，上传成功后立即退出循环
			break
		}
	}

	if len(skuImages) == 0 {
		logrus.Warnf("多件商品ASIN %s 的所有图片上传都失败了", params.ASIN)
		return nil
	}

	logrus.Infof("为多件商品ASIN %s 成功构建了 %d 张SKU图片", params.ASIN, len(skuImages))

	return &product.ImageInfo{
		ImageInfoList: skuImages,
	}
}

// CorrectQuantityTypeAndValue 智能修正数量类型和数量值的匹配性
func (u *SKUUtils) CorrectQuantityTypeAndValue(quantityType, quantity int, asin string) (int, int) {
	// 检查当前设置是否符合规则
	validator := validation.NewQuantityValidator()
	if err := validator.ValidateQuantity(quantity, quantityType); err == nil {
		// 当前设置正确，无需修正
		return quantityType, quantity
	}

	logrus.Warnf("ASIN %s 的数量设置不符合规则: quantityType=%d, quantity=%d", asin, quantityType, quantity)

	// 修正策略1：如果是同款多件(2)但数量为1，改为单品(1)
	if quantityType == 2 && quantity == 1 {
		logrus.Infof("ASIN %s: 将同款多件(quantityType=2, quantity=1) 修正为单品(quantityType=1, quantity=1)", asin)
		return 1, 1
	}

	// 修正策略2：如果是多套(4)但数量为1，改为单套(3)
	if quantityType == 4 && quantity == 1 {
		logrus.Infof("ASIN %s: 将多套(quantityType=4, quantity=1) 修正为单套(quantityType=3, quantity=1)", asin)
		return 3, 1
	}

	// 修正策略3：如果是单品(1)或单套(3)但数量>1，根据类型调整
	if (quantityType == 1 || quantityType == 3) && quantity > 1 {
		if quantityType == 1 {
			logrus.Infof("ASIN %s: 将单品(quantityType=1, quantity=%d) 修正为同款多件(quantityType=2, quantity=%d)", asin, quantity, quantity)
			return 2, quantity
		} else {
			logrus.Infof("ASIN %s: 将单套(quantityType=3, quantity=%d) 修正为多套(quantityType=4, quantity=%d)", asin, quantity, quantity)
			return 4, quantity
		}
	}

	// 修正策略4：如果是同款多件(2)或多套(4)但数量<2，强制设为2
	if (quantityType == 2 || quantityType == 4) && quantity < 2 {
		logrus.Infof("ASIN %s: 将数量从 %d 修正为 2 (quantityType=%d 要求数量≥2)", asin, quantity, quantityType)
		return quantityType, 2
	}

	// 默认策略：如果无法修正，使用最安全的单品设置
	logrus.Warnf("ASIN %s: 无法智能修正，使用默认单品设置(quantityType=1, quantity=1)", asin)
	return 1, 1
}

