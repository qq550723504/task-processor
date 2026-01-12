// Package handlers 提供混合属性处理功能
package handlers

import (
	"fmt"
	"strings"
	"task-processor/internal/platforms/temu/api/models"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// MixedAttributesProcessor 混合属性处理器
type MixedAttributesProcessor struct {
	logger *logrus.Entry
}

// NewMixedAttributesProcessor 创建混合属性处理器
func NewMixedAttributesProcessor() *MixedAttributesProcessor {
	return &MixedAttributesProcessor{
		logger: logrus.WithField("service", "MixedAttributesProcessor"),
	}
}

// DetectMixedAttributes 检测是否存在混合属性情况
func (p *MixedAttributesProcessor) DetectMixedAttributes(aiMapping *types.AISkuMappingResponse) bool {
	if len(aiMapping.SkuList) <= 1 {
		return false
	}

	// 收集所有使用的规格维度
	usedDimensions := make(map[string]bool)
	for _, sku := range aiMapping.SkuList {
		for _, spec := range sku.Spec {
			usedDimensions[spec.ParentSpecID] = true
		}
	}

	// 如果使用了多个不同的规格维度，可能是混合属性
	if len(usedDimensions) > 1 {
		p.logger.Infof("🔍 检测到多个规格维度: %v，可能存在混合属性", usedDimensions)

		// 进一步检查：是否不同SKU使用了不同的维度组合
		dimensionCombinations := make(map[string]int)
		for _, sku := range aiMapping.SkuList {
			var combination []string
			for _, spec := range sku.Spec {
				combination = append(combination, spec.ParentSpecID)
			}
			// 排序以确保一致性
			if len(combination) > 1 && combination[0] > combination[1] {
				combination[0], combination[1] = combination[1], combination[0]
			}
			key := fmt.Sprintf("%v", combination)
			dimensionCombinations[key]++
		}

		if len(dimensionCombinations) > 1 {
			p.logger.Warnf("⚠️ 检测到混合属性：不同SKU使用了不同的规格维度组合: %v", dimensionCombinations)
			return true
		}
	}

	return false
}

// ForceUnification 强制统一混合属性
func (p *MixedAttributesProcessor) ForceUnification(aiMapping *types.AISkuMappingResponse, targetDimensions []string) error {
	p.logger.Info("🔧 开始强制统一混合属性")

	// 选择最通用的维度作为统一维度
	unifiedDimension := p.selectBestUniversalDimension(targetDimensions)
	if unifiedDimension == "" {
		// 如果没有找到合适的通用维度，使用第一个目标维度
		unifiedDimension = targetDimensions[0]
	}

	p.logger.Infof("🎯 选择统一维度: %s", unifiedDimension)

	// 为所有SKU统一使用该维度
	for i := range aiMapping.SkuList {
		sku := &aiMapping.SkuList[i]

		// 查找当前SKU是否已有该维度的规格
		var existingSpec *models.SpecInfo
		for j := range sku.Spec {
			if sku.Spec[j].ParentSpecID == unifiedDimension {
				existingSpec = &sku.Spec[j]
				break
			}
		}

		if existingSpec != nil {
			// 如果已有该维度的规格，只保留这一个
			sku.Spec = []models.SpecInfo{*existingSpec}
			p.logger.Infof("✅ SKU[%d] 保留现有规格: %s = %s", i, existingSpec.ParentSpecID, existingSpec.SpecName)
		} else {
			// 如果没有该维度的规格，需要转换现有规格
			convertedSpec := p.convertToUnifiedDimension(sku.Spec, unifiedDimension)
			if convertedSpec != nil {
				sku.Spec = []models.SpecInfo{*convertedSpec}
				p.logger.Infof("🔄 SKU[%d] 转换规格到统一维度: %s = %s", i, convertedSpec.ParentSpecID, convertedSpec.SpecName)
			} else {
				// 如果无法转换，创建默认规格
				defaultSpec := p.createDefaultSpec(unifiedDimension)
				sku.Spec = []models.SpecInfo{defaultSpec}
				p.logger.Warnf("⚠️ SKU[%d] 无法转换，使用默认规格: %s = %s", i, defaultSpec.ParentSpecID, defaultSpec.SpecName)
			}
		}

		// 重新生成unique_id
		p.regenerateUniqueID(sku)
	}

	p.logger.Info("✅ 混合属性强制统一完成")
	return nil
}

// selectBestUniversalDimension 选择最佳的通用维度
func (p *MixedAttributesProcessor) selectBestUniversalDimension(dimensions []string) string {
	// 通用维度优先级：Style > Items > Size > Color
	universalPriority := map[string]int{
		"18012": 100, // Style - 最通用，可以表示任何类型的变体
		"17020": 90,  // Items - 适合数量和套装
		"3001":  80,  // Size - 适合尺寸
		"1001":  70,  // Color - 适合颜色
	}

	bestDimension := ""
	bestScore := -1

	for _, dim := range dimensions {
		if score, exists := universalPriority[dim]; exists {
			if score > bestScore {
				bestScore = score
				bestDimension = dim
			}
		}
	}

	return bestDimension
}

// convertToUnifiedDimension 将现有规格转换到统一维度
func (p *MixedAttributesProcessor) convertToUnifiedDimension(specs []models.SpecInfo, targetDimension string) *models.SpecInfo {
	if len(specs) == 0 {
		return nil
	}

	// 使用第一个规格的值作为转换基础
	sourceSpec := specs[0]

	// 根据目标维度创建转换后的规格
	switch targetDimension {
	case "18012": // Style - 可以接受任何值
		return &models.SpecInfo{
			SpecID:         fmt.Sprintf("TEMP_%s", sourceSpec.SpecName),
			SpecName:       sourceSpec.SpecName, // 保持原始值
			ParentSpecID:   "18012",
			ParentSpecName: "Style",
		}
	case "17020": // Items - 适合数量相关
		// 如果原始值包含数量信息，保持；否则转换为"Single"
		specName := sourceSpec.SpecName
		if !p.containsQuantityInfo(specName) {
			specName = "Single"
		}
		return &models.SpecInfo{
			SpecID:         fmt.Sprintf("TEMP_%s", specName),
			SpecName:       specName,
			ParentSpecID:   "17020",
			ParentSpecName: "Items",
		}
	case "3001": // Size - 适合尺寸相关
		// 如果原始值包含尺寸信息，保持；否则转换为"Standard"
		specName := sourceSpec.SpecName
		if !p.containsSizeInfo(specName) {
			specName = "Standard"
		}
		return &models.SpecInfo{
			SpecID:         fmt.Sprintf("TEMP_%s", specName),
			SpecName:       specName,
			ParentSpecID:   "3001",
			ParentSpecName: "Size",
		}
	default:
		// 对于其他维度，直接转换
		return &models.SpecInfo{
			SpecID:         fmt.Sprintf("TEMP_%s", sourceSpec.SpecName),
			SpecName:       sourceSpec.SpecName,
			ParentSpecID:   targetDimension,
			ParentSpecName: "Unknown",
		}
	}
}

// containsQuantityInfo 检查值是否包含数量信息
func (p *MixedAttributesProcessor) containsQuantityInfo(value string) bool {
	quantityKeywords := []string{"set", "pack", "piece", "count", "qty", "quantity", "数量", "套", "件"}
	valueLower := strings.ToLower(value)
	for _, keyword := range quantityKeywords {
		if strings.Contains(valueLower, keyword) {
			return true
		}
	}
	return false
}

// containsSizeInfo 检查值是否包含尺寸信息
func (p *MixedAttributesProcessor) containsSizeInfo(value string) bool {
	sizeKeywords := []string{"\"", "inch", "cm", "mm", "size", "large", "small", "medium", "xl", "xs", "尺寸", "大", "小", "中"}
	valueLower := strings.ToLower(value)
	for _, keyword := range sizeKeywords {
		if strings.Contains(valueLower, keyword) {
			return true
		}
	}
	return false
}

// createDefaultSpec 创建默认规格
func (p *MixedAttributesProcessor) createDefaultSpec(parentSpecID string) models.SpecInfo {
	switch parentSpecID {
	case "1001": // Color
		return models.SpecInfo{
			SpecID:         "DEFAULT_COLOR",
			SpecName:       "Default Color",
			ParentSpecID:   "1001",
			ParentSpecName: "Color",
		}
	case "3001": // Size
		return models.SpecInfo{
			SpecID:         "DEFAULT_SIZE",
			SpecName:       "Default Size",
			ParentSpecID:   "3001",
			ParentSpecName: "Size",
		}
	case "18014": // Capacity
		return models.SpecInfo{
			SpecID:         "DEFAULT_CAPACITY",
			SpecName:       "Default Capacity",
			ParentSpecID:   "18014",
			ParentSpecName: "Capacity",
		}
	default:
		return models.SpecInfo{
			SpecID:         fmt.Sprintf("DEFAULT_%s", parentSpecID),
			SpecName:       "Default",
			ParentSpecID:   parentSpecID,
			ParentSpecName: "Unknown",
		}
	}
}

// regenerateUniqueID 重新生成unique_id
func (p *MixedAttributesProcessor) regenerateUniqueID(sku *types.AIGeneratedSku) {
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
