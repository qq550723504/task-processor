package build

import (
	"fmt"
	"task-processor/internal/core/logger"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/shein"
	"task-processor/internal/shein/product/attribute"
	"task-processor/internal/shein/product/image"
	"task-processor/internal/shein/product/skc"
	"task-processor/internal/shein/product/sku"
	"task-processor/internal/shein/product/variant"
)

// BuildSkcListHandler 构建SKU列表处理器
type BuildSkcListHandler struct {
	imageDownloader interface {
		DownloadImage(url string) ([]byte, error)
	}
	strategyHandler *skc.AttributeStrategyHandler
	skcBuilder      *skc.SKCBuilder
}

// NewBuildSkcListHandler 创建新的构建SKC列表处理器
func NewBuildSkcListHandler(imageDownloader interface {
	DownloadImage(url string) ([]byte, error)
}, client openaiClient.ChatCompleter) *BuildSkcListHandler {
	imageProcessor := image.NewImageProcessor(imageDownloader)
	attributeMapper := attribute.NewAttributeMapper()
	variantMatcher := variant.NewVariantMatcher()
	skuBuilder := sku.NewSKUBuilder(variantMatcher)
	skcBuilder := skc.NewSKCBuilder(imageProcessor, attributeMapper, variantMatcher, skuBuilder, client)
	strategyHandler := skc.NewAttributeStrategyHandler()

	return &BuildSkcListHandler{
		imageDownloader: imageDownloader,
		strategyHandler: strategyHandler,
		skcBuilder:      skcBuilder,
	}
}

// Name 返回处理器名称
func (h *BuildSkcListHandler) Name() string {
	return "构建SKC列表"
}

// Handle 执行构建SKU列表处理
func (h *BuildSkcListHandler) Handle(ctx *shein.TaskContext) error {
	logger.GetGlobalLogger("shein/product").Infof("=== 开始构建SKC列表处理 ===")

	// 检查前置条件
	if ctx.ProductData == nil {
		logger.GetGlobalLogger("shein/product").Errorf("❌ 产品数据未获取")
		return fmt.Errorf("产品数据未获取，请先执行获取产品数据步骤")
	}
	logger.GetGlobalLogger("shein/product").Infof("✅ 产品数据检查通过")

	// 检查销售规格结果
	if ctx.SaleSpecResult == nil {
		logger.GetGlobalLogger("shein/product").Errorf("❌ 销售规格结果未获取")
		return fmt.Errorf("销售规格结果未获取，请先执行销售属性处理步骤")
	}
	logger.GetGlobalLogger("shein/product").Infof("✅ 销售规格结果检查通过 - 变体数量: %d", len(ctx.SaleSpecResult.Variants))

	// 检查属性模板
	if ctx.AttributeTemplates == nil {
		logger.GetGlobalLogger("shein/product").Errorf("❌ 属性模板未获取")
		return fmt.Errorf("属性模板未获取，请先执行属性模板处理步骤")
	}
	logger.GetGlobalLogger("shein/product").Infof("✅ 属性模板检查通过")

	logger.GetGlobalLogger("shein/product").Infof("🚀 开始调用SKC构建器...")
	skcList, customAttributeRelations, err := h.skcBuilder.BuildSKCListWithSpecAdaptation(ctx, h.strategyHandler)
	if err != nil {
		logger.GetGlobalLogger("shein/product").Errorf("❌ SKC列表构建失败: %v", err)
		return err
	}

	logger.GetGlobalLogger("shein/product").Infof("📋 SKC构建结果:")
	logger.GetGlobalLogger("shein/product").Infof("  - SKC数量: %d", len(skcList))
	logger.GetGlobalLogger("shein/product").Infof("  - 自定义属性关系数量: %d", len(customAttributeRelations))

	// 如果SKC列表为空，提供详细的调试信息
	if len(skcList) == 0 {
		// 修改为不可重试错误
		return shein.NewNonRetryableError("SKC列表构建结果为空", nil)
	} else {
		// 打印每个SKC的详情
		for i, skc := range skcList {
			logger.GetGlobalLogger("shein/product").Infof("  SKC[%d]: 属性ID=%d, 属性值ID=%d, SKU数量=%d",
				i+1, skc.SaleAttribute.AttributeID, skc.SaleAttribute.AttributeValueID, len(skc.SKUS))
		}
	}

	ctx.ProductData.SKCList = skcList
	ctx.ProductData.CustomAttributeRelation = customAttributeRelations

	logger.GetGlobalLogger("shein/product").Infof("=== SKC列表构建处理完成 ===")
	return nil
}
