// Package modules 提供SHEIN平台变体匹配工具方法
package variant

import (
	"strings"
)

// VariantMatcherUtils 变体匹配工具类
type VariantMatcherUtils struct{}

// NewVariantMatcherUtils 创建新的变体匹配工具类
func NewVariantMatcherUtils() *VariantMatcherUtils {
	return &VariantMatcherUtils{}
}

// RemoveDuplicates 去除字符串切片中的重复项
func (u *VariantMatcherUtils) RemoveDuplicates(slice []string) []string {
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

// IsSizeAttribute 判断是否为尺寸属性
func (u *VariantMatcherUtils) IsSizeAttribute(variantValue, targetValue string) bool {
	sizePatterns := []string{"x", "inch", "cm", "mm", "mat", "w/", "to"}

	for _, pattern := range sizePatterns {
		if strings.Contains(variantValue, pattern) || strings.Contains(targetValue, pattern) {
			return true
		}
	}
	return false
}

// IsColorAttribute 判断是否为颜色属性
func (u *VariantMatcherUtils) IsColorAttribute(variantValue, targetValue string) bool {
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
