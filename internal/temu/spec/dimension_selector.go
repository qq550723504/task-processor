// Package spec 提供规格维度选择功能
package spec

import (
	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// SpecDimensionSelector 规格维度选择器
type SpecDimensionSelector struct {
	logger *logrus.Entry
}

// NewSpecDimensionSelector 创建规格维度选择器
func NewSpecDimensionSelector() *SpecDimensionSelector {
	return &SpecDimensionSelector{
		logger: logger.GetGlobalLogger("SpecDimensionSelector"),
	}
}

// SelectOptimalDimensions 选择最优的规格维度组合
func (s *SpecDimensionSelector) SelectOptimalDimensions(dimensionStats map[string]int) []string {
	// 按使用频率排序
	type dimensionFreq struct {
		parentSpecID string
		count        int
		priority     int
	}

	var dimensions []dimensionFreq
	totalSkus := 0
	for parentSpecID, count := range dimensionStats {
		priority := s.getDimensionPriority(parentSpecID)
		dimensions = append(dimensions, dimensionFreq{
			parentSpecID: parentSpecID,
			count:        count,
			priority:     priority,
		})
		if count > totalSkus {
			totalSkus = count
		}
	}

	// 排序：优先级高的在前，相同优先级按使用频率排序
	for i := 0; i < len(dimensions)-1; i++ {
		for j := i + 1; j < len(dimensions); j++ {
			if dimensions[i].priority < dimensions[j].priority ||
				(dimensions[i].priority == dimensions[j].priority && dimensions[i].count < dimensions[j].count) {
				dimensions[i], dimensions[j] = dimensions[j], dimensions[i]
			}
		}
	}

	s.logger.Infof("📊 规格维度统计: %+v", dimensionStats)
	s.logger.Infof("📊 总SKU数: %d", totalSkus)

	// 智能选择策略：
	// 1. 如果有一个维度被大多数SKU使用，优先选择它
	// 2. 对于混合属性情况，选择最通用的维度
	var result []string

	// 寻找使用频率最高的维度
	if len(dimensions) > 0 {
		highestFreqDimension := dimensions[0]

		// 如果最高频率维度被超过50%的SKU使用，选择它
		if highestFreqDimension.count >= totalSkus/2 {
			result = append(result, highestFreqDimension.parentSpecID)
			s.logger.Infof("🎯 选择高频维度: %s (使用率: %d/%d)",
				highestFreqDimension.parentSpecID, highestFreqDimension.count, totalSkus)
		} else {
			// 混合属性情况：选择最通用的维度
			// 优先选择Style或Items维度，因为它们可以表示各种类型的变体
			universalDimension := s.selectUniversalDimension(dimensionStats)
			if universalDimension != "" {
				result = append(result, universalDimension)
				s.logger.Infof("🎯 选择通用维度: %s (适合混合属性)", universalDimension)
			} else {
				// 如果没有通用维度，选择优先级最高的
				result = append(result, highestFreqDimension.parentSpecID)
				s.logger.Infof("🎯 选择优先级最高维度: %s", highestFreqDimension.parentSpecID)
			}
		}
	}

	// TEMU限制：最多2个规格维度，但对于混合属性通常1个就够了
	if len(result) < 2 && len(dimensions) > 1 {
		// 只有在明确需要第二个维度时才添加
		for _, dim := range dimensions {
			if dim.parentSpecID != result[0] && dim.count >= totalSkus/3 {
				result = append(result, dim.parentSpecID)
				s.logger.Infof("🎯 添加第二维度: %s", dim.parentSpecID)
				break
			}
		}
	}

	s.logger.Infof("🎯 最终选择的统一规格维度: %v", result)
	return result
}

// selectUniversalDimension 选择最通用的维度来处理混合属性
func (s *SpecDimensionSelector) selectUniversalDimension(dimensionStats map[string]int) string {
	// 优先级：Style > Items > Size > 其他
	// Style和Items维度最适合表示各种类型的变体
	universalPriority := map[string]int{
		"18012": 100, // Style - 最通用
		"17020": 90,  // Items - 适合数量和套装
		"3001":  80,  // Size - 适合尺寸
		"1001":  70,  // Color - 适合颜色
	}

	bestDimension := ""
	bestScore := -1

	for parentSpecID := range dimensionStats {
		if score, exists := universalPriority[parentSpecID]; exists {
			if score > bestScore {
				bestScore = score
				bestDimension = parentSpecID
			}
		}
	}

	return bestDimension
}

// getDimensionPriority 获取规格维度的优先级
func (s *SpecDimensionSelector) getDimensionPriority(parentSpecID string) int {
	// 颜色相关规格优先级最高
	if parentSpecID == "1001" { // Color
		return 100
	}

	// 尺寸相关规格优先级次高
	if parentSpecID == "3001" { // Size
		return 90
	}

	// 其他常见规格
	switch parentSpecID {
	case "18014": // Capacity
		return 80
	case "18012": // Style
		return 70
	case "17017": // Material
		return 60
	default:
		return 50
	}
}
