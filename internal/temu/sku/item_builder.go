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

type skuPackagingInfo struct {
	multiplePackage        models.MultiplePackage
	originNetContentNumber string
	netContentUnitCode     int
}

type skuExpressInfo struct {
	weight             string
	length             string
	width              string
	height             string
	productExpressInfo models.ProductExpressInfo
}

type skuPricingInfo struct {
	quantity         int
	basePrice        float64
	finalSalePrice   int
	maxRetailPrice   int
	marketPrice      int
	marketPriceStr   string
	supplierPriceStr string
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
	input, err := buildSKUVariantBuildInput(temuCtx, variant, aiSku)
	if err != nil {
		ib.logger.Errorf("failed to build sku variant input: %v", err)
		return ib.buildSkuFromVariantBasic(variant, aiSku)
	}
	if input.Runtime == nil {
		ib.logger.Error("failed to build sku runtime")
		return ib.buildSkuFromVariantBasic(variant, aiSku)
	}

	pricingInfo := ib.buildSkuPricingInfo(input.Runtime, temuCtx, input.Variant)
	asin := input.Variant.Asin
	outSkuSN := ib.generateSkuFromRuntime(input.Runtime, asin)
	temuCtx.SetAsinSkuMap(ib.saveAsinSkuMappingWithRuntime(input.Runtime, outSkuSN, asin))

	specList := ib.buildSkuSpecList(input.AISKU)

	expressInfo := ib.buildSkuExpressInfo(input.Variant, input.AISKU)
	packagingInfo := ib.buildSkuPackagingInfo(input.Variant, input.AISKU)
	dimensionGallery, carouselGallery := ib.buildSkuGalleries(temuCtx, input.Variant)

	return ib.buildSkuPayload(specList, pricingInfo, outSkuSN, expressInfo, packagingInfo, dimensionGallery, carouselGallery)
}

func (ib *SkuItemBuilder) buildSkuGalleries(temuCtx *temucontext.TemuTaskContext, variant *model.Product) ([]models.ImageInfo, []models.ImageInfo) {
	dimensionGallery, err := ib.imageProcessor.BuildDimensionImagesWithUpload(temuCtx, variant)
	if err != nil {
		ib.logger.Errorf("failed to build dimension gallery: %v", err)
		dimensionGallery = []models.ImageInfo{}
	}

	carouselGallery, err := ib.imageProcessor.BuildCarouselImagesWithoutAnnotation(temuCtx, variant)
	if err != nil {
		ib.logger.Errorf("failed to build carousel gallery: %v", err)
		carouselGallery = []models.ImageInfo{}
	}

	return ib.limitSkuGalleries(dimensionGallery, carouselGallery)
}

func (ib *SkuItemBuilder) limitSkuGalleries(dimensionGallery []models.ImageInfo, carouselGallery []models.ImageInfo) ([]models.ImageInfo, []models.ImageInfo) {
	const maxTotalImages = 10
	totalImages := len(dimensionGallery) + len(carouselGallery)
	if totalImages <= maxTotalImages {
		return dimensionGallery, carouselGallery
	}

	remainingSlots := maxTotalImages - len(dimensionGallery)
	if remainingSlots < 0 {
		dimensionGallery = dimensionGallery[:maxTotalImages]
		carouselGallery = []models.ImageInfo{}
		ib.logger.Warnf("sku image count exceeded limit, keeping %d dimension images and clearing carousel images", maxTotalImages)
		return dimensionGallery, carouselGallery
	}

	if remainingSlots < len(carouselGallery) {
		carouselGallery = carouselGallery[:remainingSlots]
		ib.logger.Warnf("sku image count exceeded limit, keeping %d dimension images and trimming carousel images to %d", len(dimensionGallery), remainingSlots)
	}

	return dimensionGallery, carouselGallery
}

func (ib *SkuItemBuilder) buildSkuPackagingInfo(variant *model.Product, aiSku temucontext.AIGeneratedSku) skuPackagingInfo {
	originNetContentNumber, netContentUnitCode := ib.extractNetContentInfo(variant, aiSku)
	return skuPackagingInfo{
		multiplePackage:        ib.buildMultiplePackage(aiSku),
		originNetContentNumber: originNetContentNumber,
		netContentUnitCode:     netContentUnitCode,
	}
}

func (ib *SkuItemBuilder) buildSkuExpressInfo(variant *model.Product, aiSku temucontext.AIGeneratedSku) skuExpressInfo {
	weight, length, width, height := ib.buildProductExpressInfo(variant, aiSku)
	return skuExpressInfo{
		weight: weight,
		length: length,
		width:  width,
		height: height,
		productExpressInfo: models.ProductExpressInfo{
			WeightInfo: models.WeightInfo{Weight: weight},
			VolumeInfo: models.VolumeInfo{Length: length, Width: width, Height: height},
		},
	}
}

func (ib *SkuItemBuilder) buildSkuPricingInfo(runtime *temucontext.SKUBuildRuntime, temuCtx *temucontext.TemuTaskContext, variant *model.Product) skuPricingInfo {
	finalSalePrice := ib.priceHandler.CalculateVariantPriceWithRuntime(runtime, temuCtx, variant)
	return ib.buildSkuPricingInfoFromAmounts(float64(finalSalePrice)/100, finalSalePrice, ib.resolveDefaultSkuQuantity(runtime, temuCtx))
}

