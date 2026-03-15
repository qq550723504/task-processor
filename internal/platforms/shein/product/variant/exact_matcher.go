// Package modules 提供SHEIN平台变体精确匹配功能
package variant

import (
	"strings"
	"task-processor/internal/platforms/shein"

	"github.com/sirupsen/logrus"
)

// VariantExactMatcher 变体精确匹配器
type VariantExactMatcher struct{}

// NewVariantExactMatcher 创建新的变体精确匹配器
func NewVariantExactMatcher() *VariantExactMatcher {
	return &VariantExactMatcher{}
}

// FindExactMatches 查找精确匹配的变体
func (m *VariantExactMatcher) FindExactMatches(variants []shein.Variant, attrNames []string, targetValueNorm string) []shein.Variant {
	var exactMatches []shein.Variant

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


