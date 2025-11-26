package modules

import (
	"regexp"
	"strings"
	"task-processor/common/shein/api/attribute"

	"github.com/sirupsen/logrus"
)

// VariantMatcher 变体匹配器
type VariantMatcher struct{}

// NewVariantMatcher 创建新的变体匹配器
func NewVariantMatcher() *VariantMatcher {
	return &VariantMatcher{}
}

// FindMatchingVariants 查找匹配的变体 - 优化后的属性名与属性值匹配逻辑
func (m *VariantMatcher) FindMatchingVariants(ctx *TaskContext, variants []Variant, attrID int, targetValue string) []Variant {
	logrus.Infof("🔍 === 开始变体匹配流程 ===")
	logrus.Infof("📊 匹配参数:")
	logrus.Infof("  - 属性ID: %d", attrID)
	logrus.Infof("  - 目标属性值: '%s'", targetValue)
	logrus.Infof("  - 总变体数量: %d", len(variants))

	targetValueTrimmed := strings.TrimSpace(targetValue)
	logrus.Infof("  - 目标属性值(去空格): '%s'", targetValueTrimmed)

	attrName := m.getAttributeName(attrID, ctx.AttributeTemplates)
	attrNameAlternatives := m.getAttributeNameAlternatives(attrID, ctx.AttributeTemplates)

	logrus.Infof("📋 属性名信息:")
	logrus.Infof("  - 主属性名: %s", attrName)
	logrus.Infof("  - 属性名替代: %v", attrNameAlternatives)

	attrNames := append([]string{attrName}, attrNameAlternatives...)
	attrNames = m.removeDuplicates(attrNames)
	logrus.Infof("  - 最终属性名列表: %v", attrNames)

	targetValueNorm := strings.ToLower(targetValueTrimmed)
	logrus.Infof("  - 目标值标准化: '%s'", targetValueNorm)

	matched := []Variant{}

	// 调试信息：打印所有变体的属性值
	logrus.Infof("📋 === 所有变体的 %s 属性值 ===", attrName)
	for i, variant := range variants {
		logrus.Infof("🔍 变体[%d]: ASIN=%s, 价格=%.2f", i+1, variant.ASIN, variant.Price)
		hasTargetAttr := false
		for attrKey, value := range variant.Attributes {
			for _, name := range attrNames {
				if strings.EqualFold(attrKey, name) {
					logrus.Infof("  ✅ %s = '%s' (标准化: '%s')", attrKey, value, strings.ToLower(strings.TrimSpace(value)))
					hasTargetAttr = true
				}
			}
		}
		if !hasTargetAttr {
			logrus.Infof("  ❌ 未找到目标属性")
			// 打印该变体的所有属性
			logrus.Infof("  📋 该变体的所有属性:")
			for attrKey, value := range variant.Attributes {
				logrus.Infof("    %s = '%s'", attrKey, value)
			}
		}
	}

	// 阶段1：精确匹配
	logrus.Infof("🎯 === 阶段1: 精确匹配 ===")
	exactMatches := m.findExactMatches(variants, attrNames, targetValueNorm)
	logrus.Infof("精确匹配结果: %d 个变体", len(exactMatches))

	// 阶段2：验证精确匹配结果是否合理
	if len(exactMatches) > 0 && m.isMatchCountReasonable(exactMatches, targetValue) {
		matched = exactMatches
		logrus.Infof("✅ 使用精确匹配结果: %d 个变体", len(exactMatches))
	} else {
		// 阶段3：如果精确匹配结果不合理，尝试其他匹配策略
		logrus.Infof("⚠️ 精确匹配结果不合理（数量: %d），尝试其他匹配策略", len(exactMatches))

		// 组合值匹配
		logrus.Infof("🎯 === 阶段2: 组合值匹配 ===")
		compositeMatches := m.findCompositeMatches(variants, attrNames, targetValueNorm, targetValue)
		logrus.Infof("组合值匹配结果: %d 个变体", len(compositeMatches))

		if len(compositeMatches) > 0 && m.isMatchCountReasonable(compositeMatches, targetValue) {
			matched = compositeMatches
			logrus.Infof("✅ 使用组合值匹配结果: %d 个变体", len(compositeMatches))
		} else {
			// 模糊匹配（最后的选择）
			logrus.Infof("🎯 === 阶段3: 模糊匹配 ===")
			fuzzyMatches := m.findFuzzyMatches(variants, attrNames, targetValueNorm, targetValue)
			logrus.Infof("模糊匹配结果: %d 个变体", len(fuzzyMatches))

			if len(fuzzyMatches) > 0 {
				matched = fuzzyMatches
				logrus.Infof("✅ 使用模糊匹配结果: %d 个变体", len(fuzzyMatches))
			}
		}
	}

	logrus.Infof("🎉 === 变体匹配完成 ===")
	logrus.Infof("属性值 '%s' 匹配到的变体数量: %d", targetValue, len(matched))

	if len(matched) == 0 {
		logrus.Errorf("❌ 未找到属性ID %d, 属性值 '%s' 的匹配变体", attrID, targetValue)
		// 提供详细的调试信息
		logrus.Errorf("🔍 匹配失败详情:")
		logrus.Errorf("  目标值: '%s' (标准化: '%s')", targetValue, targetValueNorm)
		logrus.Errorf("  可用变体属性值:")
		for i, variant := range variants {
			logrus.Errorf("    变体[%d] ASIN=%s:", i+1, variant.ASIN)
			for attrKey, value := range variant.Attributes {
				for _, name := range attrNames {
					if strings.EqualFold(attrKey, name) {
						logrus.Errorf("      %s = '%s' (标准化: '%s')", attrKey, value, strings.ToLower(strings.TrimSpace(value)))
					}
				}
			}
		}
	} else {
		// 打印匹配结果摘要
		logrus.Infof("📋 匹配变体列表:")
		for i, variant := range matched {
			logrus.Infof("  [%d] ASIN=%s, 价格=%.2f", i+1, variant.ASIN, variant.Price)
		}
	}

	return matched
}

