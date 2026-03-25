package sku

import (
	"fmt"
	"strings"

	"task-processor/internal/model"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/skugen"
	models "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"
	temuformat "task-processor/internal/temu/format"
	"task-processor/internal/temu/image"
	"task-processor/internal/temu/product"

	"github.com/sirupsen/logrus"
)

// SkuItemBuilder SKU项目构建器
type SkuItemBuilder struct {
	logger         *logrus.Entry
	priceHandler   *product.PriceHandler
	imageProcessor *image.ImageProcessor
	specHandler    *SkuSpecHandler
}

// NewSkuItemBuilder 创建新的SKU项目构建器
func NewSkuItemBuilder(logger *logrus.Entry, priceHandler *product.PriceHandler, imageProcessor *image.ImageProcessor) *SkuItemBuilder {
	return &SkuItemBuilder{
		logger:         logger,
		priceHandler:   priceHandler,
		imageProcessor: imageProcessor,
		specHandler:    NewSkuSpecHandler(logger),
	}
}

// buildSkuFromVariantWithAI 使用AI映射从变体构建SKU（兼容接口）
func (ib *SkuItemBuilder) buildSkuFromVariantWithAI(ctx pipeline.TaskContext, variant *model.Product, aiSku temucontext.AIGeneratedSku) models.Sku {
	// 类型断言为强类型上下文
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return ib.buildSkuFromVariantWithAITemu(temuCtx, variant, aiSku)
	}

	// 兼容旧接口的基本实现
	return ib.buildSkuFromVariantBasic(variant, aiSku)
}

