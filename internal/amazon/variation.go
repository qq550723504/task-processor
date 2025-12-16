// Package amazon 提供Amazon变体产品处理功能
package amazon

import (
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
)

// VariationData 变体产品数据
type VariationData struct {
	ParentSKU       string            // 父产品SKU
	IsParent        bool              // 是否为父产品
	VariationTheme  string            // 变体主题（如 "ColorSize", "Color", "Size"）
	VariationValues map[string]string // 当前产品的变体值（如 {"color": "Red", "size": "Large"}）
	AllVariations   []VariationChild  // 所有子变体（仅父产品使用）
}

// VariationChild 子变体信息
type VariationChild struct {
	SKU             string            // 子产品SKU
	Price           float64           // 子产品价格
	Quantity        int               // 子产品库存
	MainImageURL    string            // 子产品主图
	VariationValues map[string]string // 子产品变体值
}

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
func (h *VariationHandler) AddVariationAttributes(attrs map[string]any, data *ProductData, marketplaceID string) {
	if data.VariationData == nil {
		return
	}

	variation := data.VariationData

	// 如果没有指定变体主题，自动生成
	if variation.VariationTheme == "" && len(variation.VariationValues) > 0 {
		variation.VariationTheme = h.generateVariationTheme(variation.VariationValues)
	}

	if variation.IsParent {
		h.addParentVariationAttributes(attrs, variation, data.SKU, marketplaceID)
	} else {
		h.addChildVariationAttributes(attrs, variation, data.SKU, data.ProductType, marketplaceID)
	}
}

// generateVariationTheme 根据变体值自动生成变体主题
func (h *VariationHandler) generateVariationTheme(variationValues map[string]string) string {
	if len(variationValues) == 0 {
		return ""
	}

	// 获取所有变体属性名并排序
	var attrs []string
	for attr := range variationValues {
		attrs = append(attrs, attr)
	}
	sort.Strings(attrs)

	// 简单地将属性名首字母大写后连接
	// 让Amazon自己决定是否接受这个变体主题
	var themeParts []string
	for _, attr := range attrs {
		if len(attr) > 0 {
			themeParts = append(themeParts, strings.ToUpper(attr[:1])+attr[1:])
		}
	}

	theme := strings.Join(themeParts, "")
	h.logger.WithFields(logrus.Fields{
		"variation_attributes": attrs,
		"generated_theme":      theme,
	}).Info("自动生成变体主题")

	return theme
}

// addParentVariationAttributes 添加父产品变体属性
func (h *VariationHandler) addParentVariationAttributes(attrs map[string]any, variation *VariationData, sku string, marketplaceID string) {
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

	h.logger.WithFields(logrus.Fields{
		"parent_sku":      sku,
		"variation_theme": variation.VariationTheme,
		"child_count":     len(variation.AllVariations),
	}).Info("设置父产品变体信息")
}

// addChildVariationAttributes 添加子产品变体属性
func (h *VariationHandler) addChildVariationAttributes(attrs map[string]any, variation *VariationData, sku string, productType string, marketplaceID string) {
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

	h.logger.WithFields(logrus.Fields{
		"child_sku":        sku,
		"parent_sku":       variation.ParentSKU,
		"product_type":     productType,
		"variation_values": variation.VariationValues,
		"variation_theme":  variation.VariationTheme,
	}).Info("设置子产品变体信息")
}

// ValidateVariationData 验证变体数据
func (h *VariationHandler) ValidateVariationData(data *ProductData) error {
	if data.VariationData == nil {
		return nil
	}

	variation := data.VariationData

	// 父产品验证
	if variation.IsParent {
		if variation.VariationTheme == "" {
			h.logger.Warn("父产品缺少变体主题")
		}
		if variation.ParentSKU != "" {
			h.logger.Warn("父产品不应设置ParentSKU")
		}
	} else {
		// 子产品验证
		if variation.ParentSKU == "" {
			h.logger.Warn("子产品缺少父产品SKU")
		}
		if len(variation.VariationValues) == 0 {
			h.logger.Warn("子产品缺少变体值")
		}
	}

	return nil
}