func (ib *SkuItemBuilder) buildSkuPricingInfoFromAmounts(basePrice float64, finalSalePrice int, quantity int) skuPricingInfo {
	marketPrice := finalSalePrice * 2
	return skuPricingInfo{
		quantity:         quantity,
		basePrice:        basePrice,
		finalSalePrice:   finalSalePrice,
		maxRetailPrice:   finalSalePrice,
		marketPrice:      marketPrice,
		marketPriceStr:   fmt.Sprintf("%.2f", float64(marketPrice)/100),
		supplierPriceStr: fmt.Sprintf("%.2f", basePrice),
	}
}

func (ib *SkuItemBuilder) buildSkuPayload(
	specList []models.SpecInfo,
	pricingInfo skuPricingInfo,
	outSkuSN string,
	expressInfo skuExpressInfo,
	packagingInfo skuPackagingInfo,
	dimensionGallery []models.ImageInfo,
	carouselGallery []models.ImageInfo,
) models.Sku {
	return models.Sku{
		Spec:                     specList,
		Currency:                 "USD",
		UseEstimateSupplierPrice: true,
		DimensionGallery:         dimensionGallery,
		CarouselGallery:          carouselGallery,
		FoodIngredientGallery:    []models.ImageInfo{},
		Quantity:                 fmt.Sprintf("%d", pricingInfo.quantity),
		ProductExpressInfo:       expressInfo.productExpressInfo,
		SupplierPriceStr:         pricingInfo.supplierPriceStr,
		OutSkuSN:                 outSkuSN,
		MultiplePackage:          packagingInfo.multiplePackage,
		OriginNetContentNumber:   packagingInfo.originNetContentNumber,
		NetContentUnitCode:       packagingInfo.netContentUnitCode,
		MaxRetailPriceStr:        fmt.Sprintf("%.2f", float64(pricingInfo.maxRetailPrice)/100),
		SupplierPrice:            pricingInfo.finalSalePrice,
		MarketPrice:              pricingInfo.marketPrice,
		MarketPriceStr:           pricingInfo.marketPriceStr,
		SkuPriceDocuments:        map[string]any{},
	}
}

func (ib *SkuItemBuilder) buildSkuSpecList(aiSku temucontext.AIGeneratedSku) []models.SpecInfo {
	specList := ib.deduplicateSpecs(convertSpecInfos(aiSku.Spec))
	hasTemp := false
	for i, spec := range specList {
		if strings.HasPrefix(spec.SpecID, "TEMP_") {
			ib.logger.Errorf("found unresolved temporary spec id[%d]: spec_id=%s spec_name=%s parent_spec_id=%s",
				i, spec.SpecID, spec.SpecName, spec.ParentSpecID)
			hasTemp = true
		}
	}

	if hasTemp {
		ib.logger.Error("temporary spec ids remain after spec resolution")
		return []models.SpecInfo{}
	}

	if err := ib.specHandler.ValidateSpecs(specList); err != nil {
		ib.logger.Errorf("spec validation failed: %v", err)
		ib.logger.Error("sku build cannot continue because default specs are not allowed")
	}

	return specList
}

func (ib *SkuItemBuilder) resolveDefaultSkuQuantity(runtime *temucontext.SKUBuildRuntime, temuCtx *temucontext.TemuTaskContext) int {
	if runtime == nil {
		return ib.priceHandler.GetDefaultStock(temuCtx)
	}
	return ib.priceHandler.GetDefaultStockWithRuntime(runtime)
}

// buildSkuFromVariantBasic 基本SKU构建（不依赖上下文）
func (ib *SkuItemBuilder) buildSkuFromVariantBasic(variant *model.Product, aiSku temucontext.AIGeneratedSku) models.Sku {
	asin := variant.Asin
	outSkuSN := asin

	pricingInfo := ib.buildSkuPricingInfoFromAmounts(variant.FinalPrice, int(variant.FinalPrice*100), 10)

	specList := ib.buildSkuSpecList(aiSku)
	expressInfo := ib.buildSkuExpressInfo(variant, aiSku)
	packagingInfo := ib.buildSkuPackagingInfo(variant, aiSku)

	return ib.buildSkuPayload(specList, pricingInfo, outSkuSN, expressInfo, packagingInfo, []models.ImageInfo{}, []models.ImageInfo{})
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
	input, err := buildSKUProcessInput(temuCtx, skcIndex, skuIndex)
	if err != nil {
		return err
	}

	sku := &input.Product.SkcList[input.SKCIndex].SkuList[input.SKUIndex]
	ib.validateBuiltSkuSpecs(input, sku)
	ib.ensureBuiltSkuQuantity(input, temuCtx, sku)

	return nil
}

func (ib *SkuItemBuilder) validateBuiltSkuSpecs(input *SKUProcessInput, sku *models.Sku) {
	if err := ib.specHandler.ValidateSpecs(sku.Spec); err != nil {
		ib.logger.Errorf("sku[%d][%d] spec validation failed: %v", input.SKCIndex, input.SKUIndex, err)
		ib.logger.Error("sku repair is not allowed because default specs are forbidden")
	}
}

func (ib *SkuItemBuilder) ensureBuiltSkuQuantity(input *SKUProcessInput, temuCtx *temucontext.TemuTaskContext, sku *models.Sku) {
	if sku.Quantity == "" || sku.Quantity == "0" {
		sku.Quantity = fmt.Sprintf("%d", ib.resolveDefaultSkuQuantity(input.Runtime, temuCtx))
	}
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
			temuCtx.SetAsinSkuMap(ib.saveAsinSkuMappingWithRuntime(runtime, outSkuSN, asin))
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