// buildSkuFromVariantWithAITemu 使用AI映射从变体构建SKU（强类型上下文）
func (ib *SkuItemBuilder) buildSkuFromVariantWithAITemu(temuCtx *temucontext.TemuTaskContext, variant *model.Product, aiSku temucontext.AIGeneratedSku) models.Sku {
	runtime, runtimeErr := temucontext.BuildSKUBuildRuntime(temuCtx)
	if runtimeErr != nil {
		ib.logger.Errorf("failed to build sku runtime: %v", runtimeErr)
		return ib.buildSkuFromVariantBasic(variant, aiSku)
	}

	// 使用利润规则计算最终销售价格（已经应用了利润倍数）
	finalSalePrice := ib.priceHandler.CalculateVariantPriceWithRuntime(runtime, temuCtx, variant)
	basePrice := float64(finalSalePrice) / 100 // 转换为元用于显示

	// 最大零售价格就是最终销售价格（不需要再次应用倍数）
	maxRetailPrice := finalSalePrice

	// 使用原来的SKU生成逻辑
	asin := variant.Asin

	outSkuSN := ib.generateSkuFromRuntime(runtime, asin)

	// 保存ASIN到SKU的映射关系到上下文，供后续SavePublishResultHandler使用
	temuCtx.AsinSkuMap = ib.saveAsinSkuMappingWithRuntime(runtime, outSkuSN, asin)

	// 从店铺配置读取库存设置（使用统一的方法）
	quantity := ib.priceHandler.GetDefaultStockWithRuntime(runtime)

	specList := ib.deduplicateSpecs(convertSpecInfos(aiSku.Spec))

	// 验证规格是否有效（检查是否还有临时ID）
	hasTemp := false
	for i, spec := range specList {
		if strings.HasPrefix(spec.SpecID, "TEMP_") {
			ib.logger.Errorf("❌ 发现未解析的临时规格ID[%d]: SpecID=%s, SpecName=%s, ParentSpecID=%s",
				i, spec.SpecID, spec.SpecName, spec.ParentSpecID)
			hasTemp = true
		}
	}

	if hasTemp {
		ib.logger.Error("❌ 存在未解析的临时规格ID，这表明resolveTemporarySpecIDs没有正确工作")
		specList = []models.SpecInfo{}
	}

	if err := ib.specHandler.ValidateSpecs(specList); err != nil {
		ib.logger.Errorf("❌ 规格验证失败: %v", err)
		ib.logger.Error("❌ 无法创建SKU，因为规格无效且不允许使用默认规格")
	}

	weight, length, width, height := ib.buildProductExpressInfo(variant, aiSku)
	multiplePackage := ib.buildMultiplePackage(aiSku)
	originNetContentNumber, netContentUnitCode := ib.extractNetContentInfo(variant, aiSku)

	// 构建图片，确保总数不超过10张
	// DimensionGallery: 使用标注过的尺寸图
	dimensionGallery, err := ib.imageProcessor.BuildDimensionImagesWithUpload(temuCtx, variant)
	if err != nil {
		ib.logger.Errorf("❌ 构建尺寸图片失败: %v", err)
		dimensionGallery = []models.ImageInfo{} // 使用空数组作为降级处理
	}

	// CarouselGallery: 使用非标注的轮播图（排除标注过的图片）
	carouselGallery, err := ib.imageProcessor.BuildCarouselImagesWithoutAnnotation(temuCtx, variant)
	if err != nil {
		ib.logger.Errorf("❌ 构建轮播图片失败: %v", err)
		carouselGallery = []models.ImageInfo{} // 使用空数组作为降级处理
	}

	// 限制图片总数不超过10张
	const maxTotalImages = 10
	totalImages := len(dimensionGallery) + len(carouselGallery)
	if totalImages > maxTotalImages {
		// 优先保留尺寸图，然后是轮播图
		remainingSlots := maxTotalImages - len(dimensionGallery)
		if remainingSlots < 0 {
			// 如果尺寸图就超过10张，只保留前10张尺寸图
			dimensionGallery = dimensionGallery[:maxTotalImages]
			carouselGallery = []models.ImageInfo{}
			ib.logger.Warnf("⚠️ SKU图片总数超限，尺寸图=%d，已截断为%d张，轮播图清空",
				len(dimensionGallery), maxTotalImages)
		} else if remainingSlots < len(carouselGallery) {
			// 截断轮播图
			carouselGallery = carouselGallery[:remainingSlots]
			ib.logger.Warnf("⚠️ SKU图片总数超限，尺寸图=%d，轮播图从%d截断为%d张",
				len(dimensionGallery), len(carouselGallery)+remainingSlots, remainingSlots)
		}
	}

	// 计算市场价：最终销售价格 * 2
	marketPrice := finalSalePrice * 2                                    // 市场价（分）
	marketPriceStr := fmt.Sprintf("%.2f", float64(finalSalePrice)*2/100) // 市场价字符串（元）

	return models.Sku{
		// 必需字段（按照正确的JSON格式）
		Spec:                     specList,
		Currency:                 "USD",
		UseEstimateSupplierPrice: true, // 根据正确JSON设置为true
		DimensionGallery:         dimensionGallery,
		CarouselGallery:          carouselGallery,
		FoodIngredientGallery:    []models.ImageInfo{},        // 空数组
		Quantity:                 fmt.Sprintf("%d", quantity), // 转换为字符串
		ProductExpressInfo: models.ProductExpressInfo{
			WeightInfo: models.WeightInfo{Weight: weight},
			VolumeInfo: models.VolumeInfo{Length: length, Width: width, Height: height},
		},
		SupplierPriceStr:       fmt.Sprintf("%.2f", basePrice), // 保留两位小数，单位：元
		OutSkuSN:               outSkuSN,
		MultiplePackage:        multiplePackage,
		OriginNetContentNumber: originNetContentNumber,
		NetContentUnitCode:     netContentUnitCode,
		MaxRetailPriceStr:      fmt.Sprintf("%.2f", float64(maxRetailPrice)/100),
		SupplierPrice:          finalSalePrice,
		MarketPrice:            marketPrice,      // 市场价（分），供货价*2
		MarketPriceStr:         marketPriceStr,   // 市场价字符串（元），供货价*2
		SkuPriceDocuments:      map[string]any{}, // 空对象
	}
}

