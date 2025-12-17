package utils

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// VariantExtractor 变体数据提取器
type VariantExtractor struct {
	logger *logrus.Entry
}

// NewVariantExtractor 创建变体提取器
func NewVariantExtractor() *VariantExtractor {
	return &VariantExtractor{
		logger: logrus.WithField("component", "VariantExtractor"),
	}
}

// ExtractVariants 从1688数据中提取变体信息
func (e *VariantExtractor) ExtractVariants(productData map[string]interface{}) (*VariantData, error) {
	e.logger.Info("开始提取变体数据")

	// 1. 检查是否有SKU信息
	skuInfo := e.extractSKUInfo(productData)
	if len(skuInfo) == 0 {
		e.logger.Info("未找到SKU信息，这是单品")
		return nil, nil
	}

	// 2. 提取变体属性
	variantAttrs := e.extractVariantAttributes(skuInfo)
	if len(variantAttrs) == 0 {
		e.logger.Warn("未找到变体属性")
		return nil, nil
	}

	// 3. 确定变体主题
	theme := e.determineVariationTheme(variantAttrs)

	// 4. 构建变体数据
	variantData := &VariantData{
		Theme:      theme,
		Attributes: variantAttrs,
		SKUs:       skuInfo,
	}

	e.logger.Infof("提取到 %d 个变体，主题: %s", len(skuInfo), theme)
	return variantData, nil
}

// extractSKUInfo 提取SKU信息
func (e *VariantExtractor) extractSKUInfo(data map[string]interface{}) []map[string]interface{} {
	// 1688常见的SKU字段
	skuFields := []string{
		"skuInfos",
		"skuList",
		"skus",
		"productSKUPropertyList",
	}

	for _, field := range skuFields {
		if val, ok := data[field]; ok {
			if skuList, ok := val.([]interface{}); ok {
				result := make([]map[string]interface{}, 0, len(skuList))
				for _, sku := range skuList {
					if skuMap, ok := sku.(map[string]interface{}); ok {
						result = append(result, skuMap)
					}
				}
				if len(result) > 0 {
					e.logger.Infof("从字段 %s 提取到 %d 个SKU", field, len(result))
					return result
				}
			}
		}
	}

	return nil
}

// extractVariantAttributes 提取变体属性
func (e *VariantExtractor) extractVariantAttributes(skuInfo []map[string]interface{}) []string {
	attributeSet := make(map[string]bool)

	for _, sku := range skuInfo {
		// 查找属性字段
		attrFields := []string{
			"specAttrs",
			"attributes",
			"properties",
			"skuProps",
		}

		for _, field := range attrFields {
			if attrs, ok := sku[field]; ok {
				if attrList, ok := attrs.([]interface{}); ok {
					for _, attr := range attrList {
						if attrMap, ok := attr.(map[string]interface{}); ok {
							// 提取属性名称
							if name, ok := attrMap["name"].(string); ok {
								attributeSet[strings.ToLower(name)] = true
							}
							if attrName, ok := attrMap["attributeName"].(string); ok {
								attributeSet[strings.ToLower(attrName)] = true
							}
						}
					}
				}
			}
		}
	}

	// 转换为数组
	attributes := make([]string, 0, len(attributeSet))
	for attr := range attributeSet {
		attributes = append(attributes, attr)
	}

	return attributes
}

// determineVariationTheme 确定变体主题
func (e *VariantExtractor) determineVariationTheme(attributes []string) string {
	hasColor := false
	hasSize := false

	for _, attr := range attributes {
		attrLower := strings.ToLower(attr)
		if strings.Contains(attrLower, "color") || strings.Contains(attrLower, "颜色") {
			hasColor = true
		}
		if strings.Contains(attrLower, "size") || strings.Contains(attrLower, "尺码") || strings.Contains(attrLower, "尺寸") {
			hasSize = true
		}
	}

	if hasColor && hasSize {
		return "SizeColor"
	} else if hasSize {
		return "Size"
	} else if hasColor {
		return "Color"
	}

	return "Style" // 默认使用款式变体
}

// VariantData 变体数据
type VariantData struct {
	Theme      string                   `json:"theme"`
	Attributes []string                 `json:"attributes"`
	SKUs       []map[string]interface{} `json:"skus"`
}

// BuildVariantChildren 构建子变体列表
func (e *VariantExtractor) BuildVariantChildren(
	variantData *VariantData,
	parentSKU string,
) ([]VariantChildData, error) {
	children := make([]VariantChildData, 0, len(variantData.SKUs))

	for i, skuInfo := range variantData.SKUs {
		child := VariantChildData{
			SKU:           fmt.Sprintf("%s-V%d", parentSKU, i+1),
			VariationData: e.extractVariationValues(skuInfo),
			Price:         e.extractPrice(skuInfo),
			Quantity:      e.extractQuantity(skuInfo),
			Images:        e.extractImages(skuInfo),
		}

		children = append(children, child)
	}

	e.logger.Infof("构建了 %d 个子变体", len(children))
	return children, nil
}

// extractVariationValues 提取变体值
func (e *VariantExtractor) extractVariationValues(skuInfo map[string]interface{}) map[string]string {
	values := make(map[string]string)

	// 尝试从不同字段提取
	if specAttrs, ok := skuInfo["specAttrs"].([]interface{}); ok {
		for _, attr := range specAttrs {
			if attrMap, ok := attr.(map[string]interface{}); ok {
				name := e.getString(attrMap, "name", "attributeName")
				value := e.getString(attrMap, "value", "attributeValue")
				if name != "" && value != "" {
					values[strings.ToLower(name)] = value
				}
			}
		}
	}

	return values
}

// extractPrice 提取价格
func (e *VariantExtractor) extractPrice(skuInfo map[string]interface{}) float64 {
	priceFields := []string{"price", "salePrice", "consignPrice"}
	for _, field := range priceFields {
		if val, ok := skuInfo[field]; ok {
			switch v := val.(type) {
			case float64:
				return v
			case string:
				var price float64
				fmt.Sscanf(v, "%f", &price)
				return price
			}
		}
	}
	return 0
}

// extractQuantity 提取库存
func (e *VariantExtractor) extractQuantity(skuInfo map[string]interface{}) int {
	qtyFields := []string{"quantity", "canBookCount", "amountOnSale"}
	for _, field := range qtyFields {
		if val, ok := skuInfo[field]; ok {
			switch v := val.(type) {
			case int:
				return v
			case float64:
				return int(v)
			case string:
				var qty int
				fmt.Sscanf(v, "%d", &qty)
				return qty
			}
		}
	}
	return 0
}

// extractImages 提取图片
func (e *VariantExtractor) extractImages(skuInfo map[string]interface{}) []string {
	var images []string

	imageFields := []string{"image", "imageUrl", "skuImage"}
	for _, field := range imageFields {
		if val, ok := skuInfo[field]; ok {
			if img, ok := val.(string); ok && img != "" {
				images = append(images, img)
			}
		}
	}

	return images
}

// getString 从map中获取字符串值
func (e *VariantExtractor) getString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			if str, ok := val.(string); ok {
				return str
			}
		}
	}
	return ""
}

// VariantChildData 子变体数据
type VariantChildData struct {
	SKU           string            `json:"sku"`
	VariationData map[string]string `json:"variation_data"`
	Price         float64           `json:"price"`
	Quantity      int               `json:"quantity"`
	Images        []string          `json:"images"`
}
