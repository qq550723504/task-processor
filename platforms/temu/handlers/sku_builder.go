package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"

	"task-processor/common/amazon"
	"task-processor/common/management/api"
	"task-processor/common/pipeline"
	"task-processor/openai"
	"task-processor/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// SkuBuilder SKU构建器
type SkuBuilder struct {
	logger           *logrus.Entry
	profitRuleClient api.ProfitRuleAPI
	priceHandler     *PriceHandler
	regionHandler    *RegionHandler
	imageProcessor   *ImageProcessor
	aiClient         *openai.Client
	specHandler      *SkuSpecHandler
	itemBuilder      *SkuItemBuilder
	skcBuilder       *SkuSkcBuilder
}

// NewSkuBuilder 创建新的SKU构建器
func NewSkuBuilder(logger *logrus.Entry, aiClient *openai.Client, profitRuleClient api.ProfitRuleAPI) *SkuBuilder {
	priceHandler := NewPriceHandler(profitRuleClient)
	regionHandler := NewRegionHandler()
	imageProcessor := NewImageProcessor()

	specHandler := NewSkuSpecHandler(logger)
	itemBuilder := NewSkuItemBuilder(logger, priceHandler, regionHandler, imageProcessor)
	skcBuilder := NewSkuSkcBuilder(logger, itemBuilder)

	return &SkuBuilder{
		logger:           logger,
		profitRuleClient: profitRuleClient,
		priceHandler:     priceHandler,
		regionHandler:    regionHandler,
		imageProcessor:   imageProcessor,
		aiClient:         aiClient,
		specHandler:      specHandler,
		itemBuilder:      itemBuilder,
		skcBuilder:       skcBuilder,
	}
}

// BuildVariantSkcs 构建变体SKC
func (sb *SkuBuilder) BuildVariantSkcs(ctx *pipeline.TaskContext, variants []*amazon.Product) error {
	sb.logger.Infof("构建变体SKC，变体数量: %d", len(variants))

	// 使用AI分析变体并生成SKU映射
	aiMapping, err := sb.generateAISkuMapping(ctx, variants)
	if err != nil {
		sb.logger.Warnf("AI生成SKU映射失败: %v，使用默认映射", err)
		return sb.buildVariantSkcsDefault(ctx, variants)
	}

	// 将AI映射存储到context中，供其他处理器使用（如图片尺寸标注）
	ctx.SetData("ai_sku_mapping", aiMapping)
	sb.logger.Info("✅ AI SKU映射已存储到context")

	// 根据AI映射构建SKC列表
	skcList, err := sb.buildSkcsFromAIMapping(ctx, variants, aiMapping)
	if err != nil {
		sb.logger.Warnf("根据AI映射构建SKC失败: %v，使用默认映射", err)
		return sb.buildVariantSkcsDefault(ctx, variants)
	}

	ctx.TemuProduct.SkcList = skcList
	sb.logger.Infof("AI辅助构建完成，创建了%d个SKC", len(skcList))
	return nil
}

