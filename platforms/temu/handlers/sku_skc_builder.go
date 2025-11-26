package handlers

import (
	"fmt"

	"task-processor/common/amazon"
	"task-processor/common/pipeline"
	"task-processor/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// SkuSkcBuilder SKC构建器
type SkuSkcBuilder struct {
	logger      *logrus.Entry
	specHandler *SkuSpecHandler
	itemBuilder *SkuItemBuilder
}

// NewSkuSkcBuilder 创建新的SKC构建器
func NewSkuSkcBuilder(logger *logrus.Entry, itemBuilder *SkuItemBuilder) *SkuSkcBuilder {
	return &SkuSkcBuilder{
		logger:      logger,
		specHandler: NewSkuSpecHandler(logger),
		itemBuilder: itemBuilder,
	}
}

// buildMultipleSkcsFromTemplate 使用模板属性构建多个SKC（按主变体分组）
func (sb *SkuSkcBuilder) buildMultipleSkcsFromTemplate(ctx *pipeline.TaskContext, variants []*amazon.Product, aiMapping *AISkuMappingResponse, templateSpecs []GoodsSpecProperty) []types.Skc {
	// 按颜色分组SKU
	colorGroups := make(map[string][]int)
	for i, aiSku := range aiMapping.SkuList {
		colorSpecID := aiSku.ColorSpecID
		if colorSpecID == "" {
			colorSpecID = "default"
		}
		colorGroups[colorSpecID] = append(colorGroups[colorSpecID], i)
	}

	skcList := make([]types.Skc, 0, len(colorGroups))

	// 为每个颜色组创建一个SKC
	for colorSpecID, skuIndices := range colorGroups {
		// 生成该颜色组的完整SKU列表（包括缺失的组合）
		skuList := sb.buildCompleteSkuListForColor(ctx, variants, aiMapping, skuIndices, colorSpecID, templateSpecs)

		// 创建SKC
		skc := types.Skc{
			SkuList: skuList,
		}

		skcList = append(skcList, skc)
	}

	return skcList
}

// buildCompleteSkuListForColor 为特定颜色构建完整的SKU列表
func (sb *SkuSkcBuilder) buildCompleteSkuListForColor(ctx *pipeline.TaskContext, variants []*amazon.Product, aiMapping *AISkuMappingResponse, skuIndices []int, colorSpecID string, templateSpecs []GoodsSpecProperty) []types.Sku {
	// 收集该颜色下所有可能的非颜色规格组合
	nonColorSpecs := sb.specHandler.collectNonColorSpecsForColor(aiMapping, skuIndices, templateSpecs)

	sb.logger.Infof("收集到 %d 个非颜色规格维度", len(nonColorSpecs))

	// 如果没有非颜色规格，直接使用现有的SKU（只有颜色规格）
	if len(nonColorSpecs) == 0 {
		sb.logger.Info("没有非颜色规格，直接使用现有SKU")
		skuList := make([]types.Sku, 0, len(skuIndices))
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
		key := sb.specHandler.createNonColorSpecKey(aiSku.Spec)
		existingSkuMap[key] = skuIndex
		sb.logger.Debugf("存储现有SKU映射: key=%s, index=%d", key, skuIndex)
	}

	sb.logger.Infof("现有SKU映射数量: %d", len(existingSkuMap))

	var skuList []types.Sku

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
			fullSpecs := sb.specHandler.addColorSpecToCombination(nonColorCombination, colorSpecID, aiMapping, skuIndices)
			sku := sb.itemBuilder.buildDeletedSku(ctx, fullSpecs)
			skuList = append(skuList, sku)
			sb.logger.Warnf("创建缺失变体: 颜色=%s, 组合=%s", colorSpecID, combinationKey)
		}
	}

	return skuList
}

