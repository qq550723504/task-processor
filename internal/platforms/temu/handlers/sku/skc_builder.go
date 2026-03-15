package sku

import (
	"task-processor/internal/domain/model"
	"task-processor/internal/pipeline"
	models "task-processor/internal/platforms/temu/api/product"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// SkuSkcBuilder SKC构建器
type SkuSkcBuilder struct {
	logger          *logrus.Entry
	specHandler     *SkuSpecHandler
	itemBuilder     *SkuItemBuilder
	parallelBuilder *SkuParallelBuilder
}

// NewSkuSkcBuilder 创建新的SKC构建器
func NewSkuSkcBuilder(logger *logrus.Entry, itemBuilder *SkuItemBuilder) *SkuSkcBuilder {
	return &SkuSkcBuilder{
		logger:          logger,
		specHandler:     NewSkuSpecHandler(logger),
		itemBuilder:     itemBuilder,
		parallelBuilder: nil, // 延迟初始化，需要配置信息
	}
}

// initParallelBuilder 初始化并行构建器（使用默认worker数量）
func (sb *SkuSkcBuilder) initParallelBuilder() {
	if sb.parallelBuilder != nil {
		return // 已经初始化
	}

	// 使用默认的并行worker数量（与浏览器池大小保持一致）
	maxWorkers := 3 // 默认值，与配置文件中的browser.poolSize保持一致

	sb.parallelBuilder = NewSkuParallelBuilder(sb.itemBuilder, maxWorkers)
	sb.logger.Infof("✅ 并行SKU构建器初始化完成，worker数量: %d", maxWorkers)
}

// buildMultipleSkcsFromTemplate 使用模板属性构建多个SKC（按主变体分组）
func (sb *SkuSkcBuilder) buildMultipleSkcsFromTemplate(ctx pipeline.TaskContext, variants []*model.Product, aiMapping *types.AISkuMappingResponse, templateSpecs []types.TemplateRespGoodsSpecProperty) []models.Skc {
	// 按颜色分组SKU
	colorGroups := make(map[string][]int)
	for i, aiSku := range aiMapping.SkuList {
		colorSpecID := aiSku.ColorSpecID
		if colorSpecID == "" {
			colorSpecID = "default"
		}
		colorGroups[colorSpecID] = append(colorGroups[colorSpecID], i)
	}

	skcList := make([]models.Skc, 0, len(colorGroups))

	// 为每个颜色组创建一个SKC
	for colorSpecID, skuIndices := range colorGroups {
		// 生成该颜色组的完整SKU列表
		skuList := sb.buildCompleteSkuListForColor(ctx, variants, aiMapping, skuIndices, colorSpecID, templateSpecs)

		// 创建SKC
		skc := models.Skc{
			SkuList: skuList,
		}

		skcList = append(skcList, skc)
	}

	return skcList
}

// buildCompleteSkuListForColor 为特定颜色构建完整的SKU列表
func (sb *SkuSkcBuilder) buildCompleteSkuListForColor(ctx pipeline.TaskContext, variants []*model.Product, aiMapping *types.AISkuMappingResponse, skuIndices []int, colorSpecID string, templateSpecs []types.TemplateRespGoodsSpecProperty) []models.Sku {
	// 收集该颜色下所有可能的非颜色规格组合
	nonColorSpecs := sb.specHandler.collectNonColorSpecsForColor(aiMapping, skuIndices, templateSpecs)

	sb.logger.Infof("收集到 %d 个非颜色规格维度", len(nonColorSpecs))

	// 如果没有非颜色规格，直接使用现有的SKU（只有颜色规格）
	if len(nonColorSpecs) == 0 {
		sb.logger.Info("没有非颜色规格，直接使用现有SKU")
		skuList := make([]models.Sku, 0, len(skuIndices))
		for _, skuIndex := range skuIndices {
			variant := variants[skuIndex]
			aiSku := aiMapping.SkuList[skuIndex]
			sku := sb.itemBuilder.buildSkuFromVariantWithAI(ctx, variant, aiSku)
			skuList = append(skuList, sku)
			sb.logger.Infof("使用现有变体: 颜色=%s, index=%d", colorSpecID, skuIndex)
		}
		return skuList
	}

	// 生成所有可能的非颜色规格组合
	allNonColorCombinations := sb.specHandler.generateNonColorSpecCombinations(nonColorSpecs)
	sb.logger.Infof("生成了 %d 个非颜色规格组合", len(allNonColorCombinations))

	// 创建现有SKU的映射
	existingSkuMap := make(map[string]int)
	for _, skuIndex := range skuIndices {
		aiSku := aiMapping.SkuList[skuIndex]
		key := sb.specHandler.createNonColorSpecKey(convertSpecInfos(aiSku.Spec))
		existingSkuMap[key] = skuIndex
		sb.logger.Debugf("存储现有SKU映射: key=%s, index=%d", key, skuIndex)
	}

	sb.logger.Infof("现有SKU映射数量: %d", len(existingSkuMap))

	var skuList []models.Sku

	// 为每个可能的非颜色规格组合创建SKU
	for _, nonColorCombination := range allNonColorCombinations {
		combinationKey := sb.specHandler.createSpecCombinationKeyFromSpecs(nonColorCombination)
		sb.logger.Debugf("检查组合: key=%s", combinationKey)

		if existingIndex, exists := existingSkuMap[combinationKey]; exists {
			// 存在的变体，使用真实数据
			variant := variants[existingIndex]
			aiSku := aiMapping.SkuList[existingIndex]
			sku := sb.itemBuilder.buildSkuFromVariantWithAI(ctx, variant, aiSku)
			skuList = append(skuList, sku)
			sb.logger.Infof("使用现有变体: 颜色=%s, 组合=%s", colorSpecID, combinationKey)
		} else {
			// 缺失的变体，创建删除状态的SKU
			// 需要添加颜色规格到组合中
			// fullSpecs := sb.specHandler.addColorSpecToCombination(nonColorCombination, colorSpecID, aiMapping, skuIndices)
			// sku := sb.itemBuilder.buildDeletedSku(ctx, fullSpecs)
			// skuList = append(skuList, sku)
			sb.logger.Warnf("创建缺失变体: 颜色=%s, 组合=%s", colorSpecID, combinationKey)
		}
	}

	return skuList
}

// buildSingleSkcFromUserInput 使用规格构建单个SKC（包含多个SKU）
func (sb *SkuSkcBuilder) buildSingleSkcFromUserInput(ctx pipeline.TaskContext, variants []*model.Product, aiMapping *types.AISkuMappingResponse, templateSpecs []types.TemplateRespGoodsSpecProperty) []models.Skc {
	// 注意：templateSpecs 参数保留用于未来可能的规格验证和修复功能
	// 当前实现中暂未使用，因为AI已经能够生成合理的规格组合
	_ = templateSpecs // 明确标记参数暂未使用

	// 类型断言为强类型上下文，尝试使用并行处理
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return sb.buildSingleSkcWithParallelImages(temuCtx, variants, aiMapping)
	}

	// 兼容旧接口的串行实现
	return sb.buildSingleSkcSerial(ctx, variants, aiMapping)
}

