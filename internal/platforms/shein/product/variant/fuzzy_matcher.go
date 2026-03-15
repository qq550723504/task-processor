// Package modules 提供SHEIN平台变体模糊匹配功能
package variant

import (
	"regexp"
	"strings"
	"task-processor/internal/platforms/shein/model"

	"github.com/sirupsen/logrus"
)

// VariantFuzzyMatcher 变体模糊匹配器
type VariantFuzzyMatcher struct {
	utils *VariantMatcherUtils
}

// NewVariantFuzzyMatcher 创建新的变体模糊匹配器
func NewVariantFuzzyMatcher(utils *VariantMatcherUtils) *VariantFuzzyMatcher {
	return &VariantFuzzyMatcher{
		utils: utils,
	}
}

// FindFuzzyMatches 查找模糊匹配的变体
func (m *VariantFuzzyMatcher) FindFuzzyMatches(variants []model.Variant, attrNames []string, targetValueNorm, targetValue string) []model.Variant {
	var fuzzyMatches []model.Variant

	for variantIndex, variant := range variants {
		matched_in_variant := false
		for _, name := range attrNames {
			if matched_in_variant {
				break
			}
			for attrKey, value := range variant.Attributes {
				if strings.EqualFold(attrKey, name) {
					valueNorm := strings.ToLower(strings.TrimSpace(value))

					// 模糊匹配 - 使用更严格的匹配条件
					if m.isValidFuzzyMatch(valueNorm, targetValueNorm) {
						fuzzyMatches = append(fuzzyMatches, variant)
						logrus.Infof("找到模糊匹配变体: 变体序号 %d, ASIN %s, 属性名 %s, 属性值 %s, 目标值 %s", variantIndex, variant.ASIN, attrKey, value, targetValue)
						matched_in_variant = true
						break
					}
				}
			}
		}
	}

	return fuzzyMatches
}

// isValidFuzzyMatch 实现更严格的模糊匹配逻辑，避免错误的包含匹配
func (m *VariantFuzzyMatcher) isValidFuzzyMatch(variantValue, targetValue string) bool {
	// 如果长度差异过大，不进行模糊匹配
	if len(variantValue) > len(targetValue)*2 || len(targetValue) > len(variantValue)*2 {
		logrus.Debugf("长度差异过大，跳过模糊匹配: '%s' vs '%s'", variantValue, targetValue)
		return false
	}

	// 对于尺寸属性，使用更严格的匹配规则
	if m.utils.IsSizeAttribute(variantValue, targetValue) {
		return m.isValidSizeFuzzyMatch(variantValue, targetValue)
	}

	// 对于颜色属性，使用相对宽松的匹配规则
	if m.utils.IsColorAttribute(variantValue, targetValue) {
		return m.isValidColorFuzzyMatch(variantValue, targetValue)
	}

	// 默认的模糊匹配逻辑
	return strings.Contains(variantValue, targetValue) || strings.Contains(targetValue, variantValue)
}

// isValidColorFuzzyMatch 颜色属性的模糊匹配
func (m *VariantFuzzyMatcher) isValidColorFuzzyMatch(variantValue, targetValue string) bool {
	// 颜色属性允许相对宽松的模糊匹配
	return strings.Contains(variantValue, targetValue) || strings.Contains(targetValue, variantValue)
}

// isValidSizeFuzzyMatch 尺寸属性的严格模糊匹配
func (m *VariantFuzzyMatcher) isValidSizeFuzzyMatch(variantValue, targetValue string) bool {
	// 对于尺寸属性，只有在以下情况下才允许模糊匹配：
	// 1. 目标值是变体值的完整子串（如 "4x6" 匹配 "4x6 inch"）
	// 2. 变体值是目标值的完整子串（如 "4x6" 匹配 "4x6"）

	// 提取主要尺寸数字（前两个数字通常是主要尺寸）
	variantNumbers := m.extractSizeNumbers(variantValue)
	targetNumbers := m.extractSizeNumbers(targetValue)

	// 如果主要尺寸数字匹配，则认为是有效匹配
	if len(variantNumbers) >= 2 && len(targetNumbers) >= 2 {
		if variantNumbers[0] == targetNumbers[0] && variantNumbers[1] == targetNumbers[1] {
			logrus.Debugf("主要尺寸数字匹配成功: %v vs %v", variantNumbers[:2], targetNumbers[:2])
			return true
		}
	} else if len(variantNumbers) == len(targetNumbers) && len(variantNumbers) > 0 {
		// 处理数字较少的情况
		allMatch := true
		for i, num := range variantNumbers {
			if num != targetNumbers[i] {
				allMatch = false
				break
			}
		}
		if allMatch {
			logrus.Debugf("所有数字匹配成功: %v vs %v", variantNumbers, targetNumbers)
			return true
		}
	}

	// 严格的包含匹配：只有当一个是另一个的完整前缀或后缀时才匹配
	if strings.HasPrefix(variantValue, targetValue) || strings.HasSuffix(variantValue, targetValue) ||
		strings.HasPrefix(targetValue, variantValue) || strings.HasSuffix(targetValue, variantValue) {
		logrus.Debugf("尺寸前缀/后缀匹配成功: '%s' vs '%s'", variantValue, targetValue)
		return true
	}

	// 检查是否为简单的包含关系，但排除复杂的组合尺寸
	if m.isSimpleContainment(variantValue, targetValue) {
		logrus.Debugf("简单包含匹配成功: '%s' vs '%s'", variantValue, targetValue)
		return true
	}

	logrus.Debugf("尺寸模糊匹配失败: '%s' vs '%s'", variantValue, targetValue)
	return false
}

// extractSizeNumbers 提取尺寸中的数字
func (m *VariantFuzzyMatcher) extractSizeNumbers(sizeStr string) []string {
	// 使用正则表达式提取数字
	re := regexp.MustCompile(`\d+`)
	return re.FindAllString(sizeStr, -1)
}

// isSimpleContainment 检查是否为简单的包含关系
func (m *VariantFuzzyMatcher) isSimpleContainment(variantValue, targetValue string) bool {
	// 如果目标值是变体值的子串，且变体值不包含复杂的描述词，则允许匹配
	if strings.Contains(variantValue, targetValue) {
		// 排除包含复杂描述的情况
		complexWords := []string{"mat to", "w/", "with", "-", "frame", "mount"}
		for _, word := range complexWords {
			if strings.Contains(strings.ToLower(variantValue), word) {
				// 如果包含复杂描述词，只有在目标值也包含相同描述词时才匹配
				if !strings.Contains(strings.ToLower(targetValue), word) {
					return false
				}
			}
		}
		return true
	}

	// 反向检查
	if strings.Contains(targetValue, variantValue) {
		return true
	}

	return false
}
