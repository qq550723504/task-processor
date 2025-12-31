package handlers

import (
	"fmt"
	"strings"

	"task-processor/internal/model"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/types"
	temuUtils "task-processor/internal/platforms/temu/utils"
	"task-processor/internal/utils"

	"github.com/sirupsen/logrus"
)

// SkuItemBuilder SKU项目构建器
type SkuItemBuilder struct {
	logger         *logrus.Entry
	priceHandler   *PriceHandler
	imageProcessor *ImageProcessor
	specHandler    *SkuSpecHandler
}

// NewSkuItemBuilder 创建新的SKU项目构建器
func NewSkuItemBuilder(logger *logrus.Entry, priceHandler *PriceHandler, imageProcessor *ImageProcessor) *SkuItemBuilder {
	return &SkuItemBuilder{
		logger:         logger,
		priceHandler:   priceHandler,
		imageProcessor: imageProcessor,
		specHandler:    NewSkuSpecHandler(logger),
	}
}

// buildSkuFromVariantWithAI 使用AI映射从变体构建SKU（兼容接口）
func (ib *SkuItemBuilder) buildSkuFromVariantWithAI(ctx pipeline.TaskContext, variant *model.Product, aiSku types.AIGeneratedSku) types.Sku {
	// 类型断言为强类型上下文
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return ib.buildSkuFromVariantWithAITemu(temuCtx, variant, aiSku)
	}

	// 兼容旧接口的基本实现
	return ib.buildSkuFromVariantBasic(variant, aiSku)
}

