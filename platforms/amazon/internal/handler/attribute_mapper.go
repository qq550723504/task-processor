// Package handler 提供Amazon属性映射处理器
package handler

import (
	"fmt"
	"strings"
	"task-processor/platforms/amazon/internal/model"
	"task-processor/platforms/amazon/utils"
)

// AttributeMapperHandler 属性映射处理器
type AttributeMapperHandler struct {
	*BaseHandler
	mapper    *utils.AttributeMapper
	validator *utils.AttributeValidator
}

// NewAttributeMapperHandler 创建属性映射处理器
func NewAttributeMapperHandler() *AttributeMapperHandler {
	return &AttributeMapperHandler{
		BaseHandler: NewBaseHandler("属性映射器"),
	}
}

// Execute 处理逻辑
func (h *AttributeMapperHandler) Execute(services *model.Services, data map[string]any) error {
	h.logger.Info("开始映射产品属性")

	// 初始化映射器（如果需要）
	if err := h.initMapper(); err != nil {
		return fmt.Errorf("初始化映射器失败: %w", err)
	}

	// 获取原始产品数据
	rawData, exists := data["raw_product_data"]
	if !exists {
		return fmt.Errorf("原始产品数据不存在")
	}

	// 转换为 map
	sourceData, ok := rawData.(map[string]any)
	if !ok {
		return fmt.Errorf("产品数据格式错误")
	}

	// 获取产品类型
	productType := h.getProductType(sourceData, data)
	h.logger.Infof("产品类型: %s", productType)

	// 映射属性
	attributes, err := h.mapper.MapAttributes(sourceData, productType)
	if err != nil {
		return fmt.Errorf("属性映射失败: %w", err)
	}

	h.logger.Infof("成功映射 %d 个属性", len(attributes))

	// 验证属性
	if err := h.validator.ValidateAttributes(attributes, productType); err != nil {
		return fmt.Errorf("属性验证失败: %w", err)
	}

	h.logger.Info("属性验证通过")

	// 保存映射后的属性
	h.SetResult(data, "mapped_attributes", attributes)
	h.SetResult(data, "product_type", productType)

	// 记录映射详情
	h.logMappingDetails(attributes)

	h.logger.Info("属性映射完成")
	return nil
}

// initMapper 初始化映射器
func (h *AttributeMapperHandler) initMapper() error {
	if h.mapper == nil {
		// 创建属性映射器
		mapper, err := utils.NewAttributeMapper("config/attribute_mapping.json")
		if err != nil {
			return fmt.Errorf("创建属性映射器失败: %w", err)
		}
		h.mapper = mapper

		// 创建属性验证器
		h.validator = utils.NewAttributeValidator(mapper)
	}
	return nil
}

// getProductType 获取产品类型
func (h *AttributeMapperHandler) getProductType(sourceData map[string]any, data map[string]any) string {
	// 1. 优先使用已推荐的产品类型
	if productType, exists := data["product_type"]; exists {
		if pt, ok := productType.(string); ok && pt != "" {
			return pt
		}
	}

	// 2. 从源数据中获取产品类型
	if productType, ok := sourceData["product_type"].(string); ok && productType != "" {
		return productType
	}

	// 3. 从分类推断产品类型
	if category, ok := sourceData["category"].(string); ok {
		return h.inferProductType(category)
	}

	// 4. 默认使用标准产品类型
	return "PRODUCT"
}

// inferProductType 从分类推断产品类型
func (h *AttributeMapperHandler) inferProductType(category string) string {
	category = strings.ToLower(strings.TrimSpace(category))

	// 服装类
	if h.containsAny(category, []string{"clothing", "apparel", "fashion", "wear"}) {
		return "CLOTHING"
	}

	// 电子产品类
	if h.containsAny(category, []string{"electronics", "computer", "phone", "digital"}) {
		return "ELECTRONICS"
	}

	// 默认标准产品
	return "PRODUCT"
}

// containsAny 检查字符串是否包含任意一个子串
func (h *AttributeMapperHandler) containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// logMappingDetails 记录映射详情
func (h *AttributeMapperHandler) logMappingDetails(attributes map[string]any) {
	h.logger.Info("映射属性详情:")

	// 记录关键属性
	keyAttributes := []string{
		"item_name",
		"brand",
		"manufacturer",
		"product_description",
		"color",
		"size",
	}

	for _, key := range keyAttributes {
		if value, exists := attributes[key]; exists {
			h.logger.Infof("  - %s: %v", key, value)
		}
	}
}
