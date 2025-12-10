package handlers

import (
	"fmt"
	"strings"
	"task-processor/platforms/amazon"
	"task-processor/platforms/amazon/utils"

	"github.com/sirupsen/logrus"
)

// AttributeMapperHandler 属性映射处理器
type AttributeMapperHandler struct {
	mapper    *utils.AttributeMapper
	validator *utils.AttributeValidator
}

// NewAttributeMapperHandler 创建属性映射处理器
func NewAttributeMapperHandler(configPath string) (*AttributeMapperHandler, error) {
	// 创建属性映射器
	mapper, err := utils.NewAttributeMapper(configPath)
	if err != nil {
		return nil, fmt.Errorf("创建属性映射器失败: %w", err)
	}

	// 创建属性验证器
	validator := utils.NewAttributeValidator(mapper)

	return &AttributeMapperHandler{
		mapper:    mapper,
		validator: validator,
	}, nil
}

// Name 返回处理器名称
func (h *AttributeMapperHandler) Name() string {
	return "映射产品属性"
}

// Handle 处理逻辑
func (h *AttributeMapperHandler) Handle(ctx *amazon.TaskContext) error {
	logrus.Info("[AttributeMapper] 开始映射产品属性")

	// 获取原始产品数据
	rawData, exists := ctx.GetData("raw_product_data")
	if !exists {
		return fmt.Errorf("原始产品数据不存在")
	}

	// 转换为 map
	sourceData, ok := rawData.(map[string]interface{})
	if !ok {
		return fmt.Errorf("产品数据格式错误")
	}

	// 获取产品类型
	productType := h.getProductType(sourceData)
	logrus.Infof("[AttributeMapper] 产品类型: %s", productType)

	// 映射属性
	attributes, err := h.mapper.MapAttributes(sourceData, productType)
	if err != nil {
		return fmt.Errorf("属性映射失败: %w", err)
	}

	logrus.Infof("[AttributeMapper] 成功映射 %d 个属性", len(attributes))

	// 验证属性
	if err := h.validator.ValidateAttributes(attributes, productType); err != nil {
		return fmt.Errorf("属性验证失败: %w", err)
	}

	logrus.Info("[AttributeMapper] 属性验证通过")

	// 保存映射后的属性
	ctx.SetData("mapped_attributes", attributes)
	ctx.SetData("product_type", productType)

	// 记录映射详情
	h.logMappingDetails(attributes)

	logrus.Info("[AttributeMapper] 属性映射完成")
	return nil
}

// getProductType 获取产品类型
func (h *AttributeMapperHandler) getProductType(sourceData map[string]interface{}) string {
	// 从源数据中获取产品类型
	if productType, ok := sourceData["product_type"].(string); ok && productType != "" {
		return productType
	}

	// 从分类推断产品类型
	if category, ok := sourceData["category"].(string); ok {
		return h.inferProductType(category)
	}

	// 默认使用标准产品类型
	return "PRODUCT"
}

// inferProductType 从分类推断产品类型
func (h *AttributeMapperHandler) inferProductType(category string) string {
	category = toLower(category)

	// 服装类
	if contains(category, "clothing") || contains(category, "apparel") ||
		contains(category, "fashion") || contains(category, "wear") {
		return "CLOTHING"
	}

	// 电子产品类
	if contains(category, "electronics") || contains(category, "computer") ||
		contains(category, "phone") || contains(category, "digital") {
		return "ELECTRONICS"
	}

	// 默认标准产品
	return "PRODUCT"
}

// logMappingDetails 记录映射详情
func (h *AttributeMapperHandler) logMappingDetails(attributes map[string]interface{}) {
	logrus.Info("[AttributeMapper] 映射属性详情:")

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
			logrus.Infof("  - %s: %v", key, value)
		}
	}
}

// toLower 转换为小写
func toLower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return strings.Contains(toLower(s), toLower(substr))
}

// GetMapper 获取属性映射器
func (h *AttributeMapperHandler) GetMapper() *utils.AttributeMapper {
	return h.mapper
}

// GetValidator 获取属性验证器
func (h *AttributeMapperHandler) GetValidator() *utils.AttributeValidator {
	return h.validator
}