// buildSkuFromVariantWithAITemu 使用AI映射从变体构建SKU（强类型上下文）
func (ib *SkuItemBuilder) buildSkuFromVariantWithAITemu(temuCtx *temucontext.TemuTaskContext, variant *model.Product, aiSku types.AIGeneratedSku) types.Sku {
	// 使用利润规则计算最终销售价格（已经应用了利润倍数）
	finalSalePrice := ib.priceHandler.CalculateVariantPrice(temuCtx, variant)
	basePrice := float64(finalSalePrice) / 100 // 转换为元用于显示

	// 最大零售价格就是最终销售价格（不需要再次应用倍数）
	maxRetailPrice := finalSalePrice

	// 使用原来的SKU生成逻辑
	asin := variant.Asin

	outSkuSN := ib.generateSkuFromStoreConfigTemu(temuCtx, asin)

	// 保存ASIN到SKU的映射关系到上下文，供后续SavePublishResultHandler使用
	ib.saveAsinSkuMappingTemu(temuCtx, outSkuSN, asin)

	// 从店铺配置读取库存设置（使用统一的方法）
	quantity := ib.priceHandler.GetDefaultStock(temuCtx)

	specList := aiSku.Spec

	// 去重：确保每个parent_spec_id只出现一次
	specList = ib.deduplicateSpecs(specList)

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
		// 返回空规格的SKU，让后续流程能够检测到问题
		specList = []types.SpecInfo{}
	}

	if err := ib.specHandler.ValidateSpecs(specList); err != nil {
		ib.logger.Errorf("❌ 规格验证失败: %v", err)
		ib.logger.Error("❌ 无法创建SKU，因为规格无效且不允许使用默认规格")
		// 返回空规格的SKU，让后续流程能够检测到问题
	}

	// 使用AI提取/估算的重量和尺寸（单位：lb和in）
	// 格式化重量为两位小数（TEMU API要求）
	weight := temuUtils.FormatWeight(aiSku.Weight)
	if aiSku.Weight == "" || aiSku.Weight == "0.22" {
		ib.logger.Errorf("❌ AI未能估算重量（ASIN: %s），使用兜底默认值: %slb - 这可能不准确！", variant.Asin, weight)
	} else {
		ib.logger.Infof("✅ AI提取/估算重量: %slb -> 格式化为: %slb (ASIN: %s)", aiSku.Weight, weight, variant.Asin)
	}

	// 格式化尺寸为一位小数（TEMU API要求）
	length := temuUtils.FormatDimension(aiSku.Length)
	if aiSku.Length == "" || aiSku.Length == "3.94" {
		ib.logger.Errorf("❌ AI未能估算长度（ASIN: %s），使用兜底默认值: %sin - 这可能不准确！", variant.Asin, length)
	} else {
		ib.logger.Infof("✅ AI提取/估算长度: %sin -> 格式化为: %sin (ASIN: %s)", aiSku.Length, length, variant.Asin)
	}

	width := temuUtils.FormatDimension(aiSku.Width)
	if aiSku.Width == "" || aiSku.Width == "5.91" {
		ib.logger.Errorf("❌ AI未能估算宽度（ASIN: %s），使用兜底默认值: %sin - 这可能不准确！", variant.Asin, width)
	} else {
		ib.logger.Infof("✅ AI提取/估算宽度: %sin -> 格式化为: %sin (ASIN: %s)", aiSku.Width, width, variant.Asin)
	}

	height := temuUtils.FormatDimension(aiSku.Height)
	if aiSku.Height == "" || aiSku.Height == "7.87" {
		ib.logger.Errorf("❌ AI未能估算高度（ASIN: %s），使用兜底默认值: %sin - 这可能不准确！", variant.Asin, height)
	} else {
		ib.logger.Infof("✅ AI提取/估算高度: %sin -> 格式化为: %sin (ASIN: %s)", aiSku.Height, height, variant.Asin)
	}

	// 使用AI判断的多件装信息
	multiplePackage := types.MultiplePackage{
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
		ib.logger.Warnf("⚠️ AI未提供sku_classification，使用默认值: 1 (单品)")
	}
	if multiplePackage.NumberOfPieces == 0 {
		multiplePackage.NumberOfPieces = 1
		multiplePackage.NumberOfPiecesNew = "1"
		ib.logger.Warnf("⚠️ AI未提供number_of_pieces，使用默认值: 1")
	}
	if multiplePackage.PieceUnitCode == 0 {
		multiplePackage.PieceUnitCode = 1
		multiplePackage.PieceNewUnitCode = 1
		ib.logger.Warnf("⚠️ AI未提供piece_unit_code，使用默认值: 1 (件)")
	}

	// TEMU API规则：对于单品(SkuClassification=1)，必须满足以下条件
	// - NumberOfPieces 必须为 1
	// - IndividuallyPacked 必须为 1 (yes)
	if multiplePackage.SkuClassification == 1 {
		if multiplePackage.NumberOfPieces != 1 {
			ib.logger.Warnf("⚠️ 单品的包装数量必须为1，已自动修正: %d -> 1", multiplePackage.NumberOfPieces)
			multiplePackage.NumberOfPieces = 1
			multiplePackage.NumberOfPiecesNew = "1"
		}
		if multiplePackage.IndividuallyPacked != 1 {
			ib.logger.Warnf("⚠️ 单品必须独立包装，已自动修正: %d -> 1", multiplePackage.IndividuallyPacked)
			multiplePackage.IndividuallyPacked = 1
		}
	}

	// 处理净含量信息
	var originNetContentNumber string
	var netContentUnitCode int
	if aiSku.NetContentNumber != "" {
		originNetContentNumber = aiSku.NetContentNumber
		netContentUnitCode = aiSku.NetContentUnitCode
		ib.logger.Infof("✅ AI提取的净含量信息 (ASIN: %s): %s (unit_code: %d)",
			variant.Asin, originNetContentNumber, netContentUnitCode)
	}

	// 构建图片，确保总数不超过10张
	// DimensionGallery: 使用标注过的尺寸图
	dimensionGallery, err := ib.imageProcessor.BuildDimensionImagesWithUpload(temuCtx, variant)
	if err != nil {
		ib.logger.Errorf("❌ 构建尺寸图片失败: %v", err)
		dimensionGallery = []types.ImageInfo{} // 使用空数组作为降级处理
	}

	// CarouselGallery: 使用非标注的轮播图（排除标注过的图片）
	carouselGallery, err := ib.imageProcessor.BuildCarouselImagesWithoutAnnotation(temuCtx, variant)
	if err != nil {
		ib.logger.Errorf("❌ 构建轮播图片失败: %v", err)
		carouselGallery = []types.ImageInfo{} // 使用空数组作为降级处理
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
			carouselGallery = []types.ImageInfo{}
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
	marketPrice := int(finalSalePrice * 2)                               // 市场价（分）
	marketPriceStr := fmt.Sprintf("%.2f", float64(finalSalePrice)*2/100) // 市场价字符串（元）

	return types.Sku{
		// 必需字段（按照正确的JSON格式）
		Spec:                     specList,
		Currency:                 "USD",
		UseEstimateSupplierPrice: true, // 根据正确JSON设置为true
		DimensionGallery:         dimensionGallery,
		CarouselGallery:          carouselGallery,
		FoodIngredientGallery:    []types.ImageInfo{},         // 空数组
		Quantity:                 fmt.Sprintf("%d", quantity), // 转换为字符串
		ProductExpressInfo: types.ProductExpressInfo{
			WeightInfo: types.WeightInfo{Weight: weight},
			VolumeInfo: types.VolumeInfo{Length: length, Width: width, Height: height},
		},
		SupplierPriceStr:       fmt.Sprintf("%.2f", basePrice), // 保留两位小数，单位：元
		OutSkuSN:               outSkuSN,
		MultiplePackage:        multiplePackage,
		OriginNetContentNumber: originNetContentNumber,
		NetContentUnitCode:     netContentUnitCode,
		MaxRetailPriceStr:      fmt.Sprintf("%.2f", float64(maxRetailPrice)/100),
		SupplierPrice:          int(finalSalePrice),
		MarketPrice:            marketPrice,      // 市场价（分），供货价*2
		MarketPriceStr:         marketPriceStr,   // 市场价字符串（元），供货价*2
		SkuPriceDocuments:      map[string]any{}, // 空对象
	}
}