// buildSkuFromVariantBasic 基本SKU构建（不依赖上下文）
func (ib *SkuItemBuilder) buildSkuFromVariantBasic(variant *model.Product, aiSku temucontext.AIGeneratedSku) models.Sku {
	asin := variant.Asin
	outSkuSN := asin

	basePrice := variant.FinalPrice
	finalSalePrice := int64(basePrice * 100)
	quantity := 10

	specList := ib.deduplicateSpecs(convertSpecInfos(aiSku.Spec))
	weight, length, width, height := ib.buildProductExpressInfo(variant, aiSku)
	multiplePackage := ib.buildMultiplePackage(aiSku)
	originNetContentNumber, netContentUnitCode := ib.extractNetContentInfo(variant, aiSku)

	marketPrice := int(finalSalePrice * 2)
	marketPriceStr := fmt.Sprintf("%.2f", float64(finalSalePrice)*2/100)

	return models.Sku{
		Spec:                     specList,
		Currency:                 "USD",
		UseEstimateSupplierPrice: true,
		DimensionGallery:         []models.ImageInfo{},
		CarouselGallery:          []models.ImageInfo{},
		FoodIngredientGallery:    []models.ImageInfo{},
		Quantity:                 fmt.Sprintf("%d", quantity),
		ProductExpressInfo: models.ProductExpressInfo{
			WeightInfo: models.WeightInfo{Weight: weight},
			VolumeInfo: models.VolumeInfo{Length: length, Width: width, Height: height},
		},
		SupplierPriceStr:       fmt.Sprintf("%.2f", basePrice),
		OutSkuSN:               outSkuSN,
		MultiplePackage:        multiplePackage,
		OriginNetContentNumber: originNetContentNumber,
		NetContentUnitCode:     netContentUnitCode,
		MaxRetailPriceStr:      fmt.Sprintf("%.2f", basePrice),
		SupplierPrice:          int(finalSalePrice),
		MarketPrice:            marketPrice,
		MarketPriceStr:         marketPriceStr,
		SkuPriceDocuments:      map[string]any{},
	}
}

// buildProductExpressInfo 构建产品物流信息（重量和尺寸）
func (ib *SkuItemBuilder) buildProductExpressInfo(variant *model.Product, aiSku temucontext.AIGeneratedSku) (weight, length, width, height string) {
	weight = temuformat.Weight(aiSku.Weight)
	if aiSku.Weight == "" || aiSku.Weight == "0.22" {
		ib.logger.Errorf("❌ AI未能估算重量（ASIN: %s），使用兜底默认值: %slb - 这可能不准确！", variant.Asin, weight)
	} else {
		ib.logger.Infof("✅ AI提取/估算重量: %slb -> 格式化为: %slb (ASIN: %s)", aiSku.Weight, weight, variant.Asin)
	}

	length = temuformat.Dimension(aiSku.Length)
	if aiSku.Length == "" || aiSku.Length == "3.94" {
		ib.logger.Errorf("❌ AI未能估算长度（ASIN: %s），使用兜底默认值: %sin - 这可能不准确！", variant.Asin, length)
	} else {
		ib.logger.Infof("✅ AI提取/估算长度: %sin -> 格式化为: %sin (ASIN: %s)", aiSku.Length, length, variant.Asin)
	}

	width = temuformat.Dimension(aiSku.Width)
	if aiSku.Width == "" || aiSku.Width == "5.91" {
		ib.logger.Errorf("❌ AI未能估算宽度（ASIN: %s），使用兜底默认值: %sin - 这可能不准确！", variant.Asin, width)
	} else {
		ib.logger.Infof("✅ AI提取/估算宽度: %sin -> 格式化为: %sin (ASIN: %s)", aiSku.Width, width, variant.Asin)
	}

	height = temuformat.Dimension(aiSku.Height)
	if aiSku.Height == "" || aiSku.Height == "7.87" {
		ib.logger.Errorf("❌ AI未能估算高度（ASIN: %s），使用兜底默认值: %sin - 这可能不准确！", variant.Asin, height)
	} else {
		ib.logger.Infof("✅ AI提取/估算高度: %sin -> 格式化为: %sin (ASIN: %s)", aiSku.Height, height, variant.Asin)
	}

	return weight, length, width, height
}

