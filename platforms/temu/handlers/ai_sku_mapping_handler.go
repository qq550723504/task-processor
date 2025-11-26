package handlers

import (
	"fmt"

	"task-processor/common/amazon"
	"task-processor/common/pipeline"
	"task-processor/openai"

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
	logger := logrus.WithField("handler", "AISkuMappingHandler")

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

// Handle 处理任务
func (h *AISkuMappingHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始生成AI SKU映射")

	if h.aiClient == nil {
		h.logger.Warn("AI客户端未配置，跳过AI映射生成")
		return nil
	}

	// 获取变体列表
	variants, err := h.getVariants(ctx)
	if err != nil {
		return fmt.Errorf("获取变体列表失败: %w", err)
	}

	// 如果没有变体，尝试使用主产品
	if len(variants) == 0 {
		if ctx.AmazonProduct != nil {
			h.logger.Info("没有变体，使用主产品生成AI映射")
			variants = []*amazon.Product{ctx.AmazonProduct}
		} else {
			h.logger.Info("没有变体也没有主产品，跳过AI映射生成")
			return nil
		}
	}

	h.logger.Infof("开始为 %d 个产品生成AI映射", len(variants))

	// 生成AI SKU映射
	aiMapping, err := h.skuBuilder.generateAISkuMapping(ctx, variants)
	if err != nil {
		h.logger.Errorf("❌ AI生成SKU映射失败: %v", err)
		return fmt.Errorf("AI生成SKU映射失败: %w", err)
	}

	// 将AI映射存储到context中
	ctx.SetData("ai_sku_mapping", aiMapping)
	h.logger.Infof("✅ AI SKU映射已生成并存储，包含 %d 个SKU", len(aiMapping.SkuList))

	return nil
}

// getVariants 从context获取变体列表
func (h *AISkuMappingHandler) getVariants(ctx *pipeline.TaskContext) ([]*amazon.Product, error) {
	// 使用TaskContext的GetAmazonVariants方法
	variants := ctx.GetAmazonVariants()
	// 返回变体列表（可能为空），由调用方决定如何处理
	return variants, nil
}
