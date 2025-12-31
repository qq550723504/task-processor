// Package handlers 提供TEMU平台的变体数据处理功能
package handlers

import (
	"task-processor/internal/domain/model"
	"task-processor/internal/pkg/utils"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// VariantProcessor 变体数据处理器
type VariantProcessor struct {
	logger *logrus.Entry
}

// NewVariantProcessor 创建新的变体数据处理器
func NewVariantProcessor(logger *logrus.Entry) *VariantProcessor {
	return &VariantProcessor{
		logger: logger.WithField("component", "VariantProcessor"),
	}
}

// Name 返回处理器名称
func (p *VariantProcessor) Name() string {
	return "变体数据处理器"
}

// HandleTemu 处理任务（强类型上下文）
func (p *VariantProcessor) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	// 这里可以根据需要调用具体的处理方法
	return p.ProcessSingleProduct(temuCtx)
}

// ProcessSingleProduct 处理单一产品（无变体）
func (p *VariantProcessor) ProcessSingleProduct(temuCtx *temucontext.TemuTaskContext) error {
	p.logger.Info("处理单一产品模式")

	var productName, description string

	// 获取Amazon产品数据 - 从强类型上下文获取
	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct != nil {
		productName = amazonProduct.Title
		description = amazonProduct.Description
	}

	// 检查TEMU产品数据
	if temuCtx.TemuProduct != nil && productName != "" {
		// 清理产品标题，移除特殊符号和表情符号
		cleanedTitle := utils.CleanProductTitle(productName)
		temuCtx.TemuProduct.GoodsBasic.GoodsName = cleanedTitle

		p.logger.Debugf("产品标题已清理: %s -> %s", productName, cleanedTitle)

		// 设置产品描述
		if description != "" {
			temuCtx.TemuProduct.GoodsExtensionInfo.GoodsDesc = description
		}
	}

	return nil
}

// ProcessVariantData 处理变体数据
func (p *VariantProcessor) ProcessVariantData(temuCtx *temucontext.TemuTaskContext, variants []*model.Product) error {
	p.logger.Info("开始处理产品变体数据")

	if len(variants) == 0 {
		p.logger.Info("未发现变体数据，使用单一产品模式")
		return p.ProcessSingleProduct(temuCtx)
	}

	p.logger.Infof("发现 %d 个变体", len(variants))

	// 处理每个变体
	for i, variant := range variants {
		if variant == nil {
			continue
		}

		// 清理变体标题
		if variant.Title != "" {
			originalTitle := variant.Title
			variant.Title = utils.CleanProductTitle(variant.Title)
			if originalTitle != variant.Title {
				p.logger.Debugf("变体 %d 标题已清理: %s -> %s", i+1, originalTitle, variant.Title)
			}
		}

		p.logger.Infof("处理变体 %d: %s (ASIN: %s)", i+1, variant.Title, variant.Asin)
	}

	// 设置主产品信息（使用第一个变体的信息）
	if len(variants) > 0 && variants[0] != nil {
		mainVariant := variants[0]

		// 检查TEMU产品数据
		if temuCtx.TemuProduct != nil {
			if mainVariant.Title != "" {
				// 清理产品标题，移除特殊符号和表情符号
				cleanedTitle := utils.CleanProductTitle(mainVariant.Title)
				temuCtx.TemuProduct.GoodsBasic.GoodsName = cleanedTitle
				p.logger.Debugf("主变体标题已清理: %s -> %s", mainVariant.Title, cleanedTitle)
			}
			if mainVariant.Description != "" {
				temuCtx.TemuProduct.GoodsExtensionInfo.GoodsDesc = mainVariant.Description
			}
		}
	}

	p.logger.Info("变体数据处理完成")
	return nil
}