// buildSkcsFromAIMapping 根据AI映射构建SKC
func (sb *SkuBuilder) buildSkcsFromAIMapping(ctx *pipeline.TaskContext, variants []*amazon.Product, aiMapping *AISkuMappingResponse) ([]types.Skc, error) {
	// 检查AI映射数量
	if len(aiMapping.SkuList) != len(variants) {
		sb.logger.Warnf("⚠️ AI映射数量(%d)与变体数量(%d)不匹配", len(aiMapping.SkuList), len(variants))

		// 如果AI映射数量少于变体数量，尝试补充缺失的映射
		if len(aiMapping.SkuList) < len(variants) {
			sb.logger.Infof("尝试为缺失的%d个变体补充默认映射", len(variants)-len(aiMapping.SkuList))
			if err := sb.supplementMissingMappings(aiMapping, variants); err != nil {
				return nil, fmt.Errorf("补充缺失映射失败: %w", err)
			}
			sb.logger.Infof("✅ 成功补充缺失映射，当前映射数量: %d", len(aiMapping.SkuList))
		} else {
			// AI映射数量多于变体数量，尝试去重或移除多余的映射
			diff := len(aiMapping.SkuList) - len(variants)
			sb.logger.Warnf("⚠️ AI映射数量多于变体数量，差异: %d个", diff)

			// 如果差异在可接受范围内（≤2个），尝试智能处理
			if diff <= 2 {
				sb.logger.Infof("差异在可接受范围内，尝试去重和修复...")
				if err := sb.removeDuplicateOrExcessMappings(aiMapping, variants); err != nil {
					return nil, fmt.Errorf("移除多余映射失败: %w", err)
				}
				sb.logger.Infof("✅ 成功处理多余映射，当前映射数量: %d", len(aiMapping.SkuList))
			} else {
				// 差异过大，无法处理
				return nil, fmt.Errorf("AI映射数量(%d)远多于变体数量(%d)，差异过大(%d)，无法处理",
					len(aiMapping.SkuList), len(variants), diff)
			}
		}
	}

	// 预防性检查：验证AI映射中的规格数量和有效性
	for i, aiSku := range aiMapping.SkuList {
		// 检查规格数量是否超过2个（TEMU限制）
		if len(aiSku.Spec) > 2 {
			sb.logger.Errorf("❌ AI映射[%d]规格数量超限: 当前有%d个规格，TEMU最多允许2个销售规格", i, len(aiSku.Spec))
			sb.logger.Errorf("❌ 规格详情: %+v", aiSku.Spec)
			return nil, fmt.Errorf("AI映射[%d]规格数量超限: 有%d个规格，但TEMU最多允许2个", i, len(aiSku.Spec))
		}

		// 验证规格是否有效
		if err := sb.specHandler.ValidateSpecs(aiSku.Spec); err != nil {
			sb.logger.Errorf("❌ AI映射[%d]规格验证失败: %v", i, err)
			sb.logger.Error("❌ AI必须从TEMU模板中选择有效的规格，不能使用默认规格")
			// 不修复，让AI重新生成或者报错
			return nil, fmt.Errorf("AI映射[%d]规格无效: %w", i, err)
		}
		// 输出AI提取的物流信息
		sb.logger.Infof("📦 SKU[%d] AI提取的物流信息: weight=%s, length=%s, width=%s, height=%s",
			i, aiSku.Weight, aiSku.Length, aiSku.Width, aiSku.Height)
	}

	// 解析临时规格ID为真实规格ID（必须成功）
	if err := sb.resolveTemporarySpecIDs(ctx, aiMapping); err != nil {
		sb.logger.Errorf("❌ 解析规格ID失败: %v", err)
		return nil, fmt.Errorf("解析临时规格ID失败: %w", err)
	}
	sb.logger.Info("✅ 成功解析所有临时规格ID")

	// 检查规格来源：只有GoodsSpecProperties不为空且有预置规格值时，才创建多SKC
	// 其他情况都创建单SKC，多SKU
	templateInfo, hasTemplateInfo := GetTemplateInfoFromContext(ctx)
	userInputSpecs, hasUserInputSpecs := GetUserInputParentSpecListFromContext(ctx)

	var skcList []types.Skc

	// 检查是否应该创建多SKC：GoodsSpecProperties不为空且有预置规格值
	shouldCreateMultipleSkcs := false
	if hasTemplateInfo && len(templateInfo.GoodsSpecProperties) > 0 {
		// 检查是否有预置的规格值
		for _, prop := range templateInfo.GoodsSpecProperties {
			if len(prop.Values) > 0 {
				shouldCreateMultipleSkcs = true
				break
			}
		}
	}

	if shouldCreateMultipleSkcs {
		// 创建多个SKC（按主变体分组）
		sb.logger.Infof("GoodsSpecProperties有预置规格值，构建多个SKC，规格属性数量: %d", len(templateInfo.GoodsSpecProperties))
		skcList = sb.skcBuilder.buildMultipleSkcsFromTemplate(ctx, variants, aiMapping, templateInfo.GoodsSpecProperties)
	} else {
		// 创建单个SKC，多个SKU
		sb.logger.Info("创建单个SKC，多个SKU")

		// 优先使用UserInputParentSpecList，否则使用空的模板规格
		var templateSpecs []GoodsSpecProperty
		if hasUserInputSpecs && len(userInputSpecs) > 0 {
			sb.logger.Infof("使用UserInputParentSpecList，用户规格数量: %d", len(userInputSpecs))
			templateSpecs = sb.specHandler.convertUserInputSpecsToGoodsSpecProperties(userInputSpecs)
		} else if hasTemplateInfo {
			sb.logger.Infof("使用GoodsSpecProperties（无预置值），规格属性数量: %d", len(templateInfo.GoodsSpecProperties))
			templateSpecs = templateInfo.GoodsSpecProperties
		} else {
			sb.logger.Warn("未找到任何规格信息")
			templateSpecs = []GoodsSpecProperty{}
		}

		skcList = sb.skcBuilder.buildSingleSkcFromUserInput(ctx, variants, aiMapping, templateSpecs)
	}

	return skcList, nil
}