// buildMultiplePackage 构建多件装信息
func (ib *SkuItemBuilder) buildMultiplePackage(aiSku temucontext.AIGeneratedSku) models.MultiplePackage {
	mp := models.MultiplePackage{
		SkuClassification:  aiSku.SkuClassification,
		NumberOfPieces:     aiSku.NumberOfPieces,
		IndividuallyPacked: aiSku.IndividuallyPacked,
		NumberOfPiecesNew:  fmt.Sprintf("%d", aiSku.NumberOfPieces),
		PieceUnitCode:      aiSku.PieceUnitCode,
		PieceNewUnitCode:   aiSku.PieceUnitCode,
	}

	if mp.SkuClassification == 0 {
		mp.SkuClassification = 1
		ib.logger.Warnf("⚠️ AI未提供sku_classification，使用默认值: 1 (单品)")
	}
	if mp.NumberOfPieces == 0 {
		mp.NumberOfPieces = 1
		mp.NumberOfPiecesNew = "1"
		ib.logger.Warnf("⚠️ AI未提供number_of_pieces，使用默认值: 1")
	}
	if mp.PieceUnitCode == 0 {
		mp.PieceUnitCode = 1
		mp.PieceNewUnitCode = 1
		ib.logger.Warnf("⚠️ AI未提供piece_unit_code，使用默认值: 1 (件)")
	}

	// TEMU API规则：单品必须满足 NumberOfPieces=1 且 IndividuallyPacked=1
	if mp.SkuClassification == 1 {
		if mp.NumberOfPieces != 1 {
			ib.logger.Warnf("⚠️ 单品的包装数量必须为1，已自动修正: %d -> 1", mp.NumberOfPieces)
			mp.NumberOfPieces = 1
			mp.NumberOfPiecesNew = "1"
		}
		if mp.IndividuallyPacked != 1 {
			ib.logger.Warnf("⚠️ 单品必须独立包装，已自动修正: %d -> 1", mp.IndividuallyPacked)
			mp.IndividuallyPacked = 1
		}
	}

	return mp
}

// extractNetContentInfo 提取净含量信息
func (ib *SkuItemBuilder) extractNetContentInfo(variant *model.Product, aiSku temucontext.AIGeneratedSku) (string, int) {
	if aiSku.NetContentNumber == "" {
		return "", 0
	}
	ib.logger.Infof("✅ AI提取的净含量信息 (ASIN: %s): %s (unit_code: %d)",
		variant.Asin, aiSku.NetContentNumber, aiSku.NetContentUnitCode)
	return aiSku.NetContentNumber, aiSku.NetContentUnitCode
}

// processSkuItem 处理SKU项目（兼容接口）
func (ib *SkuItemBuilder) processSkuItem(ctx pipeline.TaskContext, skcIndex, skuIndex int) error {
	// 类型断言为强类型上下文
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return ib.processSkuItemTemu(temuCtx, skcIndex, skuIndex)
	}

	// 兼容旧接口的基本实现
	return fmt.Errorf("不支持的上下文类型")
}

// processSkuItemTemu 处理SKU项目（强类型上下文）
func (ib *SkuItemBuilder) processSkuItemTemu(temuCtx *temucontext.TemuTaskContext, skcIndex, skuIndex int) error {
	// 获取TEMU产品数据
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品数据不存在")
	}

	temuProduct := temuCtx.TemuProduct

	if skcIndex >= len(temuProduct.SkcList) || skuIndex >= len(temuProduct.SkcList[skcIndex].SkuList) {
		return fmt.Errorf("SKU索引超出范围")
	}

	sku := &temuProduct.SkcList[skcIndex].SkuList[skuIndex]

	// 验证SKU的规格
	if err := ib.specHandler.ValidateSpecs(sku.Spec); err != nil {
		ib.logger.Errorf("❌ SKU[%d][%d]规格验证失败: %v", skcIndex, skuIndex, err)
		ib.logger.Error("❌ 无法修复，因为不允许使用默认规格")
		// 不修复，让错误暴露出来
	}

	// 从店铺配置读取库存设置
	if sku.Quantity == "" || sku.Quantity == "0" {
		// 使用统一的库存获取方法
		runtime, err := temucontext.BuildSKUBuildRuntime(temuCtx)
		if err != nil {
			sku.Quantity = fmt.Sprintf("%d", ib.priceHandler.GetDefaultStock(temuCtx))
		} else {
			sku.Quantity = fmt.Sprintf("%d", ib.priceHandler.GetDefaultStockWithRuntime(runtime))
		}
	}

	// TemuProduct已经是引用，直接修改即可

	return nil
}