// getAttributeName 获取属性名称
func (m *VariantMatcher) getAttributeName(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) string {
	if attrInfo := m.findAttributeInfoByID(attrID, attributeTemplates); attrInfo != nil {
		if attrInfo.AttributeName != "" {
			return attrInfo.AttributeName
		}
		if attrInfo.AttributeNameEn != "" {
			return attrInfo.AttributeNameEn
		}
	}
	return ""
}

// getAttributeNameAlternatives 获取属性名的替代形式
func (m *VariantMatcher) getAttributeNameAlternatives(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) []string {
	alternatives := make([]string, 0)

	// 从模板中获取
	if attrInfo := m.findAttributeInfoByID(attrID, attributeTemplates); attrInfo != nil {
		if attrInfo.AttributeName != "" {
			alternatives = append(alternatives, attrInfo.AttributeName)
		}
		if attrInfo.AttributeNameEn != "" && attrInfo.AttributeNameEn != attrInfo.AttributeName {
			alternatives = append(alternatives, attrInfo.AttributeNameEn)
		}
	}

	return alternatives
}

// findAttributeInfoByID 根据属性ID查找属性信息
func (m *VariantMatcher) findAttributeInfoByID(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) *attribute.AttributeInfo {
	if attributeTemplates == nil || len(attributeTemplates.Data) == 0 {
		return nil
	}

	for _, data := range attributeTemplates.Data {
		for i := range data.AttributeInfos {
			if data.AttributeInfos[i].AttributeID == attrID {
				return &data.AttributeInfos[i]
			}
		}
	}
	return nil
}

// findExactMatches 查找精确匹配的变体
func (m *VariantMatcher) findExactMatches(variants []Variant, attrNames []string, targetValueNorm string) []Variant {
	var exactMatches []Variant

	for variantIndex, variant := range variants {
		matched_in_variant := false
		for _, name := range attrNames {
			if matched_in_variant {
				break
			}
			for attrKey, value := range variant.Attributes {
				if strings.EqualFold(attrKey, name) {
					valueNorm := strings.ToLower(strings.TrimSpace(value))

					// 精确匹配
					if valueNorm == targetValueNorm {
						exactMatches = append(exactMatches, variant)
						logrus.Infof("找到精确匹配变体: 变体序号 %d, ASIN %s, 属性名 %s, 属性值 %s", variantIndex, variant.ASIN, attrKey, value)
						matched_in_variant = true
						break
					}
				}
			}
		}
	}

	return exactMatches
}

// isMatchCountReasonable 验证匹配结果的数量是否合理
func (m *VariantMatcher) isMatchCountReasonable(matches []Variant, targetValue string) bool {
	// 基本规则：每个属性值应该只匹配一个变体
	// 如果匹配到多个变体，可能是匹配逻辑有问题

	if len(matches) == 0 {
		return false
	}

	// 对于大多数情况，一个属性值应该只对应一个变体
	if len(matches) == 1 {
		logrus.Debugf("匹配数量合理: 属性值 '%s' 匹配到 1 个变体", targetValue)
		return true
	}

	// 如果匹配到多个变体，检查是否是合理的情况
	if len(matches) <= 5 { // 放宽限制，允许更多合理的匹配
		// 对于尺寸等属性，多个变体匹配是正常的（不同颜色相同尺寸）
		logrus.Debugf("匹配数量可接受: 属性值 '%s' 匹配到 %d 个变体", targetValue, len(matches))
		return true
	}

	// 如果匹配到太多变体，可能是匹配逻辑过于宽松
	logrus.Warnf("匹配数量异常: 属性值 '%s' 匹配到 %d 个变体，可能存在匹配逻辑问题", targetValue, len(matches))
	return false
}