// removeDuplicateOrExcessMappings 移除重复或多余的AI映射
func (sb *SkuBuilder) removeDuplicateOrExcessMappings(aiMapping *AISkuMappingResponse, variants []*amazon.Product) error {
	// 创建变体ASIN集合
	validAsins := make(map[string]bool)
	for _, variant := range variants {
		validAsins[variant.Asin] = true
	}

	// 统计每个ASIN出现的次数
	asinCount := make(map[string]int)
	for _, sku := range aiMapping.SkuList {
		asinCount[sku.Asin]++
	}

	// 找出重复的ASIN
	duplicateAsins := make(map[string]bool)
	for asin, count := range asinCount {
		if count > 1 {
			duplicateAsins[asin] = true
			sb.logger.Warnf("⚠️ 检测到重复的ASIN: %s (出现%d次)", asin, count)
		}
	}

	// 找出不在变体列表中的ASIN
	invalidAsins := make(map[string]bool)
	for _, sku := range aiMapping.SkuList {
		if !validAsins[sku.Asin] {
			invalidAsins[sku.Asin] = true
			sb.logger.Warnf("⚠️ 检测到无效的ASIN: %s (不在变体列表中)", sku.Asin)
		}
	}

	// 过滤SKU列表：移除重复和无效的映射
	var filteredSkus []AIGeneratedSku
	seenAsins := make(map[string]bool)

	for _, sku := range aiMapping.SkuList {
		// 跳过无效的ASIN
		if invalidAsins[sku.Asin] {
			sb.logger.Infof("🗑️ 移除无效映射: ASIN=%s", sku.Asin)
			continue
		}

		// 如果是重复的ASIN，只保留第一个
		if duplicateAsins[sku.Asin] {
			if seenAsins[sku.Asin] {
				sb.logger.Infof("🗑️ 移除重复映射: ASIN=%s", sku.Asin)
				continue
			}
		}

		filteredSkus = append(filteredSkus, sku)
		seenAsins[sku.Asin] = true
	}

	// 如果过滤后数量仍然不匹配，移除多余的映射（保留前N个）
	if len(filteredSkus) > len(variants) {
		excess := len(filteredSkus) - len(variants)
		sb.logger.Warnf("⚠️ 过滤后仍有%d个多余映射，将移除末尾的映射", excess)
		filteredSkus = filteredSkus[:len(variants)]
	}

	// 更新映射列表
	removedCount := len(aiMapping.SkuList) - len(filteredSkus)
	aiMapping.SkuList = filteredSkus
	sb.logger.Infof("✅ 移除了%d个多余/重复的映射，剩余%d个映射", removedCount, len(filteredSkus))

	// 验证最终数量
	if len(aiMapping.SkuList) != len(variants) {
		return fmt.Errorf("处理后映射数量(%d)仍与变体数量(%d)不匹配", len(aiMapping.SkuList), len(variants))
	}

	return nil
}

// supplementMissingMappings 为缺失的变体补充默认映射
func (sb *SkuBuilder) supplementMissingMappings(aiMapping *AISkuMappingResponse, variants []*amazon.Product) error {
	// 创建已映射的ASIN集合
	mappedAsins := make(map[string]bool)
	for _, sku := range aiMapping.SkuList {
		mappedAsins[sku.Asin] = true
	}

	// 分析已有映射的spec模式，用于推断缺失映射的spec
	specTemplate := sb.analyzeSpecPattern(aiMapping)

	// 为未映射的变体创建默认映射
	missingCount := 0
	for _, variant := range variants {
		if !mappedAsins[variant.Asin] {
			missingCount++
			sb.logger.Infof("为变体 %s 创建补充映射 (第%d个缺失)", variant.Asin, missingCount)

			// 创建默认SKU映射，尝试使用spec模板
			defaultSku := AIGeneratedSku{
				UniqueID:          variant.Asin,
				Asin:              variant.Asin,
				Spec:              specTemplate, // 使用从已有映射推断的spec模板
				Weight:            "",
				Length:            "",
				Width:             "",
				Height:            "",
				VariantAttributes: make(map[string]string),
			}

			aiMapping.SkuList = append(aiMapping.SkuList, defaultSku)
			sb.logger.Infof("✅ 已为变体 %s 添加补充映射 (使用spec模板: %d个规格)", variant.Asin, len(specTemplate))
		}
	}

	if missingCount > 0 {
		sb.logger.Warnf("⚠️ 补充了%d个缺失的映射，这些映射使用了推断的spec模板", missingCount)
		sb.logger.Warn("⚠️ 建议检查AI映射生成逻辑，确保为所有变体生成正确的映射")
	}

	return nil
}