// generateSkuFromStoreConfig 根据店铺配置生成SKU编码（兼容接口）
func (ib *SkuItemBuilder) generateSkuFromStoreConfig(ctx pipeline.TaskContext, asin string) string {
	// 类型断言为强类型上下文
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		runtime, err := temucontext.BuildSKUBuildRuntime(temuCtx)
		if err == nil {
			return ib.generateSkuFromRuntime(runtime, asin)
		}
	}

	// 兼容旧接口的基本实现
	return asin // 简单使用ASIN作为SKU
}

// generateSkuFromRuntime 根据店铺配置生成SKU编码
func (ib *SkuItemBuilder) generateSkuFromRuntime(runtime *temucontext.SKUBuildRuntime, asin string) string {
	// 从强类型上下文获取店铺配置
	var prefix, suffix, strategyStr string
	if runtime != nil && runtime.StoreInfo != nil {
		if runtime.StoreInfo.Prefix != "" {
			prefix = runtime.StoreInfo.Prefix
		}
		if runtime.StoreInfo.Suffix != "" {
			suffix = runtime.StoreInfo.Suffix
		}
		if runtime.StoreInfo.SkuGenerateStrategy != "" {
			strategyStr = runtime.StoreInfo.SkuGenerateStrategy
		}
	}

	// 解析SKU生成策略
	strategy := 0 // 默认策略：仅使用ASIN
	if strategyStr != "" {
		// 策略可能是 "0", "1", "2", "3" 等字符串
		if s, err := fmt.Sscanf(strategyStr, "%d", &strategy); err != nil || s != 1 {
			ib.logger.Warnf("无法解析SKU生成策略: %s，使用默认策略", strategyStr)
			strategy = 0
		}
	}

	// 使用工具函数生成SKU
	sku := skugen.Generate(asin, strategy, prefix, suffix)

	ib.logger.Infof("生成的SKU: %s (策略: %d, 前缀: %s, 后缀: %s)", sku, strategy, prefix, suffix)

	return sku
}

// deduplicateSpecs 去重规格，确保每个parent_spec_id只出现一次
func (ib *SkuItemBuilder) deduplicateSpecs(specs []models.SpecInfo) []models.SpecInfo {
	if len(specs) <= 1 {
		return specs
	}

	// 使用map记录每个parent_spec_id的第一个规格
	seenParentSpecs := make(map[string]models.SpecInfo)
	var result []models.SpecInfo

	for _, spec := range specs {
		if _, exists := seenParentSpecs[spec.ParentSpecID]; !exists {
			// 第一次遇到这个parent_spec_id，保留
			seenParentSpecs[spec.ParentSpecID] = spec
			result = append(result, spec)
		}
	}

	if len(result) < len(specs) {
		ib.logger.Warnf("⚠️ 规格去重: 原始%d个 -> 去重后%d个", len(specs), len(result))
	}

	return result
}

// saveAsinSkuMapping 保存ASIN到SKU的映射关系到上下文（兼容接口）
func (ib *SkuItemBuilder) saveAsinSkuMapping(ctx pipeline.TaskContext, outSkuSN string, asin string) {
	// ???????????
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		runtime, err := temucontext.BuildSKUBuildRuntime(temuCtx)
		if err == nil {
			temuCtx.AsinSkuMap = ib.saveAsinSkuMappingWithRuntime(runtime, outSkuSN, asin)
			return
		}
		return
	}

	// ??????????
	ib.logger.Warnf("????ASIN?SKU????????????")
}

// saveAsinSkuMappingWithRuntime ??ASIN?SKU?????????
func (ib *SkuItemBuilder) saveAsinSkuMappingWithRuntime(runtime *temucontext.SKUBuildRuntime, outSkuSN string, asin string) map[string]string {
	if runtime == nil {
		runtime = &temucontext.SKUBuildRuntime{}
	}

	mapping := runtime.SaveAsinSkuMapping(outSkuSN, asin)
	ib.logger.Debugf("save asin sku mapping: %s -> %s", outSkuSN, asin)
	return mapping
}
