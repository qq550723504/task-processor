// Package handlers 提供并行SKU构建功能
package sku

import (
	"fmt"
	"strings"

	"task-processor/internal/domain/model"
	models "task-processor/internal/platforms/temu/api/product"
	temucontext "task-processor/internal/platforms/temu/context"
	temuformat "task-processor/internal/platforms/temu/format"
	"task-processor/internal/platforms/temu/handlers/image"
	"task-processor/internal/platforms/temu/types"

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
		logger:                 logrus.WithField("component", "SkuParallelBuilder"),
	}
}

// BuildSkusWithParallelImages 并行构建SKU（图片处理并行化）
func (spb *SkuParallelBuilder) BuildSkusWithParallelImages(temuCtx *temucontext.TemuTaskContext, variants []*model.Product, aiSkus []types.AIGeneratedSku) ([]models.Sku, error) {
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
		var aiSku types.AIGeneratedSku
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
func (spb *SkuParallelBuilder) buildSkuWithoutImages(temuCtx *temucontext.TemuTaskContext, variant *model.Product, aiSku types.AIGeneratedSku, index int) models.Sku {
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
	marketPrice := int(finalSalePrice * 2)
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
		SupplierPrice:          int(finalSalePrice),
		SkuPriceDocuments:      make(map[string]any),
		MarketPrice:            marketPrice,
		MarketPriceStr:         marketPriceStr,
	}
}

// buildSpecList 构建规格列表
func (spb *SkuParallelBuilder) buildSpecList(temuCtx *temucontext.TemuTaskContext, variant *model.Product, aiSku types.AIGeneratedSku) []models.SpecInfo {
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

// buildProductExpressInfo 构建产品物流信息（重量和尺寸）
func (spb *SkuParallelBuilder) buildProductExpressInfo(variant *model.Product, aiSku types.AIGeneratedSku) (weight, length, width, height string) {
	// 使用AI提取/估算的重量和尺寸（单位：lb和in）
	// 格式化重量为两位小数（TEMU API要求）
	weight = temuformat.Weight(aiSku.Weight)
	if aiSku.Weight == "" || aiSku.Weight == "0.22" {
		spb.logger.Errorf("❌ AI未能估算重量（ASIN: %s），使用兜底默认值: %slb - 这可能不准确！", variant.Asin, weight)
	} else {
		spb.logger.Infof("✅ AI提取/估算重量: %slb -> 格式化为: %slb (ASIN: %s)", aiSku.Weight, weight, variant.Asin)
	}

	// 格式化尺寸为一位小数（TEMU API要求）
	length = temuformat.Dimension(aiSku.Length)
	if aiSku.Length == "" || aiSku.Length == "3.94" {
		spb.logger.Errorf("❌ AI未能估算长度（ASIN: %s），使用兜底默认值: %sin - 这可能不准确！", variant.Asin, length)
	} else {
		spb.logger.Infof("✅ AI提取/估算长度: %sin -> 格式化为: %sin (ASIN: %s)", aiSku.Length, length, variant.Asin)
	}

	width = temuformat.Dimension(aiSku.Width)
	if aiSku.Width == "" || aiSku.Width == "5.91" {
		spb.logger.Errorf("❌ AI未能估算宽度（ASIN: %s），使用兜底默认值: %sin - 这可能不准确！", variant.Asin, width)
	} else {
		spb.logger.Infof("✅ AI提取/估算宽度: %sin -> 格式化为: %sin (ASIN: %s)", aiSku.Width, width, variant.Asin)
	}

	height = temuformat.Dimension(aiSku.Height)
	if aiSku.Height == "" || aiSku.Height == "7.87" {
		spb.logger.Errorf("❌ AI未能估算高度（ASIN: %s），使用兜底默认值: %sin - 这可能不准确！", variant.Asin, height)
	} else {
		spb.logger.Infof("✅ AI提取/估算高度: %sin -> 格式化为: %sin (ASIN: %s)", aiSku.Height, height, variant.Asin)
	}

	return weight, length, width, height
}

// buildMultiplePackage 构建多件装信息
func (spb *SkuParallelBuilder) buildMultiplePackage(variant *model.Product, aiSku types.AIGeneratedSku) models.MultiplePackage {
	// 使用AI判断的多件装信息
	multiplePackage := models.MultiplePackage{
		SkuClassification:  aiSku.SkuClassification,
		NumberOfPieces:     aiSku.NumberOfPieces,
		IndividuallyPacked: aiSku.IndividuallyPacked,
		NumberOfPiecesNew:  fmt.Sprintf("%d", aiSku.NumberOfPieces),
		PieceUnitCode:      aiSku.PieceUnitCode,
		PieceNewUnitCode:   aiSku.PieceUnitCode,
	}

	// 如果AI没有提供值，使用默认值
	if multiplePackage.SkuClassification == 0 {
		multiplePackage.SkuClassification = 1
		spb.logger.Warnf("⚠️ AI未提供sku_classification，使用默认值: 1 (单品)")
	}
	if multiplePackage.NumberOfPieces == 0 {
		multiplePackage.NumberOfPieces = 1
		multiplePackage.NumberOfPiecesNew = "1"
		spb.logger.Warnf("⚠️ AI未提供number_of_pieces，使用默认值: 1")
	}
	if multiplePackage.PieceUnitCode == 0 {
		multiplePackage.PieceUnitCode = 1
		multiplePackage.PieceNewUnitCode = 1
		spb.logger.Warnf("⚠️ AI未提供piece_unit_code，使用默认值: 1 (件)")
	}

	// TEMU API规则：对于单品(SkuClassification=1)，必须满足以下条件
	// - NumberOfPieces 必须为 1
	// - IndividuallyPacked 必须为 1 (yes)
	if multiplePackage.SkuClassification == 1 {
		if multiplePackage.NumberOfPieces != 1 {
			spb.logger.Warnf("⚠️ 单品的包装数量必须为1，已自动修正: %d -> 1", multiplePackage.NumberOfPieces)
			multiplePackage.NumberOfPieces = 1
			multiplePackage.NumberOfPiecesNew = "1"
		}
		if multiplePackage.IndividuallyPacked != 1 {
			spb.logger.Warnf("⚠️ 单品必须独立包装，已自动修正: %d -> 1", multiplePackage.IndividuallyPacked)
			multiplePackage.IndividuallyPacked = 1
		}
	}

	return multiplePackage
}

// extractNetContentInfo 提取净含量信息
func (spb *SkuParallelBuilder) extractNetContentInfo(variant *model.Product, aiSku types.AIGeneratedSku) (string, int) {
	var originNetContentNumber string
	var netContentUnitCode int

	// 从AI映射中提取净含量信息
	if aiSku.NetContentNumber != "" {
		originNetContentNumber = aiSku.NetContentNumber
		netContentUnitCode = aiSku.NetContentUnitCode
		spb.logger.Infof("✅ AI提取的净含量信息 (ASIN: %s): %s (unit_code: %d)",
			variant.Asin, originNetContentNumber, netContentUnitCode)
	}

	return originNetContentNumber, netContentUnitCode
}
