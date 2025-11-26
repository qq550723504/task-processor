package handlers

import (
	"fmt"
	"strings"

	"task-processor/common/amazon"
	"task-processor/common/pipeline"
	"task-processor/common/utils"
	"task-processor/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// SkuItemBuilder SKU项目构建器
type SkuItemBuilder struct {
	logger         *logrus.Entry
	priceHandler   *PriceHandler
	regionHandler  *RegionHandler
	imageProcessor *ImageProcessor
	specHandler    *SkuSpecHandler
}

// NewSkuItemBuilder 创建新的SKU项目构建器
func NewSkuItemBuilder(logger *logrus.Entry, priceHandler *PriceHandler, regionHandler *RegionHandler, imageProcessor *ImageProcessor) *SkuItemBuilder {
	return &SkuItemBuilder{
		logger:         logger,
		priceHandler:   priceHandler,
		regionHandler:  regionHandler,
		imageProcessor: imageProcessor,
		specHandler:    NewSkuSpecHandler(logger),
	}
}

// buildSkuFromVariantWithAI 使用AI映射从变体构建SKU
func (ib *SkuItemBuilder) buildSkuFromVariantWithAI(ctx *pipeline.TaskContext, variant *amazon.Product, aiSku AIGeneratedSku) types.Sku {
	// 使用利润规则计算价格
	supplierPrice := ib.priceHandler.CalculateVariantPriceWithStoreConfig(ctx, variant)
	basePrice := float64(supplierPrice) / 100 // 转换为元用于显示

	// 获取利润规则的价格倍数
	priceMultiplier := ib.priceHandler.GetPriceMultiplier(ctx)

	// 初始零售价格（将由PriceQueryHandler更新为正确值）
	maxRetailPrice := int(float64(supplierPrice) * priceMultiplier) // 使用利润规则倍数

	ib.logger.Infof("💰 SKU价格计算 (ASIN: %s): 供应商价格=%.2f元, 倍数=%.2fx, 临时零售价=%.2f元",
		variant.Asin, basePrice, priceMultiplier, float64(maxRetailPrice)/100)

	// 使用原来的SKU生成逻辑
	asin := variant.Asin
	if asin == "" {
		asin = aiSku.UniqueID // 如果ASIN为空，使用unique_id作为备选
	}
	outSkuSN := ib.generateSkuFromStoreConfig(ctx, asin)

	// 保存ASIN到SKU的映射关系到上下文，供后续SavePublishResultHandler使用
	ib.saveAsinSkuMapping(ctx, outSkuSN, asin)

	// 从店铺配置读取库存设置（使用统一的方法）
	quantity := ib.priceHandler.GetDefaultStock(ctx)

	// 直接使用aiSku.Spec，不要复制！
	// 这样resolveTemporarySpecIDs的修改才能传递到最终的SKU
	// 注意：aiSku.Spec已经在resolveTemporarySpecIDs中被解析过了
	specList := aiSku.Spec

	// 调试：记录接收到的spec
	ib.logger.Debugf("📋 buildSkuFromVariantWithAI 接收到的规格: %+v", specList)

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
	weight := aiSku.Weight
	if weight == "" || weight == "0.22" {
		weight = "0.22" // 默认0.22磅（约100克）
		ib.logger.Errorf("❌ AI未能估算重量（ASIN: %s），使用兜底默认值: %slb - 这可能不准确！", variant.Asin, weight)
	} else {
		ib.logger.Infof("✅ AI提取/估算重量: %slb (ASIN: %s)", weight, variant.Asin)
	}

	length := aiSku.Length
	if length == "" || length == "3.94" {
		length = "3.94" // 默认3.94英寸（约100毫米）
		ib.logger.Errorf("❌ AI未能估算长度（ASIN: %s），使用兜底默认值: %sin - 这可能不准确！", variant.Asin, length)
	} else {
		ib.logger.Infof("✅ AI提取/估算长度: %sin (ASIN: %s)", length, variant.Asin)
	}

	width := aiSku.Width
	if width == "" || width == "5.91" {
		width = "5.91" // 默认5.91英寸（约150毫米）
		ib.logger.Errorf("❌ AI未能估算宽度（ASIN: %s），使用兜底默认值: %sin - 这可能不准确！", variant.Asin, width)
	} else {
		ib.logger.Infof("✅ AI提取/估算宽度: %sin (ASIN: %s)", width, variant.Asin)
	}

	height := aiSku.Height
	if height == "" || height == "7.87" {
		height = "7.87" // 默认7.87英寸（约200毫米）
		ib.logger.Errorf("❌ AI未能估算高度（ASIN: %s），使用兜底默认值: %sin - 这可能不准确！", variant.Asin, height)
	} else {
		ib.logger.Infof("✅ AI提取/估算高度: %sin (ASIN: %s)", height, variant.Asin)
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

	ib.logger.Infof("✅ AI判断的多件装信息 (ASIN: %s): classification=%d, pieces=%d, unit=%d, individually_packed=%d",
		variant.Asin, multiplePackage.SkuClassification, multiplePackage.NumberOfPieces,
		multiplePackage.PieceUnitCode, multiplePackage.IndividuallyPacked)

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
	dimensionGallery := ib.imageProcessor.BuildDimensionImagesWithUpload(ctx, variant)
	carouselGallery := ib.imageProcessor.BuildVariantImagesWithUpload(ctx, variant)

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

	ib.logger.Infof("📸 SKU图片: 尺寸图=%d张, 轮播图=%d张, 总计=%d张",
		len(dimensionGallery), len(carouselGallery), len(dimensionGallery)+len(carouselGallery))

	// 计算市场价：供货价 * 2
	marketPrice := supplierPrice * 2                                    // 市场价（分）
	marketPriceStr := fmt.Sprintf("%.2f", float64(supplierPrice)*2/100) // 市场价字符串（元）

	ib.logger.Infof("💰 市场价计算: 供货价=%.2f元, 市场价=%.2f元 (供货价*2)",
		basePrice, float64(marketPrice)/100)

	return types.Sku{
		// 必需字段（按照正确的JSON格式）
		Spec:                     specList,
		Currency:                 ib.regionHandler.GetCurrencyByRegion(ctx.Task.Region),
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
		SupplierPrice:          supplierPrice,
		MarketPrice:            marketPrice,      // 市场价（分），供货价*2
		MarketPriceStr:         marketPriceStr,   // 市场价字符串（元），供货价*2
		SkuPriceDocuments:      map[string]any{}, // 空对象
	}
}

// buildDeletedSku 构建删除状态的SKU
func (ib *SkuItemBuilder) buildDeletedSku(ctx *pipeline.TaskContext, specs []types.SpecInfo) types.Sku {
	// 验证规格是否有效
	if err := ib.specHandler.ValidateSpecs(specs); err != nil {
		ib.logger.Errorf("❌ 删除SKU的规格验证失败: %v", err)
		ib.logger.Error("❌ 删除SKU也必须有有效的规格")
		// 保持原有的specs，即使无效
	}

	return types.Sku{
		Spec:                     specs,
		Currency:                 ib.regionHandler.GetCurrencyByRegion(ctx.Task.Region),
		UseEstimateSupplierPrice: true,
		DimensionGallery:         []types.ImageInfo{},
		CarouselGallery:          []types.ImageInfo{},
		FoodIngredientGallery:    []types.ImageInfo{},
		SkuDeleted:               true,  // 标记为已删除
		Quantity:                 "100", // 保持一致的数量格式
		SupplierPriceStr:         "0",   // 价格为0
		ProductExpressInfo: types.ProductExpressInfo{
			WeightInfo: types.WeightInfo{
				Weight: "", // 删除的SKU重量为空
			},
			VolumeInfo: types.VolumeInfo{
				Length: "", // 删除的SKU尺寸为空
				Width:  "",
				Height: "",
			},
		},
		MultiplePackage: types.MultiplePackage{
			SkuClassification:  1,
			NumberOfPieces:     1,
			IndividuallyPacked: 1,
			PieceUnitCode:      1,
			NumberOfPiecesNew:  "1",
			PieceNewUnitCode:   1,
		},
		MaxRetailPriceStr: "0",
		SupplierPrice:     0,
		MarketPrice:       0,   // 删除的SKU市场价为0
		MarketPriceStr:    "0", // 删除的SKU市场价字符串为0
		SkuPriceDocuments: make(map[string]any),
	}
}

// processSkuItem 处理SKU项目
func (ib *SkuItemBuilder) processSkuItem(ctx *pipeline.TaskContext, skcIndex, skuIndex int) error {
	sku := &ctx.TemuProduct.SkcList[skcIndex].SkuList[skuIndex]

	// 验证SKU的规格
	if err := ib.specHandler.ValidateSpecs(sku.Spec); err != nil {
		ib.logger.Errorf("❌ SKU[%d][%d]规格验证失败: %v", skcIndex, skuIndex, err)
		ib.logger.Error("❌ 无法修复，因为不允许使用默认规格")
		// 不修复，让错误暴露出来
	}

	// 从店铺配置读取库存设置
	if sku.Quantity == "" || sku.Quantity == "0" {
		// 使用统一的库存获取方法
		sku.Quantity = fmt.Sprintf("%d", ib.priceHandler.GetDefaultStock(ctx))
	}

	// 设置货币
	if sku.Currency == "" {
		sku.Currency = ib.regionHandler.GetCurrencyByRegion(ctx.Task.Region)
	}

	return nil
}

// generateSkuFromStoreConfig 根据店铺配置生成SKU编码
func (ib *SkuItemBuilder) generateSkuFromStoreConfig(ctx *pipeline.TaskContext, asin string) string {
	// 从店铺配置读取前后缀和生成策略
	if ctx.StoreInfo == nil {
		ib.logger.Warn("店铺信息为空，直接使用ASIN作为SKU")
		return asin
	}

	prefix := ctx.StoreInfo.Prefix
	suffix := ctx.StoreInfo.Suffix
	strategy := 0 // 默认策略：仅使用ASIN

	// 解析SKU生成策略
	if ctx.StoreInfo.SkuGenerateStrategy != "" {
		// 策略可能是 "0", "1", "2", "3" 等字符串
		if s, err := fmt.Sscanf(ctx.StoreInfo.SkuGenerateStrategy, "%d", &strategy); err != nil || s != 1 {
			ib.logger.Warnf("无法解析SKU生成策略: %s，使用默认策略", ctx.StoreInfo.SkuGenerateStrategy)
			strategy = 0
		}
	}

	ib.logger.Infof("SKU生成配置: prefix=%s, suffix=%s, strategy=%d, asin=%s",
		prefix, suffix, strategy, asin)

	// 使用工具函数生成SKU
	sku := utils.GenerateSKU(asin, strategy, prefix, suffix)

	ib.logger.Infof("生成的SKU: %s", sku)

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
		} else {
			// 重复的parent_spec_id，记录警告
			existingSpec := seenParentSpecs[spec.ParentSpecID]
			ib.logger.Warnf("⚠️ 检测到重复的规格维度 parent_spec_id=%s (%s):",
				spec.ParentSpecID, spec.ParentSpecName)
			ib.logger.Warnf("   保留: %s (spec_id=%s)", existingSpec.SpecName, existingSpec.SpecID)
			ib.logger.Warnf("   丢弃: %s (spec_id=%s)", spec.SpecName, spec.SpecID)
		}
	}

	if len(result) < len(specs) {
		ib.logger.Warnf("⚠️ 规格去重: 原始%d个 -> 去重后%d个", len(specs), len(result))
	}

	return result
}

// saveAsinSkuMapping 保存ASIN到SKU的映射关系到上下文
func (ib *SkuItemBuilder) saveAsinSkuMapping(ctx *pipeline.TaskContext, outSkuSN string, asin string) {
	// 获取或创建映射表
	var asinSkuMap map[string]string
	if mapData, exists := ctx.GetData("asin_sku_map"); exists {
		if existingMap, ok := mapData.(map[string]string); ok {
			asinSkuMap = existingMap
		} else {
			ib.logger.Warn("asin_sku_map类型不正确，重新创建")
			asinSkuMap = make(map[string]string)
		}
	} else {
		asinSkuMap = make(map[string]string)
	}

	// 保存映射关系：SKU -> ASIN
	asinSkuMap[outSkuSN] = asin
	ctx.SetData("asin_sku_map", asinSkuMap)

	ib.logger.Debugf("保存ASIN到SKU映射: SKU=%s -> ASIN=%s", outSkuSN, asin)
}
