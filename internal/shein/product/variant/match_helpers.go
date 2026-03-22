// Package variant 提供SHEIN平台变体匹配辅助工具
package variant

import (
	"task-processor/internal/core/logger"
	"strings"
	"task-processor/internal/shein"

)

// findMatchesWithFunc 通用变体匹配外层循环，matchFn 为具体匹配逻辑
func findMatchesWithFunc(
	variants []shein.Variant,
	attrNames []string,
	targetValueNorm, targetValue string,
	matchLabel string,
	matchFn func(valueNorm, targetNorm string) bool,
) []shein.Variant {
	var matches []shein.Variant

	for variantIndex, variant := range variants {
		matchedInVariant := false
		for _, name := range attrNames {
			if matchedInVariant {
				break
			}
			for attrKey, value := range variant.Attributes {
				if strings.EqualFold(attrKey, name) {
					valueNorm := strings.ToLower(strings.TrimSpace(value))
					if matchFn(valueNorm, targetValueNorm) {
						matches = append(matches, variant)
						logger.GetGlobalLogger("shein/product").Infof("找到%s匹配变体: 变体序号 %d, ASIN %s, 属性名 %s, 属性值 %s, 目标值 %s",
							matchLabel, variantIndex, variant.ASIN, attrKey, value, targetValue)
						matchedInVariant = true
						break
					}
				}
			}
		}
	}

	return matches
}
