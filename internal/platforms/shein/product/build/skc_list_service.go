package build

import (
	"fmt"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/platforms/shein"
	"task-processor/internal/platforms/shein/product/attribute"
	"task-processor/internal/platforms/shein/product/image"
	"task-processor/internal/platforms/shein/product/skc"
	"task-processor/internal/platforms/shein/product/sku"
	"task-processor/internal/platforms/shein/product/variant"

	"github.com/sirupsen/logrus"
)

// BuildSkcListHandler 构建SKU列表处理器
type BuildSkcListHandler struct {
	imageDownloader interface {
		DownloadImage(url string) ([]byte, error)
	}
	strategyHandler *skc.AttributeStrategyHandler
	skcBuilder      *skc.SKCBuilder
	openaiConfig    *openaiClient.ClientConfig
}

// NewBuildSkcListHandler 创建新的构建SKC列表处理器
func NewBuildSkcListHandler(imageDownloader interface {
	DownloadImage(url string) ([]byte, error)
}, openaiConfig *openaiClient.ClientConfig) *BuildSkcListHandler {
	openaiClient := openaiClient.NewClient(openaiConfig)
	// 创建依赖组件
	imageProcessor := image.NewImageProcessor(imageDownloader)
	attributeMapper := attribute.NewAttributeMapper()
	variantMatcher := variant.NewVariantMatcher()
	skuBuilder := sku.NewSKUBuilder(variantMatcher)
	skcBuilder := skc.NewSKCBuilder(imageProcessor, attributeMapper, variantMatcher, skuBuilder, openaiClient)
	strategyHandler := skc.NewAttributeStrategyHandler()

	return &BuildSkcListHandler{
		imageDownloader: imageDownloader,
		strategyHandler: strategyHandler,
		skcBuilder:      skcBuilder,
		openaiConfig:    openaiConfig,
	}
}

// Name 返回处理器名称
func (h *BuildSkcListHandler) Name() string {
	return "构建SKC列表"
}

// Handle 执行构建SKU列表处理
func (h *BuildSkcListHandler) Handle(ctx *shein.TaskContext) error {
	logrus.Infof("=== 开始构建SKC列表处理 ===")

	// 检查前置条件
	if ctx.ProductData == nil {
		logrus.Errorf("❌ 产品数据未获取")
		return fmt.Errorf("产品数据未获取，请先执行获取产品数据步骤")
	}
	logrus.Infof("✅ 产品数据检查通过")

	// 检查销售规格结果
	if ctx.SaleSpecResult == nil {
		logrus.Errorf("❌ 销售规格结果未获取")
		return fmt.Errorf("销售规格结果未获取，请先执行销售属性处理步骤")
	}
	logrus.Infof("✅ 销售规格结果检查通过 - 变体数量: %d", len(ctx.SaleSpecResult.Variants))

	// 检查属性模板
	if ctx.AttributeTemplates == nil {
		logrus.Errorf("❌ 属性模板未获取")
		return fmt.Errorf("属性模板未获取，请先执行属性模板处理步骤")
	}
	logrus.Infof("✅ 属性模板检查通过")

	logrus.Infof("🚀 开始调用SKC构建器...")
	skcList, customAttributeRelations, err := h.skcBuilder.BuildSKCListWithSpecAdaptation(ctx, h.strategyHandler)
	if err != nil {
		logrus.Errorf("❌ SKC列表构建失败: %v", err)
		return err
	}

	logrus.Infof("📋 SKC构建结果:")
	logrus.Infof("  - SKC数量: %d", len(skcList))
	logrus.Infof("  - 自定义属性关系数量: %d", len(customAttributeRelations))

	// 如果SKC列表为空，提供详细的调试信息
	if len(skcList) == 0 {
		// 修改为不可重试错误
		return shein.NewNonRetryableError("SKC列表构建结果为空", nil)
	} else {
		// 打印每个SKC的详情
		for i, skc := range skcList {
			logrus.Infof("  SKC[%d]: 属性ID=%d, 属性值ID=%d, SKU数量=%d",
				i+1, skc.SaleAttribute.AttributeID, skc.SaleAttribute.AttributeValueID, len(skc.SKUS))
		}
	}

	ctx.ProductData.SKCList = skcList
	ctx.ProductData.CustomAttributeRelation = customAttributeRelations

	logrus.Infof("=== SKC列表构建处理完成 ===")
	return nil
}



