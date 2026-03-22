package sku

import (
	"fmt"

	"task-processor/internal/infra/clients/openai"
	"task-processor/internal/model"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/temu/context"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// AISkuMappingHandler AI SKU映射处理器 - 提前生成AI映射供后续处理器使用
type AISkuMappingHandler struct {
	logger     *logrus.Entry
	aiClient   *openai.Client
	skuBuilder *SkuBuilder
}

// NewAISkuMappingHandler 创建新的AI SKU映射处理器
func NewAISkuMappingHandler(openaiConfig *openai.ClientConfig) *AISkuMappingHandler {
	logger := logger.GetGlobalLogger("AISkuMappingHandler")

	var aiClient *openai.Client
	if openaiConfig != nil {
		aiClient = openai.NewClient(openaiConfig)
	}

	skuBuilder := NewSkuBuilder(logger, aiClient, nil)

	return &AISkuMappingHandler{
		logger:     logger,
		aiClient:   aiClient,
		skuBuilder: skuBuilder,
	}
}

// Name 返回处理器名称
func (h *AISkuMappingHandler) Name() string {
	return "AI SKU映射处理器"
}

// HandleTemu 处理TEMU任务（实现TemuHandler接口）
func (h *AISkuMappingHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始生成AI SKU映射")

	if h.aiClient == nil {
		h.logger.Warn("AI客户端未配置，跳过AI映射生成")
		return nil
	}

	variants := h.getVariants(temuCtx)

	// 如果没有变体，尝试使用主产品
	if len(variants) == 0 {
		// 直接从强类型上下文获取Amazon产品
		amazonProduct := temuCtx.GetAmazonProduct()
		if amazonProduct != nil {
			h.logger.Info("没有变体，使用主产品生成AI映射")
			variants = []*model.Product{amazonProduct}
		} else {
			h.logger.Info("没有变体也没有主产品，跳过AI映射生成")
			return nil
		}
	}

	h.logger.Infof("开始为 %d 个产品生成AI映射", len(variants))

	// 生成AI SKU映射
	aiMapping, err := h.skuBuilder.variantProcessor.GenerateAISkuMapping(temuCtx, variants)
	if err != nil {
		h.logger.Errorf("❌ AI生成SKU映射失败: %v", err)
		return fmt.Errorf("AI生成SKU映射失败: %w", err)
	}

	// 将AI映射存储到强类型字段中
	temuCtx.AISkuMapping = aiMapping

	h.logger.Infof("✅ AI SKU映射已生成并存储，包含 %d 个SKU", len(aiMapping.SkuList))

	return nil
}

// getVariants 从context获取变体列表
func (h *AISkuMappingHandler) getVariants(temuCtx *temucontext.TemuTaskContext) []*model.Product {
	return temuCtx.GetVariants()
}

// Handle 兼容原有的Handler接口（用于pipeline.AddHandler）
func (h *AISkuMappingHandler) Handle(ctx pipeline.TaskContext) error {
	// 尝试类型断言为TemuTaskContext
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return h.HandleTemu(temuCtx)
	}
	// 如果不是TemuTaskContext，返回错误
	return fmt.Errorf("上下文类型错误，期望*temucontext.TemuTaskContext，实际类型: %T", ctx)
}
