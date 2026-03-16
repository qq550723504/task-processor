package variations

import (
	"strings"
)

// Matcher ASIN匹配器
type Matcher struct {
	config *Config
}

// NewMatcher 创建ASIN匹配器
func NewMatcher(config *Config) *Matcher {
	return &Matcher{config: config}
}

// FindMatchingASIN 通用的ASIN匹配方法，支持动态属性结构
func (m *Matcher) FindMatchingASIN(combo map[string]any, asinMapping map[string]map[string]string) string {
	// 遍历所有ASIN映射，找到属性匹配的ASIN
	for asin, attributes := range asinMapping {
		if m.AttributesMatch(combo, attributes) {
			return asin
		}
	}
	return ""
}

// AttributesMatch 通用的属性匹配方法，支持动态属性结构
func (m *Matcher) AttributesMatch(combo map[string]any, asinAttrs map[string]string) bool {
	// 计算匹配的属性数量
	matchCount := 0
	totalComboAttrs := 0

	// 检查组合中的每个属性是否在ASIN属性中有匹配
	for key, value := range combo {
		if valueStr, ok := value.(string); ok {
			totalComboAttrs++

			// 尝试直接键名匹配
			if asinValue, exists := asinAttrs[key]; exists {
				if m.ValuesMatch(valueStr, asinValue) {
					matchCount++
					continue
				}
			}

			// 如果直接匹配失败，尝试通过值匹配（用于attribute_1, attribute_2等通用键名）
			// 但要确保值是精确匹配的，避免"Black"匹配"Light Brown & Black"
			for _, asinValue := range asinAttrs {
				if m.ValuesMatch(valueStr, asinValue) {
					matchCount++
					break
				}
			}
		}
	}

	// 要求所有属性都完全匹配
	return totalComboAttrs > 0 && matchCount == totalComboAttrs
}

// ValuesMatch 通用的值匹配方法
func (m *Matcher) ValuesMatch(value1, value2 string) bool {
	// 标准化值进行比较
	norm1 := strings.ToLower(strings.TrimSpace(value1))
	norm2 := strings.ToLower(strings.TrimSpace(value2))

	// 精确匹配
	if norm1 == norm2 {
		return true
	}

	// 移除特殊字符后精确匹配（不使用包含匹配，避免"Black"匹配"Light Brown & Black"）
	clean1 := strings.ReplaceAll(strings.ReplaceAll(norm1, "-", ""), " ", "")
	clean2 := strings.ReplaceAll(strings.ReplaceAll(norm2, "-", ""), " ", "")

	return clean1 == clean2
}