// buildSingleSkcFromUserInput 使用规格构建单个SKC（包含多个SKU）
func (sb *SkuSkcBuilder) buildSingleSkcFromUserInput(ctx *pipeline.TaskContext, variants []*amazon.Product, aiMapping *AISkuMappingResponse, templateSpecs []GoodsSpecProperty) []types.Skc {
	// 1. 检查规格维度一致性
	if err := sb.validateSpecDimensionConsistency(aiMapping); err != nil {
		sb.logger.Errorf("❌ 规格维度不一致: %v", err)
		sb.logger.Error("❌ 所有SKU必须具有相同的规格维度（parent_spec_id集合）")

		// 尝试修复：统一规格维度
		sb.normalizeSpecDimensions(aiMapping, templateSpecs)
	}

	var skuList []types.Sku
	specCombinationMap := make(map[string]bool) // 用于去重

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

		// 检查规格组合是否重复
		specKey := sb.specHandler.createSpecCombinationKey(aiSku.Spec)
		if specCombinationMap[specKey] {
			sb.logger.Warnf("⚠️ 跳过重复的SKU[%d]，规格组合: %s", i, specKey)
			continue
		}
		specCombinationMap[specKey] = true

		sku := sb.itemBuilder.buildSkuFromVariantWithAI(ctx, variant, aiSku)
		skuList = append(skuList, sku)
		sb.logger.Infof("为变体[%d]创建SKU，规格: %+v", i, aiSku.Spec)
	}

	sb.logger.Infof("SKU去重完成: 原始=%d, 去重后=%d", len(variants), len(skuList))

	// 创建单个SKC
	skc := types.Skc{
		SkuList: skuList,
	}

	return []types.Skc{skc}
}

// validateSpecDimensionConsistency 验证规格维度一致性
func (sb *SkuSkcBuilder) validateSpecDimensionConsistency(aiMapping *AISkuMappingResponse) error {
	if len(aiMapping.SkuList) == 0 {
		return nil
	}

	// 收集第一个SKU的规格维度作为基准
	baseParentSpecIDs := make(map[string]bool)
	for _, spec := range aiMapping.SkuList[0].Spec {
		baseParentSpecIDs[spec.ParentSpecID] = true
	}

	// 检查其他SKU是否具有相同的规格维度
	for i := 1; i < len(aiMapping.SkuList); i++ {
		currentParentSpecIDs := make(map[string]bool)
		for _, spec := range aiMapping.SkuList[i].Spec {
			currentParentSpecIDs[spec.ParentSpecID] = true
		}

		// 比较规格维度
		if !sb.areSpecDimensionsEqual(baseParentSpecIDs, currentParentSpecIDs) {
			return fmt.Errorf("SKU[%d]的规格维度与SKU[0]不一致: base=%v, current=%v",
				i, sb.getParentSpecIDList(baseParentSpecIDs), sb.getParentSpecIDList(currentParentSpecIDs))
		}
	}

	return nil
}

// areSpecDimensionsEqual 比较两个规格维度集合是否相等
func (sb *SkuSkcBuilder) areSpecDimensionsEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for key := range a {
		if !b[key] {
			return false
		}
	}
	return true
}

// getParentSpecIDList 获取parent_spec_id列表
func (sb *SkuSkcBuilder) getParentSpecIDList(parentSpecIDs map[string]bool) []string {
	var list []string
	for id := range parentSpecIDs {
		list = append(list, id)
	}
	return list
}

