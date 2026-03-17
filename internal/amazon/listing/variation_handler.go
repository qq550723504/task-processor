package listing

import (
	"strings"
	"task-processor/internal/amazon/model"

	"github.com/sirupsen/logrus"
)

// VariationHandler 变体处理器
type VariationHandler struct {
	logger *logrus.Entry
}

// NewVariationHandler 创建变体处理器
func NewVariationHandler() *VariationHandler {
	return &VariationHandler{
		logger: logrus.WithField("component", "VariationHandler"),
	}
}

// AddVariationAttributes 添加变体属性
func (vh *VariationHandler) AddVariationAttributes(attrs map[string]any, data *model.ProductData, marketplaceID string) {
	if !data.HasVariation() {
		return
	}

	variation := data.VariationData

	// 如果没有指定变体主题，自动生成
	if variation.VariationTheme == "" && len(variation.VariationValues) > 0 {
		variation.VariationTheme = vh.generateVariationTheme(variation.VariationValues)
	}

	if variation.IsParent {
		vh.addParentVariationAttributes(attrs, variation, data.SKU, marketplaceID)
	} else {
		vh.addChildVariationAttributes(attrs, variation, data.SKU, data.ProductType, marketplaceID)
	}
}

// ValidateVariationData 验证变体数据
func (vh *VariationHandler) ValidateVariationData(data *model.ProductData) error {
	if !data.HasVariation() {
		return nil
	}

	variationData := data.VariationData

	if variationData.VariationTheme == "" {
		return model.NewValidationError("variation_theme", variationData.VariationTheme, "变体主题不能为空")
	}

	if !model.IsValidVariationTheme(variationData.VariationTheme) {
		return model.NewValidationError("variation_theme", variationData.VariationTheme, "无效的变体主题")
	}

	expectedAttrs := model.GetAttributesForTheme(variationData.VariationTheme)
	for _, attr := range expectedAttrs {
		if _, exists := variationData.VariationValues[attr]; !exists {
			return model.NewValidationError("variation_values", attr, "缺少必需的变体属性")
		}
	}

	return nil
}

// generateVariationTheme 根据变体值自动生成变体主题
func (vh *VariationHandler) generateVariationTheme(variationValues map[string]string) string {
	if len(variationValues) == 0 {
		return ""
	}

	// 获取所有变体属性名并排序
	var attrs []string
	for attr := range variationValues {
		attrs = append(attrs, attr)
	}

	// 简单地将属性名首字母大写后连接
	// 让Amazon自己决定是否接受这个变体主题
	var themeParts []string
	for _, attr := range attrs {
		if len(attr) > 0 {
			themeParts = append(themeParts, strings.ToUpper(attr[:1])+attr[1:])
		}
	}

	theme := strings.Join(themeParts, "")
	vh.logger.WithFields(logrus.Fields{
		"variation_attributes": attrs,
		"generated_theme":      theme,
	}).Info("自动生成变体主题")

	return theme
}

// addParentVariationAttributes 添加父产品变体属性
func (vh *VariationHandler) addParentVariationAttributes(attrs map[string]any, variation *model.VariationData, sku string, marketplaceID string) {
	// 设置父产品层级
	attrs["parentage_level"] = []map[string]any{
		{"value": "parent", "marketplace_id": marketplaceID},
	}

	// 设置父子关系类型
	attrs["child_parent_sku_relationship"] = []map[string]any{
		{
			"child_relationship_type": "variation",
			"marketplace_id":          marketplaceID,
		},
	}

	// 设置变体主题
	if variation.VariationTheme != "" {
		attrs["variation_theme"] = []map[string]any{
			{"name": variation.VariationTheme, "marketplace_id": marketplaceID},
		}
	}

	vh.logger.WithFields(logrus.Fields{
		"parent_sku":      sku,
		"variation_theme": variation.VariationTheme,
		"child_count":     len(variation.AllVariations),
	}).Info("设置父产品变体信息")
}

// addChildVariationAttributes 添加子产品变体属性
func (vh *VariationHandler) addChildVariationAttributes(attrs map[string]any, variation *model.VariationData, sku string, productType string, marketplaceID string) {
	// 设置子产品层级
	attrs["parentage_level"] = []map[string]any{
		{"value": "child", "marketplace_id": marketplaceID},
	}

	// 设置父子关系
	if variation.ParentSKU != "" {
		attrs["child_parent_sku_relationship"] = []map[string]any{
			{
				"child_relationship_type": "variation",
				"parent_sku":              variation.ParentSKU,
				"marketplace_id":          marketplaceID,
			},
		}
	}

	// 设置变体主题
	if variation.VariationTheme != "" {
		attrs["variation_theme"] = []map[string]any{
			{"name": variation.VariationTheme, "marketplace_id": marketplaceID},
		}
	}

	// 直接设置变体值，完全通用，不限制任何属性
	for attrName, attrValue := range variation.VariationValues {
		attrs[attrName] = []map[string]any{
			{"value": attrValue, "marketplace_id": marketplaceID},
		}
	}

	vh.logger.WithFields(logrus.Fields{
		"child_sku":        sku,
		"parent_sku":       variation.ParentSKU,
		"product_type":     productType,
		"variation_values": variation.VariationValues,
		"variation_theme":  variation.VariationTheme,
	}).Info("设置子产品变体信息")
}