// analyzeSpecPattern 分析已有映射的spec模式，返回一个spec模板
func (sb *SkuBuilder) analyzeSpecPattern(aiMapping *AISkuMappingResponse) []types.SpecInfo {
	if len(aiMapping.SkuList) == 0 {
		return []types.SpecInfo{}
	}

	// 统计每个spec_id出现的频率
	specFrequency := make(map[string]int)
	specExamples := make(map[string]types.SpecInfo)

	for _, sku := range aiMapping.SkuList {
		for _, spec := range sku.Spec {
			specFrequency[spec.SpecID]++
			if _, exists := specExamples[spec.SpecID]; !exists {
				// 保存第一个遇到的spec作为示例（但清空具体的值）
				specExamples[spec.SpecID] = types.SpecInfo{
					SpecID:         spec.SpecID,
					SpecName:       spec.SpecName,
					ParentSpecID:   spec.ParentSpecID,
					ParentSpecName: spec.ParentSpecName,
					ParentID:       "", // 清空具体的值，让后续逻辑处理
				}
			}
		}
	}

	// 选择出现频率最高的spec作为模板
	var template []types.SpecInfo
	for specID, spec := range specExamples {
		if specFrequency[specID] > len(aiMapping.SkuList)/2 {
			// 如果这个spec在超过一半的SKU中出现，认为它是必需的
			template = append(template, spec)
		}
	}

	if len(template) > 0 {
		sb.logger.Infof("从已有映射中推断出spec模板: %d个规格", len(template))
	} else {
		sb.logger.Warn("无法从已有映射中推断spec模板，将使用空spec")
	}

	return template
}

// CreateDefaultSkc 创建默认SKC（用于没有变体的产品）
// 使用AI从Amazon产品信息中提取规格、重量、尺寸等信息
func (sb *SkuBuilder) CreateDefaultSkc(ctx *pipeline.TaskContext) (types.Skc, error) {
	sb.logger.Info("创建默认SKC（产品没有变体），使用AI提取信息")

	if ctx.AmazonProduct == nil {
		return types.Skc{}, fmt.Errorf("没有Amazon产品信息")
	}

	// 将单一产品包装成变体列表，让AI处理
	variants := []*amazon.Product{ctx.AmazonProduct}

	// 使用AI生成SKU映射
	aiMapping, err := sb.generateAISkuMapping(ctx, variants)
	if err != nil {
		sb.logger.Errorf("❌ AI生成SKU映射失败: %v", err)
		return types.Skc{}, fmt.Errorf("AI生成SKU映射失败: %w", err)
	}

	if len(aiMapping.SkuList) == 0 {
		return types.Skc{}, fmt.Errorf("AI未生成任何SKU")
	}

	// 使用第一个AI生成的SKU
	aiSku := aiMapping.SkuList[0]

	// 验证规格
	if err := sb.specHandler.ValidateSpecs(aiSku.Spec); err != nil {
		sb.logger.Errorf("❌ AI生成的规格验证失败: %v", err)
		return types.Skc{}, fmt.Errorf("AI生成的规格无效: %w", err)
	}

	sb.logger.Infof("✅ AI成功生成规格: %+v", aiSku.Spec)
	sb.logger.Infof("✅ AI提取的重量尺寸: weight=%s, length=%s, width=%s, height=%s",
		aiSku.Weight, aiSku.Length, aiSku.Width, aiSku.Height)

	// 解析临时规格ID为真实规格ID（必须成功）
	if err := sb.resolveTemporarySpecIDs(ctx, aiMapping); err != nil {
		sb.logger.Errorf("❌ 解析规格ID失败: %v", err)
		return types.Skc{}, fmt.Errorf("解析临时规格ID失败: %w", err)
	}
	sb.logger.Info("✅ 成功解析所有临时规格ID")

	// 使用AI生成的SKU构建完整的SKU
	sku := sb.itemBuilder.buildSkuFromVariantWithAI(ctx, ctx.AmazonProduct, aiSku)

	return types.Skc{
		SkuList: []types.Sku{sku},
	}, nil
}

// ProcessSkcItem 处理SKC项目
func (sb *SkuBuilder) ProcessSkcItem(ctx *pipeline.TaskContext, skcIndex int) error {
	skc := &ctx.TemuProduct.SkcList[skcIndex]

	// 处理SKC下的每个SKU
	for i := range skc.SkuList {
		if err := sb.itemBuilder.processSkuItem(ctx, skcIndex, i); err != nil {
			return fmt.Errorf("处理SKU[%d]失败: %w", i, err)
		}
	}

	sb.logger.Infof("SKC[%d]处理完成，包含%d个SKU", skcIndex, len(skc.SkuList))
	return nil
}

// GetTotalSkuCount 获取总SKU数量
func (sb *SkuBuilder) GetTotalSkuCount(skcList []types.Skc) int {
	total := 0
	for _, skc := range skcList {
		total += len(skc.SkuList)
	}
	return total
}

// marshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
func (sb *SkuBuilder) marshalWithoutHTMLEscape(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false) // 关闭HTML转义，避免&被转义为\u0026

	if err := encoder.Encode(v); err != nil {
		return nil, err
	}

	// 移除最后的换行符
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return result, nil
}