// buildSkuFromVariantBasic 基本SKU构建（不依赖上下文）
func (ib *SkuItemBuilder) buildSkuFromVariantBasic(variant *model.Product, aiSku types.AIGeneratedSku) types.Sku {
	// 基本实现，使用默认值
	asin := variant.Asin
	outSkuSN := asin // 简单使用ASIN作为SKU

	// 默认价格和库存
	basePrice := variant.FinalPrice
	finalSalePrice := int64(basePrice * 100) // 转换为分
	quantity := 10                           // 默认库存

	specList := aiSku.Spec
	specList = ib.deduplicateSpecs(specList)

	// 使用AI提取/估算的重量和尺寸
	weight := temuUtils.FormatWeight(aiSku.Weight)
	length := temuUtils.FormatDimension(aiSku.Length)
	width := temuUtils.FormatDimension(aiSku.Width)
	height := temuUtils.FormatDimension(aiSku.Height)

	// 使用AI判断的多件装信息
	multiplePackage := types.MultiplePackage{
		SkuClassification:  aiSku.SkuClassification,
		NumberOfPieces:     aiSku.NumberOfPieces,
		IndividuallyPacked: aiSku.IndividuallyPacked,
		NumberOfPiecesNew:  fmt.Sprintf("%d", aiSku.NumberOfPieces),
		PieceUnitCode:      aiSku.PieceUnitCode,
		PieceNewUnitCode:   aiSku.PieceUnitCode,
	}

	// 默认值处理
	if multiplePackage.SkuClassification == 0 {
		multiplePackage.SkuClassification = 1
	}
	if multiplePackage.NumberOfPieces == 0 {
		multiplePackage.NumberOfPieces = 1
		multiplePackage.NumberOfPiecesNew = "1"
	}
	if multiplePackage.PieceUnitCode == 0 {
		multiplePackage.PieceUnitCode = 1
		multiplePackage.PieceNewUnitCode = 1
	}

	// 处理净含量信息
	var originNetContentNumber string
	var netContentUnitCode int
	if aiSku.NetContentNumber != "" {
		originNetContentNumber = aiSku.NetContentNumber
		netContentUnitCode = aiSku.NetContentUnitCode
	}

	// 基本图片处理（空数组）
	dimensionGallery := []types.ImageInfo{}
	carouselGallery := []types.ImageInfo{}

	// 计算市场价：最终销售价格 * 2
	marketPrice := int(finalSalePrice * 2)
	marketPriceStr := fmt.Sprintf("%.2f", float64(finalSalePrice)*2/100)

	return types.Sku{
		Spec:                     specList,
		Currency:                 "USD",
		UseEstimateSupplierPrice: true,
		DimensionGallery:         dimensionGallery,
		CarouselGallery:          carouselGallery,
		FoodIngredientGallery:    []types.ImageInfo{},
		Quantity:                 fmt.Sprintf("%d", quantity),
		ProductExpressInfo: types.ProductExpressInfo{
			WeightInfo: types.WeightInfo{Weight: weight},
			VolumeInfo: types.VolumeInfo{Length: length, Width: width, Height: height},
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
		sku.Quantity = fmt.Sprintf("%d", ib.priceHandler.GetDefaultStock(temuCtx))
	}

	// TemuProduct已经是引用，直接修改即可

	return nil
}

// generateSkuFromStoreConfig 根据店铺配置生成SKU编码（兼容接口）
func (ib *SkuItemBuilder) generateSkuFromStoreConfig(ctx pipeline.TaskContext, asin string) string {
	// 类型断言为强类型上下文
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return ib.generateSkuFromStoreConfigTemu(temuCtx, asin)
	}

	// 兼容旧接口的基本实现
	return asin // 简单使用ASIN作为SKU
}

