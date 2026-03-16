// Package variant 提供SHEIN平台变体组合值匹配功能
package variant

import (
	"strings"
	"task-processor/internal/shein"

	"github.com/sirupsen/logrus"
)

// VariantCompositeMatcher 变体组合值匹配器
type VariantCompositeMatcher struct {
	utils *VariantMatcherUtils
}

// NewVariantCompositeMatcher 创建新的变体组合值匹配器
func NewVariantCompositeMatcher(utils *VariantMatcherUtils) *VariantCompositeMatcher {
	return &VariantCompositeMatcher{
		utils: utils,
	}
}

// FindCompositeMatches 查找组合值匹配的变体
func (m *VariantCompositeMatcher) FindCompositeMatches(variants []shein.Variant, attrNames []string, targetValueNorm, targetValue string) []shein.Variant {
	var compositeMatches []shein.Variant

	for variantIndex, variant := range variants {
		matched_in_variant := false
		for _, name := range attrNames {
			if matched_in_variant {
				break
			}
			for attrKey, value := range variant.Attributes {
				if strings.EqualFold(attrKey, name) {
					valueNorm := strings.ToLower(strings.TrimSpace(value))

					// 组合值匹配
					if m.matchesCompositeValue(valueNorm, targetValueNorm) {
						compositeMatches = append(compositeMatches, variant)
						logrus.Infof("找到组合值匹配变体: 变体序号 %d, ASIN %s, 属性名 %s, 属性值 %s, 目标值 %s", variantIndex, variant.ASIN, attrKey, value, targetValue)
						matched_in_variant = true
						break
					}
				}
			}
		}
	}

	return compositeMatches
}

// matchesCompositeValue 检查组合属性值是否匹配（通用函数）
func (m *VariantCompositeMatcher) matchesCompositeValue(variantValue, targetValue string) bool {
	// 标准化处理
	variantNorm := strings.ToLower(strings.TrimSpace(variantValue))
	targetNorm := strings.ToLower(strings.TrimSpace(targetValue))

	// 如果完全相同，直接匹配
	if variantNorm == targetNorm {
		return true
	}

	// 对于尺寸属性，不进行组合值匹配，避免错误匹配
	if m.utils.IsSizeAttribute(variantValue, targetValue) {
		logrus.Debugf("尺寸属性跳过组合值匹配: '%s' vs '%s'", variantValue, targetValue)
		return false
	}

	// 处理组合值（如 "Black/Royal Blue" 匹配 "Royal Blue"）
	// 注意：不包含空格分隔符，因为颜色名称可能包含空格（如 "Royal Blue"）
	separators := []string{"/", ",", "|"} // 移除 "-" 分隔符，避免尺寸属性的误匹配

	for _, sep := range separators {
		if strings.Contains(variantNorm, sep) {
			parts := strings.Split(variantNorm, sep)
			for _, part := range parts {
				part = strings.TrimSpace(part)
				// 精确匹配整个部分
				if part == targetNorm {
					logrus.Infof("组合值匹配成功: '%s' 包含 '%s'", variantValue, targetValue)
					return true
				}
				// 对于颜色属性，支持部分匹配（如 "Medium Grey" 匹配 "Grey"）
				if m.isColorPart(part, targetNorm) {
					logrus.Infof("颜色部分匹配成功: '%s' 中的 '%s' 匹配 '%s'", variantValue, part, targetValue)
					return true
				}
			}
		}
	}

	// 反向匹配：目标值包含变体值的情况（仅限颜色属性）
	if m.utils.IsColorAttribute(variantValue, targetValue) {
		for _, sep := range separators {
			if strings.Contains(targetNorm, sep) {
				parts := strings.Split(targetNorm, sep)
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if part == variantNorm {
						logrus.Infof("反向组合值匹配成功: '%s' 包含 '%s'", targetValue, variantValue)
						return true
					}
				}
			}
		}
	}
	return false
}

// isColorPart 检查是否为颜色部分匹配
func (m *VariantCompositeMatcher) isColorPart(part, target string) bool {
	// 支持颜色修饰词 + 基础颜色的匹配
	// 如 "Medium Grey" 匹配 "Grey", "Dark Blue" 匹配 "Blue"
	colorModifiers := []string{"light", "dark", "medium", "bright", "deep", "pale"}

	partWords := strings.Fields(part)
	targetWords := strings.Fields(target)

	// 如果目标是单个颜色词，检查部分是否包含该颜色词
	if len(targetWords) == 1 && len(partWords) == 2 {
		targetWord := targetWords[0]

		// 检查是否有一个词是目标颜色，另一个词是修饰词
		var hasTargetColor, hasModifier bool
		for _, word := range partWords {
			if word == targetWord {
				hasTargetColor = true
			}
			for _, modifier := range colorModifiers {
				if word == modifier {
					hasModifier = true
					break
				}
			}
		}

		// 只有当同时包含目标颜色和修饰词时才匹配
		if hasTargetColor && hasModifier {
			return true
		}
	}

	return false
}
