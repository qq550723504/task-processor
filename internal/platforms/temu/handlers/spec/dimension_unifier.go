// Package handlers 提供规格维度统一服务
package spec

import (
	"fmt"
	"task-processor/internal/platforms/temu/handlers/property"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// SpecDimensionUnifier 规格维度统一器
type SpecDimensionUnifier struct {
	logger                   *logrus.Entry
	selector                 *SpecDimensionSelector
	mixedAttributesProcessor *property.MixedAttributesProcessor
}

// NewSpecDimensionUnifier 创建规格维度统一器
func NewSpecDimensionUnifier() *SpecDimensionUnifier {
	return &SpecDimensionUnifier{
		logger:                   logrus.WithField("service", "SpecDimensionUnifier"),
		selector:                 NewSpecDimensionSelector(),
		mixedAttributesProcessor: property.NewMixedAttributesProcessor(),
	}
}

// UnifySpecDimensions 统一AI映射中的规格维度
func (u *SpecDimensionUnifier) UnifySpecDimensions(aiMapping *types.AISkuMappingResponse) error {
	if len(aiMapping.SkuList) == 0 {
		return nil
	}

	u.logger.Info("🔧 开始统一规格维度")

	// 1. 分析所有SKU的规格维度使用情况
	dimensionStats := u.analyzeSpecDimensions(aiMapping)

	// 2. 选择最优的规格维度组合
	targetDimensions := u.selector.SelectOptimalDimensions(dimensionStats)

	// 3. 统一所有SKU的规格维度
	return u.applyUnifiedDimensions(aiMapping, targetDimensions)
}

// analyzeSpecDimensions 分析规格维度使用情况
func (u *SpecDimensionUnifier) analyzeSpecDimensions(aiMapping *types.AISkuMappingResponse) map[string]int {
	dimensionCount := make(map[string]int)

	for _, sku := range aiMapping.SkuList {
		for _, spec := range sku.Spec {
			dimensionCount[spec.ParentSpecID]++
		}
	}

	u.logger.Infof("📊 规格维度使用统计: %v", dimensionCount)
	return dimensionCount
}

// applyUnifiedDimensions 应用统一的规格维度
func (u *SpecDimensionUnifier) applyUnifiedDimensions(aiMapping *types.AISkuMappingResponse, targetDimensions []string) error {
	if len(targetDimensions) == 0 {
		u.logger.Warn("⚠️ 目标规格维度为空，跳过统一处理")
		return nil
	}

	u.logger.Infof("🔧 应用统一规格维度: %v", targetDimensions)

	// 检测是否为混合属性情况
	isMixedAttributes := u.mixedAttributesProcessor.DetectMixedAttributes(aiMapping)

	// 统计需要添加默认规格的SKU数量
	needsDefaultCount := 0
	for i := range aiMapping.SkuList {
		sku := &aiMapping.SkuList[i]
		unifiedSpecs := u.extractTargetSpecs(sku.Spec, targetDimensions)
		if len(unifiedSpecs) < len(targetDimensions) {
			needsDefaultCount++
		}
	}

	// 对于混合属性情况，强制进行统一处理
	if isMixedAttributes {
		u.logger.Infof("🎯 检测到混合属性情况，强制进行规格维度统一")
		return u.mixedAttributesProcessor.ForceUnification(aiMapping, targetDimensions)
	}

	// 如果超过一半的SKU需要添加默认规格，说明统一策略可能有问题
	if needsDefaultCount > len(aiMapping.SkuList)/2 {
		u.logger.Warnf("⚠️ 超过一半的SKU(%d/%d)需要添加默认规格，可能存在规格维度不匹配问题",
			needsDefaultCount, len(aiMapping.SkuList))
		u.logger.Warn("⚠️ 建议检查AI生成的规格是否合理，或调整统一策略")

		// 在这种情况下，不强制统一，保持原有规格
		u.logger.Info("🔄 保持原有规格维度，不进行强制统一")
		return nil
	}

	for i := range aiMapping.SkuList {
		sku := &aiMapping.SkuList[i]

		// 提取当前SKU在目标维度上的规格
		unifiedSpecs := u.extractTargetSpecs(sku.Spec, targetDimensions)

		// 只为缺少少量维度的SKU添加默认规格
		if len(unifiedSpecs) < len(targetDimensions) && len(unifiedSpecs) > 0 {
			// 添加缺失的维度
			for _, targetDim := range targetDimensions {
				found := false
				for _, spec := range unifiedSpecs {
					if spec.ParentSpecID == targetDim {
						found = true
						break
					}
				}
				if !found {
					defaultSpec := u.createDefaultSpec(targetDim)
					unifiedSpecs = append(unifiedSpecs, defaultSpec)
					u.logger.Warnf("⚠️ SKU[%d] 缺少维度 %s，使用默认规格: %+v", i, targetDim, defaultSpec)
				}
			}
		} else if len(unifiedSpecs) == 0 {
			u.logger.Warnf("⚠️ SKU[%d] 在目标维度上没有规格，保持原有规格", i)
			continue // 保持原有规格，不添加默认规格
		}

		// 更新SKU的规格
		if len(unifiedSpecs) > 0 {
			sku.Spec = unifiedSpecs
			// 重新生成unique_id
			u.regenerateUniqueID(sku)
		}
	}

	u.logger.Info("✅ 规格维度统一完成")
	return nil
}

// extractTargetSpecs 提取目标维度的规格
func (u *SpecDimensionUnifier) extractTargetSpecs(specs []types.SpecInfo, targetDimensions []string) []types.SpecInfo {
	var result []types.SpecInfo

	for _, targetDim := range targetDimensions {
		for _, spec := range specs {
			if spec.ParentSpecID == targetDim {
				result = append(result, spec)
				break
			}
		}
	}

	return result
}

// createDefaultSpec 创建默认规格
func (u *SpecDimensionUnifier) createDefaultSpec(parentSpecID string) types.SpecInfo {
	switch parentSpecID {
	case "1001": // Color
		return types.SpecInfo{
			SpecID:         "DEFAULT_COLOR",
			SpecName:       "Default Color",
			ParentSpecID:   "1001",
			ParentSpecName: "Color",
		}
	case "3001": // Size
		return types.SpecInfo{
			SpecID:         "DEFAULT_SIZE",
			SpecName:       "Default Size",
			ParentSpecID:   "3001",
			ParentSpecName: "Size",
		}
	case "18014": // Capacity
		return types.SpecInfo{
			SpecID:         "DEFAULT_CAPACITY",
			SpecName:       "Default Capacity",
			ParentSpecID:   "18014",
			ParentSpecName: "Capacity",
		}
	default:
		return types.SpecInfo{
			SpecID:         fmt.Sprintf("DEFAULT_%s", parentSpecID),
			SpecName:       "Default",
			ParentSpecID:   parentSpecID,
			ParentSpecName: "Unknown",
		}
	}
}

// regenerateUniqueID 重新生成unique_id
func (u *SpecDimensionUnifier) regenerateUniqueID(sku *types.AIGeneratedSku) {
	if len(sku.Spec) >= 2 {
		sku.UniqueID = fmt.Sprintf("%s_%s", sku.Spec[0].SpecID, sku.Spec[1].SpecID)
	} else if len(sku.Spec) == 1 {
		sku.UniqueID = sku.Spec[0].SpecID
	} else {
		sku.UniqueID = sku.Asin
	}

	// 更新spec_id
	if len(sku.Spec) > 0 {
		sku.SpecID = sku.Spec[len(sku.Spec)-1].SpecID
	}
}