// generateSkuFromStoreConfigTemu 根据店铺配置生成SKU编码（强类型上下文）
func (ib *SkuItemBuilder) generateSkuFromStoreConfigTemu(temuCtx *temucontext.TemuTaskContext, asin string) string {
	// 从强类型上下文获取店铺配置
	var prefix, suffix, strategyStr string
	if temuCtx.StoreInfo != nil {
		if temuCtx.StoreInfo.Prefix != "" {
			prefix = temuCtx.StoreInfo.Prefix
		}
		if temuCtx.StoreInfo.Suffix != "" {
			suffix = temuCtx.StoreInfo.Suffix
		}
		if temuCtx.StoreInfo.SkuGenerateStrategy != "" {
			strategyStr = temuCtx.StoreInfo.SkuGenerateStrategy
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
	sku := utils.GenerateSKU(asin, strategy, prefix, suffix)

	ib.logger.Infof("生成的SKU: %s (策略: %d, 前缀: %s, 后缀: %s)", sku, strategy, prefix, suffix)

	return sku
}

// deduplicateSpecs 去重规格，确保每个parent_spec_id只出现一次
func (ib *SkuItemBuilder) deduplicateSpecs(specs []types.SpecInfo) []types.SpecInfo {
	if len(specs) <= 1 {
		return specs
	}

	// 使用map记录每个parent_spec_id的第一个规格
	seenParentSpecs := make(map[string]types.SpecInfo)
	var result []types.SpecInfo

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
	// 类型断言为强类型上下文
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		ib.saveAsinSkuMappingTemu(temuCtx, outSkuSN, asin)
		return
	}

	// 兼容旧接口的基本实现
	ib.logger.Warnf("无法保存ASIN到SKU映射，不支持的上下文类型")
}

// saveAsinSkuMappingTemu 保存ASIN到SKU的映射关系到强类型上下文
func (ib *SkuItemBuilder) saveAsinSkuMappingTemu(temuCtx *temucontext.TemuTaskContext, outSkuSN string, asin string) {
	// 获取或创建映射表
	if temuCtx.AsinSkuMap == nil {
		temuCtx.AsinSkuMap = make(map[string]string)
	}

	// 保存映射关系：SKU -> ASIN
	temuCtx.AsinSkuMap[outSkuSN] = asin

	ib.logger.Debugf("保存ASIN到SKU映射: %s -> %s", outSkuSN, asin)
}