// findCompositeMatches 查找组合值匹配的变体
func (m *VariantMatcher) findCompositeMatches(variants []Variant, attrNames []string, targetValueNorm, targetValue string) []Variant {
	var compositeMatches []Variant

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
func (m *VariantMatcher) matchesCompositeValue(variantValue, targetValue string) bool {
	// 标准化处理
	variantNorm := strings.ToLower(strings.TrimSpace(variantValue))
	targetNorm := strings.ToLower(strings.TrimSpace(targetValue))

	// 如果完全相同，直接匹配
	if variantNorm == targetNorm {
		return true
	}

	// 对于尺寸属性，不进行组合值匹配，避免错误匹配
	if m.isSizeAttribute(variantValue, targetValue) {
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
	if m.isColorAttribute(variantValue, targetValue) {
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

// isSizeAttribute 判断是否为尺寸属性
func (m *VariantMatcher) isSizeAttribute(variantValue, targetValue string) bool {
	sizePatterns := []string{"x", "inch", "cm", "mm", "mat", "w/", "to"}

	for _, pattern := range sizePatterns {
		if strings.Contains(variantValue, pattern) || strings.Contains(targetValue, pattern) {
			return true
		}
	}
	return false
}

// findFuzzyMatches 查找模糊匹配的变体
func (m *VariantMatcher) findFuzzyMatches(variants []Variant, attrNames []string, targetValueNorm, targetValue string) []Variant {
	var fuzzyMatches []Variant

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
func (m *VariantMatcher) isValidFuzzyMatch(variantValue, targetValue string) bool {
	// 如果长度差异过大，不进行模糊匹配
	if len(variantValue) > len(targetValue)*2 || len(targetValue) > len(variantValue)*2 {
		logrus.Debugf("长度差异过大，跳过模糊匹配: '%s' vs '%s'", variantValue, targetValue)
		return false
	}

	// 对于尺寸属性，使用更严格的匹配规则
	if m.isSizeAttribute(variantValue, targetValue) {
		return m.isValidSizeFuzzyMatch(variantValue, targetValue)
	}

	// 对于颜色属性，使用相对宽松的匹配规则
	if m.isColorAttribute(variantValue, targetValue) {
		return m.isValidColorFuzzyMatch(variantValue, targetValue)
	}

	// 默认的模糊匹配逻辑
	return strings.Contains(variantValue, targetValue) || strings.Contains(targetValue, variantValue)
}

// isValidColorFuzzyMatch 颜色属性的模糊匹配
func (m *VariantMatcher) isValidColorFuzzyMatch(variantValue, targetValue string) bool {
	// 颜色属性允许相对宽松的模糊匹配
	return strings.Contains(variantValue, targetValue) || strings.Contains(targetValue, variantValue)
}

// isValidSizeFuzzyMatch 尺寸属性的严格模糊匹配
func (m *VariantMatcher) isValidSizeFuzzyMatch(variantValue, targetValue string) bool {
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
func (m *VariantMatcher) extractSizeNumbers(sizeStr string) []string {
	// 使用正则表达式提取数字
	re := regexp.MustCompile(`\d+`)
	return re.FindAllString(sizeStr, -1)
}

// isColorAttribute 判断是否为颜色属性
func (m *VariantMatcher) isColorAttribute(variantValue, targetValue string) bool {
	colorWords := []string{"black", "white", "red", "blue", "green", "yellow", "orange", "purple", "pink", "brown", "gray", "grey", "silver", "gold"}

	variantLower := strings.ToLower(variantValue)
	targetLower := strings.ToLower(targetValue)

	for _, color := range colorWords {
		if strings.Contains(variantLower, color) || strings.Contains(targetLower, color) {
			return true
		}
	}
	return false
}

// isColorPart 检查是否为颜色部分匹配
func (m *VariantMatcher) isColorPart(part, target string) bool {
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

// isSimpleContainment 检查是否为简单的包含关系
func (m *VariantMatcher) isSimpleContainment(variantValue, targetValue string) bool {
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

// removeDuplicates 去除字符串切片中的重复项
func (m *VariantMatcher) removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}