// normalizeSpecDimensions 统一规格维度
func (sb *SkuSkcBuilder) normalizeSpecDimensions(aiMapping *AISkuMappingResponse, templateSpecs []GoodsSpecProperty) {
	sb.logger.Warn("⚠️ 尝试统一规格维度...")

	// 收集所有出现过的parent_spec_id
	allParentSpecIDs := make(map[string]*types.SpecInfo)
	for _, aiSku := range aiMapping.SkuList {
		for _, spec := range aiSku.Spec {
			if _, exists := allParentSpecIDs[spec.ParentSpecID]; !exists {
				allParentSpecIDs[spec.ParentSpecID] = &types.SpecInfo{
					ParentSpecID:   spec.ParentSpecID,
					ParentSpecName: spec.ParentSpecName,
				}
			}
		}
	}

	// 如果没有任何规格维度，说明是单一产品，不需要统一
	if len(allParentSpecIDs) == 0 {
		sb.logger.Info("✅ 没有规格维度，跳过统一（单一产品）")
		return
	}

	// 转换为bool map用于日志输出
	parentSpecIDsForLog := make(map[string]bool)
	for id := range allParentSpecIDs {
		parentSpecIDsForLog[id] = true
	}
	sb.logger.Infof("发现%d个不同的规格维度: %v", len(allParentSpecIDs), sb.getParentSpecIDList(parentSpecIDsForLog))

	// TEMU规则：最多只能有2个销售规格
	// 这个检查应该不会触发，因为AI映射阶段已经强制限制为2个规格
	// 但保留作为防御性检查
	if len(allParentSpecIDs) > 2 {
		sb.logger.Errorf("❌ 无法统一规格维度：产品包含%d个不同的规格维度，但TEMU最多只允许2个销售规格", len(allParentSpecIDs))
		sb.logger.Error("❌ 这不应该发生，因为AI映射阶段应该已经限制为2个规格")
		sb.logger.Error("❌ 建议：检查AI映射逻辑或联系技术支持")
		return
	}

	// 为每个SKU补全缺失的规格维度
	for i := range aiMapping.SkuList {
		aiSku := &aiMapping.SkuList[i]

		// 检查当前SKU缺少哪些规格维度
		currentParentSpecIDs := make(map[string]bool)
		for _, spec := range aiSku.Spec {
			currentParentSpecIDs[spec.ParentSpecID] = true
		}

		// 补全缺失的规格维度
		for parentSpecID, parentSpecInfo := range allParentSpecIDs {
			if !currentParentSpecIDs[parentSpecID] {
				// 1. 尝试从模板中找到默认值
				defaultSpec := sb.getDefaultSpecForParent(parentSpecID, templateSpecs)
				if defaultSpec != nil {
					aiSku.Spec = append(aiSku.Spec, *defaultSpec)
					sb.logger.Warnf("⚠️ 为SKU[%d]补全规格维度（从模板）: %s=%s", i, parentSpecInfo.ParentSpecName, defaultSpec.SpecName)
					continue
				}

				// 2. 从其他SKU中找到第一个该维度的规格值作为默认值
				defaultSpec = sb.getFirstSpecValueFromOtherSkus(parentSpecID, aiMapping)
				if defaultSpec != nil {
					aiSku.Spec = append(aiSku.Spec, *defaultSpec)
					sb.logger.Warnf("⚠️ 为SKU[%d]补全规格维度（从其他SKU）: %s=%s", i, parentSpecInfo.ParentSpecName, defaultSpec.SpecName)
					continue
				}

				// 3. 如果还是找不到，记录错误
				sb.logger.Errorf("❌ SKU[%d]缺少规格维度 %s (ParentSpecID=%s)，无法找到默认值",
					i, parentSpecInfo.ParentSpecName, parentSpecID)
			}
		}
	}

	sb.logger.Info("✅ 规格维度统一完成")
}

// getDefaultSpecForParent 获取指定parent_spec_id的默认规格值
func (sb *SkuSkcBuilder) getDefaultSpecForParent(parentSpecID string, templateSpecs []GoodsSpecProperty) *types.SpecInfo {
	for _, templateSpec := range templateSpecs {
		if templateSpec.ParentSpecID == parentSpecID && len(templateSpec.Values) > 0 {
			// 返回第一个可用值作为默认值
			firstValue := templateSpec.Values[0]
			return &types.SpecInfo{
				SpecID:         firstValue.SpecID,
				SpecName:       firstValue.Value,
				ParentSpecID:   parentSpecID,
				ParentSpecName: templateSpec.Name,
			}
		}
	}
	return nil
}

// getFirstSpecValueFromOtherSkus 从其他SKU中获取指定parent_spec_id的第一个规格值
func (sb *SkuSkcBuilder) getFirstSpecValueFromOtherSkus(parentSpecID string, aiMapping *AISkuMappingResponse) *types.SpecInfo {
	for _, aiSku := range aiMapping.SkuList {
		for _, spec := range aiSku.Spec {
			if spec.ParentSpecID == parentSpecID {
				return &types.SpecInfo{
					SpecID:         spec.SpecID,
					SpecName:       spec.SpecName,
					ParentSpecID:   spec.ParentSpecID,
					ParentSpecName: spec.ParentSpecName,
				}
			}
		}
	}
	return nil
}