// buildSingleSkcWithParallelImages 使用并行图片处理构建单个SKC
func (sb *SkuSkcBuilder) buildSingleSkcWithParallelImages(temuCtx *temucontext.TemuTaskContext, variants []*model.Product, aiMapping *types.AISkuMappingResponse) []models.Skc {
	sb.logger.Infof("🚀 开始并行构建单个SKC，包含%d个变体", len(variants))

	// 初始化并行构建器
	sb.initParallelBuilder()

	// 准备AI SKU数据
	asinMap := make(map[string]bool) // 用于去重，基于ASIN而不是规格组合
	validVariants := make([]*model.Product, 0, len(variants))
	validAiSkus := make([]types.AIGeneratedSku, 0, len(variants))

	// 过滤重复的ASIN
	for i, variant := range variants {
		// 边界检查：防止数组越界
		if i >= len(aiMapping.SkuList) {
			sb.logger.Errorf("❌ 变体索引[%d]超出AI映射范围(长度=%d)，跳过该变体: ASIN=%s",
				i, len(aiMapping.SkuList), variant.Asin)
			continue
		}

		aiSku := aiMapping.SkuList[i]

		// 检查ASIN是否重复
		if asinMap[variant.Asin] {
			sb.logger.Warnf("⚠️ 跳过重复的ASIN[%d]: %s", i, variant.Asin)
			continue
		}
		asinMap[variant.Asin] = true

		validVariants = append(validVariants, variant)
		validAiSkus = append(validAiSkus, aiSku)
		sb.logger.Infof("为变体[%d]准备SKU，规格: %+v", i, aiSku.Spec)
	}

	sb.logger.Infof("SKU去重完成: 原始=%d, 去重后=%d", len(variants), len(validVariants))

	// 使用并行构建器构建SKU
	skuList, err := sb.parallelBuilder.BuildSkusWithParallelImages(temuCtx, validVariants, validAiSkus)
	if err != nil {
		sb.logger.Errorf("❌ 并行SKU构建失败: %v，回退到串行处理", err)
		// 回退到串行处理
		return sb.buildSingleSkcSerial(temuCtx, validVariants, &types.AISkuMappingResponse{SkuList: validAiSkus})
	}

	// 创建单个SKC
	skc := models.Skc{
		SkuList: skuList,
	}

	sb.logger.Infof("🎉 并行SKC构建完成: %d个SKU", len(skuList))
	return []models.Skc{skc}
}

// buildSingleSkcSerial 串行构建单个SKC（兼容旧接口）
func (sb *SkuSkcBuilder) buildSingleSkcSerial(ctx pipeline.TaskContext, variants []*model.Product, aiMapping *types.AISkuMappingResponse) []models.Skc {
	var skuList []models.Sku
	asinMap := make(map[string]bool) // 用于去重，基于ASIN而不是规格组合

	// 直接为每个变体创建SKU，不生成缺失的组合
	// 因为这是单SKC模式，所有变体都在同一个SKC中
	for i, variant := range variants {
		// 边界检查：防止数组越界
		if i >= len(aiMapping.SkuList) {
			sb.logger.Errorf("❌ 变体索引[%d]超出AI映射范围(长度=%d)，跳过该变体: ASIN=%s",
				i, len(aiMapping.SkuList), variant.Asin)
			continue
		}

		aiSku := aiMapping.SkuList[i]

		// 检查ASIN是否重复
		if asinMap[variant.Asin] {
			sb.logger.Warnf("⚠️ 跳过重复的ASIN[%d]: %s", i, variant.Asin)
			continue
		}
		asinMap[variant.Asin] = true

		sku := sb.itemBuilder.buildSkuFromVariantWithAI(ctx, variant, aiSku)
		skuList = append(skuList, sku)
		sb.logger.Infof("为变体[%d]创建SKU，ASIN: %s，规格: %+v", i, variant.Asin, aiSku.Spec)
	}

	sb.logger.Infof("SKU去重完成: 原始=%d, 去重后=%d", len(variants), len(skuList))

	// 创建单个SKC
	skc := models.Skc{
		SkuList: skuList,
	}

	return []models.Skc{skc}
}
