// package pipeline 提供Amazon属性映射处理器
package pipeline

import (
	"context"
	"fmt"
	"task-processor/internal/amazon/llm"
	"task-processor/internal/amazon/model"
)

// AttributeMapperHandler 属性映射处理器
type AttributeMapperHandler struct {
	*BaseHandler
	services *model.Services
}

// NewAttributeMapperHandler 创建属性映射处理器
func NewAttributeMapperHandler(services *model.Services) *AttributeMapperHandler {
	return &AttributeMapperHandler{
		BaseHandler: NewBaseHandler("LLM属性映射器"),
		services:    services,
	}
}

// Handle 处理逻辑
func (h *AttributeMapperHandler) Handle(ctx context.Context, taskContext *model.TaskContext) error {
	h.logger.Info("开始LLM智能属性映射")

	// 获取原始产品数据
	rawData, exists := taskContext.GetResult("raw_product_data")
	if !exists {
		return fmt.Errorf("原始产品数据不存在")
	}

	sourceData, ok := rawData.(map[string]any)
	if !ok {
		return fmt.Errorf("产品数据格式错误")
	}

	// 获取LLM属性映射器
	llmMapper := h.getLLMAttributeMapper()
	if llmMapper == nil {
		// 如果没有LLM服务，回退到基础映射
		h.logger.Warn("LLM服务不可用，使用基础映射")
		return h.handleBasicMapping(ctx, taskContext, sourceData)
	}

	// 使用LLM进行智能映射
	return h.handleLLMMapping(ctx, taskContext, sourceData, llmMapper)
}

// getLLMAttributeMapper 获取LLM属性映射器
func (h *AttributeMapperHandler) getLLMAttributeMapper() *llm.LLMAttributeMapper {
	if h.services == nil {
		return nil
	}

	mapper := h.services.GetLLMAttributeMapper()
	if mapper == nil {
		return nil
	}

	llmMapper, ok := mapper.(*llm.LLMAttributeMapper)
	if !ok {
		h.logger.Warn("LLM属性映射器类型转换失败")
		return nil
	}

	return llmMapper
}

// handleLLMMapping 使用LLM进行智能映射
func (h *AttributeMapperHandler) handleLLMMapping(ctx context.Context, taskContext *model.TaskContext, sourceData map[string]any, llmMapper *llm.LLMAttributeMapper) error {
	// 获取产品类型
	productType := h.getProductType(sourceData, taskContext.Data)

	// 构建LLM映射请求
	req := &llm.AttributeMappingRequest{
		SourcePlatform: "1688",
		TargetPlatform: "Amazon",
		ProductData:    sourceData,
		ProductType:    productType,
	}

	// 调用LLM进行映射
	resp, err := llmMapper.MapAttributes(ctx, req)
	if err != nil {
		h.logger.WithError(err).Error("LLM属性映射失败，回退到基础映射")
		return h.handleBasicMapping(ctx, taskContext, sourceData)
	}

	// 验证映射结果
	if err := llmMapper.ValidateMapping(resp.MappedAttributes); err != nil {
		h.logger.WithError(err).Warn("LLM映射结果验证失败，回退到基础映射")
		return h.handleBasicMapping(ctx, taskContext, sourceData)
	}

	h.logger.WithFields(map[string]any{
		"mapped_count": len(resp.MappedAttributes),
		"confidence":   resp.Confidence,
		"product_type": resp.ProductType,
	}).Info("LLM属性映射成功")

	// 保存映射结果
	taskContext.SetResult("mapped_attributes", resp.MappedAttributes)
	taskContext.SetResult("product_type", resp.ProductType)
	taskContext.SetResult("mapping_confidence", resp.Confidence)
	taskContext.SetResult("mapping_reasoning", resp.Reasoning)

	return nil
}

// handleBasicMapping 基础映射（回退方案）
func (h *AttributeMapperHandler) handleBasicMapping(_ context.Context, taskContext *model.TaskContext, sourceData map[string]any) error {
	h.logger.Info("使用基础属性映射")

	attributes := make(map[string]any)

	// 基础属性映射
	if title, ok := sourceData["title"].(string); ok {
		attributes["item_name"] = title
	}
	if brand, ok := sourceData["brand"].(string); ok {
		attributes["brand"] = brand
	}
	if desc, ok := sourceData["description"].(string); ok {
		attributes["product_description"] = desc
	}

	// 获取产品类型
	productType := h.getProductType(sourceData, taskContext.Data)

	h.logger.Infof("基础映射完成，映射了 %d 个属性", len(attributes))

	// 保存映射结果
	taskContext.SetResult("mapped_attributes", attributes)
	taskContext.SetResult("product_type", productType)
	taskContext.SetResult("mapping_confidence", 0.6) // 基础映射置信度较低

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

	// 3. 默认使用标准产品类型
	return "PRODUCT"
}
