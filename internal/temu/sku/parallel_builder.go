// Package sku 提供并行SKU构建功能
package sku

import (
	"fmt"
	"strings"

	"task-processor/internal/model"
	models "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"
	"task-processor/internal/temu/image"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// SkuParallelBuilder 并行SKU构建器
type SkuParallelBuilder struct {
	itemBuilder            *SkuItemBuilder
	parallelImageProcessor *image.ParallelImageProcessor
	logger                 *logrus.Entry
}

// NewSkuParallelBuilder 创建并行SKU构建器
func NewSkuParallelBuilder(itemBuilder *SkuItemBuilder, maxWorkers int) *SkuParallelBuilder {
	return &SkuParallelBuilder{
		itemBuilder:            itemBuilder,
		parallelImageProcessor: image.NewParallelImageProcessor(maxWorkers),
		logger:                 logger.GetGlobalLogger("SkuParallelBuilder"),
	}
}

// BuildSkusWithParallelImages 并行构建SKU（图片处理并行化）
func (spb *SkuParallelBuilder) BuildSkusWithParallelImages(temuCtx *temucontext.TemuTaskContext, variants []*model.Product, aiSkus []temucontext.AIGeneratedSku) ([]models.Sku, error) {
	if len(variants) == 0 {
		return []models.Sku{}, nil
	}

	spb.logger.Infof("🚀 开始并行构建%d个SKU", len(variants))

	// 第一步：并行处理所有变体的图片
	spb.logger.Info("📸 第一步：并行处理变体图片")
	imageResults, err := spb.parallelImageProcessor.ProcessVariantImagesParallel(temuCtx, variants)
	if err != nil {
		return nil, fmt.Errorf("并行图片处理失败: %w", err)
	}

	// 第二步：串行构建SKU（除了图片处理）
	spb.logger.Info("🔧 第二步：构建SKU基础数据")
	skus := make([]models.Sku, len(variants))

	for i, variant := range variants {
		// 获取对应的AI SKU数据
		var aiSku temucontext.AIGeneratedSku
		if i < len(aiSkus) {
			aiSku = aiSkus[i]
		}

		// 构建SKU基础数据（不包含图片）
		sku := spb.buildSkuWithoutImages(temuCtx, variant, aiSku, i)
		skus[i] = sku

		spb.logger.Infof("✅ SKU[%d]基础数据构建完成: %s", i, variant.Asin)
	}

	// 第三步：应用图片处理结果
	spb.logger.Info("🖼️ 第三步：应用图片处理结果")
	spb.parallelImageProcessor.ApplyImageResults(skus, imageResults)

	spb.logger.Infof("🎉 并行SKU构建完成: %d个SKU", len(skus))
	return skus, nil
}

// buildSkuWithoutImages 构建SKU（不包含图片处理）
func (spb *SkuParallelBuilder) buildSkuWithoutImages(temuCtx *temucontext.TemuTaskContext, variant *model.Product, aiSku temucontext.AIGeneratedSku, _ int) models.Sku {
	// 使用利润规则计算最终销售价格
	finalSalePrice := spb.itemBuilder.priceHandler.CalculateVariantPrice(temuCtx, variant)

	// 生成SKU编码
	outSkuSN := spb.itemBuilder.generateSkuFromStoreConfigTemu(temuCtx, variant.Asin)

	// 保存ASIN到SKU的映射关系
	spb.itemBuilder.saveAsinSkuMappingTemu(temuCtx, outSkuSN, variant.Asin)

	// 构建规格信息
	specList := spb.buildSpecList(temuCtx, variant, aiSku)

	// 提取净含量信息
	originNetContentNumber, netContentUnitCode := spb.extractNetContentInfo(variant, aiSku)

	// 从店铺配置读取库存设置（使用统一的方法）
	quantity := spb.itemBuilder.priceHandler.GetDefaultStock(temuCtx)

	// 使用AI提取/估算的重量和尺寸（单位：lb和in）
	weight, length, width, height := spb.buildProductExpressInfo(variant, aiSku)

	// 使用AI判断的多件装信息
	multiplePackage := spb.buildMultiplePackage(variant, aiSku)

	// 计算市场价
	marketPrice := finalSalePrice * 2
	marketPriceStr := fmt.Sprintf("%.2f", float64(finalSalePrice)*2/100)

	// 构建SKU（不包含图片，图片将在后续步骤中添加）
	return models.Sku{
		Spec:                     specList,
		Currency:                 "USD",
		UseEstimateSupplierPrice: true,
		DimensionGallery:         []models.ImageInfo{}, // 稍后填充
		CarouselGallery:          []models.ImageInfo{}, // 稍后填充
		FoodIngredientGallery:    []models.ImageInfo{},
		Quantity:                 fmt.Sprintf("%d", quantity),
		ProductExpressInfo: models.ProductExpressInfo{
			WeightInfo: models.WeightInfo{Weight: weight},
			VolumeInfo: models.VolumeInfo{Length: length, Width: width, Height: height},
		},
		SupplierPriceStr:       fmt.Sprintf("%.2f", float64(finalSalePrice)/100),
		OutSkuSN:               outSkuSN,
		MultiplePackage:        multiplePackage,
		OriginNetContentNumber: originNetContentNumber,
		NetContentUnitCode:     netContentUnitCode,
		MaxRetailPriceStr:      marketPriceStr,
		SupplierPrice:          finalSalePrice,
		SkuPriceDocuments:      make(map[string]any),
		MarketPrice:            marketPrice,
		MarketPriceStr:         marketPriceStr,
	}
}

// buildSpecList 构建规格列表
func (spb *SkuParallelBuilder) buildSpecList(_ *temucontext.TemuTaskContext, _ *model.Product, aiSku temucontext.AIGeneratedSku) []models.SpecInfo {
	specList := spb.itemBuilder.deduplicateSpecs(convertSpecInfos(aiSku.Spec))

	// 验证规格是否有效（检查是否还有临时ID）
	hasTemp := false
	for i, spec := range specList {
		if strings.HasPrefix(spec.SpecID, "TEMP_") {
			spb.logger.Errorf("❌ 发现未解析的临时规格ID[%d]: SpecID=%s, SpecName=%s, ParentSpecID=%s",
				i, spec.SpecID, spec.SpecName, spec.ParentSpecID)
			hasTemp = true
		}
	}

	if hasTemp {
		spb.logger.Error("❌ 存在未解析的临时规格ID，这表明resolveTemporarySpecIDs没有正确工作")
		return []models.SpecInfo{}
	}

	if err := spb.itemBuilder.specHandler.ValidateSpecs(specList); err != nil {
		spb.logger.Errorf("❌ 规格验证失败: %v", err)
		spb.logger.Error("❌ 无法创建SKU，因为规格无效且不允许使用默认规格")
	}

	return specList
}

// buildProductExpressInfo 委托给 SkuItemBuilder
func (spb *SkuParallelBuilder) buildProductExpressInfo(variant *model.Product, aiSku temucontext.AIGeneratedSku) (weight, length, width, height string) {
	return spb.itemBuilder.buildProductExpressInfo(variant, aiSku)
}

// buildMultiplePackage 委托给 SkuItemBuilder
func (spb *SkuParallelBuilder) buildMultiplePackage(_ *model.Product, aiSku temucontext.AIGeneratedSku) models.MultiplePackage {
	return spb.itemBuilder.buildMultiplePackage(aiSku)
}

// extractNetContentInfo 委托给 SkuItemBuilder
func (spb *SkuParallelBuilder) extractNetContentInfo(variant *model.Product, aiSku temucontext.AIGeneratedSku) (string, int) {
	return spb.itemBuilder.extractNetContentInfo(variant, aiSku)
}
