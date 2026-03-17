package product

import (
	"fmt"

	"task-processor/internal/infra/clients/management/api"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/temu/ai"
	temucontext "task-processor/internal/temu/context"
	"task-processor/internal/temu/handlerbase"

	"github.com/sirupsen/logrus"
)

// BuildSpuHandler SPU构建处理器
type BuildSpuHandler struct {
	logger       *logrus.Entry
	builder      *SpuBuilder
	validator    *SpuValidator
	openaiConfig *openaiClient.ClientConfig
}

// NewBuildSpuHandler 创建新的SPU构建处理器
func NewBuildSpuHandler(openaiConfig *openaiClient.ClientConfig, profitRuleClient api.ProfitRuleAPI, skuBuilder handlerbase.SkuBuilder, specHandler handlerbase.SpecHandler) *BuildSpuHandler {
	logger := logrus.WithField("handler", "build_spu")
	return &BuildSpuHandler{
		logger:       logger,
		builder:      NewSpuBuilder(logger, openaiConfig, profitRuleClient, skuBuilder, specHandler),
		validator:    NewSpuValidator(logger),
		openaiConfig: openaiConfig,
	}
}

// Name 返回处理器名称
func (h *BuildSpuHandler) Name() string {
	return "BuildSpuHandler"
}

// Handle 处理SPU构建（优化版：AI内容重写与SKU构建并行）
func (h *BuildSpuHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始构建TEMU产品SPU")

	// 构建基本信息和扩展信息（AI重写需要这些数据）
	if err := h.builder.BuildBasicInfo(temuCtx, temuCtx.TemuProduct); err != nil {
		return fmt.Errorf("构建基本信息失败: %w", err)
	}

	if err := h.builder.BuildExtensionInfo(temuCtx, temuCtx.TemuProduct); err != nil {
		return fmt.Errorf("构建扩展信息失败: %w", err)
	}

	// 🚀 优化：AI内容重写与SKU构建并行执行
	h.logger.Info("🔄 开始并行执行：AI内容重写 + SKU构建")

	var aiErr, skuErr, serviceErr, saleErr error
	done := make(chan struct{})

	// 并行执行AI内容重写
	go func() {
		defer close(done)
		aiErr = h.triggerAIContentRewrite(temuCtx)
	}()

	// 主线程继续构建SKU、服务承诺和销售信息
	if err := h.builder.BuildSkcAndSku(temuCtx, temuCtx.TemuProduct); err != nil {
		skuErr = fmt.Errorf("构建SKC和SKU失败: %w", err)
	}

	if skuErr == nil {
		if err := h.builder.BuildServicePromise(temuCtx, temuCtx.TemuProduct); err != nil {
			serviceErr = fmt.Errorf("构建服务承诺失败: %w", err)
		}
	}

	if serviceErr == nil {
		if err := h.builder.BuildSaleInfo(temuCtx, temuCtx.TemuProduct); err != nil {
			saleErr = fmt.Errorf("构建销售信息失败: %w", err)
		}
	}

	// 等待AI内容重写完成
	<-done
	h.logger.Info("✅ 并行执行完成")

	// 检查错误
	if skuErr != nil {
		return skuErr
	}
	if serviceErr != nil {
		return serviceErr
	}
	if saleErr != nil {
		return saleErr
	}
	if aiErr != nil {
		// AI重写失败不阻断流程，只记录警告
		h.logger.Warnf("⚠️ AI内容重写失败（不影响主流程）: %v", aiErr)
	}

	// 验证产品数据
	if err := h.validator.ValidateProductData(temuCtx, temuCtx.TemuProduct); err != nil {
		return fmt.Errorf("产品数据验证失败: %w", err)
	}

	// 记录产品摘要
	h.validator.LogProductSummary(temuCtx, temuCtx.TemuProduct)

	h.logger.Info("TEMU产品SPU构建完成")
	return nil
}

// triggerAIContentRewrite 触发AI内容重写
func (h *BuildSpuHandler) triggerAIContentRewrite(temuCtx *temucontext.TemuTaskContext) error {
	// 检查是否配置了OpenAI
	if h.openaiConfig == nil {
		h.logger.Info("OpenAI未配置，跳过AI内容重写")
		return nil
	}

	h.logger.Info("🤖 开始AI内容重写（并行执行）")

	// 创建AI内容重写器
	rewriter := ai.NewAIContentRewriterHandler(h.openaiConfig)

	// 执行重写
	if err := rewriter.HandleTemu(temuCtx); err != nil {
		return fmt.Errorf("AI内容重写失败: %w", err)
	}

	h.logger.Info("✅ AI内容重写完成")
	return nil
}
