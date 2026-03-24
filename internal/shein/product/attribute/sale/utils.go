package sale

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/types"
	sheinattr "task-processor/internal/shein/product/attribute"
)

// parseFloat 安全地解析浮点数
func parseFloat(str string) float64 {
	if str == "" {
		return 0
	}

	// 移除可能的单位后缀和空格
	str = strings.TrimSpace(str)

	// 使用正则表达式提取数字部分
	re := regexp.MustCompile(`^(\d+\.?\d*)`)
	matches := re.FindStringSubmatch(str)
	if len(matches) > 1 {
		if result, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return result
		}
	}

	return 0
}

// convertMillimetersToCentimeters 将毫米转换为厘米
func convertMillimetersToCentimeters(variant *sheinattr.Variant) {
	if length := parseFloat(variant.Length.String()); length > 0 {
		variant.Length = types.FlexibleString(fmt.Sprintf("%.1f", length/10))
	}
	if width := parseFloat(variant.Width.String()); width > 0 {
		variant.Width = types.FlexibleString(fmt.Sprintf("%.1f", width/10))
	}
	if height := parseFloat(variant.Height.String()); height > 0 {
		variant.Height = types.FlexibleString(fmt.Sprintf("%.1f", height/10))
	}
	logger.GetGlobalLogger("shein/product").Infof("ASIN %s: 已将尺寸从毫米转换为厘米", variant.ASIN)
}

// convertMetersToCentimeters 将米转换为厘米
func convertMetersToCentimeters(variant *sheinattr.Variant) {
	if length := parseFloat(variant.Length.String()); length > 0 {
		variant.Length = types.FlexibleString(fmt.Sprintf("%.1f", length*100))
	}
	if width := parseFloat(variant.Width.String()); width > 0 {
		variant.Width = types.FlexibleString(fmt.Sprintf("%.1f", width*100))
	}
	if height := parseFloat(variant.Height.String()); height > 0 {
		variant.Height = types.FlexibleString(fmt.Sprintf("%.1f", height*100))
	}
	logger.GetGlobalLogger("shein/product").Infof("ASIN %s: 已将尺寸从米转换为厘米", variant.ASIN)
}

// convertInchesToCentimeters 将英寸转换为厘米 (1 inch = 2.54 cm)
func convertInchesToCentimeters(variant *sheinattr.Variant) {
	if length := parseFloat(variant.Length.String()); length > 0 {
		variant.Length = types.FlexibleString(fmt.Sprintf("%.1f", length*2.54))
	}
	if width := parseFloat(variant.Width.String()); width > 0 {
		variant.Width = types.FlexibleString(fmt.Sprintf("%.1f", width*2.54))
	}
	if height := parseFloat(variant.Height.String()); height > 0 {
		variant.Height = types.FlexibleString(fmt.Sprintf("%.1f", height*2.54))
	}
	logger.GetGlobalLogger("shein/product").Infof("ASIN %s: 已将尺寸从英寸转换为厘米 (长=%s, 宽=%s, 高=%s)",
		variant.ASIN, variant.Length.String(), variant.Width.String(), variant.Height.String())
}

// convertFeetToCentimeters 将英尺转换为厘米 (1 ft = 30.48 cm)
func convertFeetToCentimeters(variant *sheinattr.Variant) {
	if length := parseFloat(variant.Length.String()); length > 0 {
		variant.Length = types.FlexibleString(fmt.Sprintf("%.1f", length*30.48))
	}
	if width := parseFloat(variant.Width.String()); width > 0 {
		variant.Width = types.FlexibleString(fmt.Sprintf("%.1f", width*30.48))
	}
	if height := parseFloat(variant.Height.String()); height > 0 {
		variant.Height = types.FlexibleString(fmt.Sprintf("%.1f", height*30.48))
	}
	logger.GetGlobalLogger("shein/product").Infof("ASIN %s: 已将尺寸从英尺转换为厘米 (长=%s, 宽=%s, 高=%s)",
		variant.ASIN, variant.Length.String(), variant.Width.String(), variant.Height.String())
}

// isAttributeNameSimilar 检查两个属性名是否相似
func isAttributeNameSimilar(name1, name2 string) bool {
	// 转换为小写并移除常见的分隔符
	normalize := func(name string) string {
		name = strings.ToLower(name)
		name = strings.ReplaceAll(name, " ", "")
		name = strings.ReplaceAll(name, "_", "")
		name = strings.ReplaceAll(name, "-", "")
		return name
	}

	normalized1 := normalize(name1)
	normalized2 := normalize(name2)

	// 精确匹配
	if normalized1 == normalized2 {
		return true
	}

	// 包含匹配
	if strings.Contains(normalized1, normalized2) || strings.Contains(normalized2, normalized1) {
		return true
	}

	// 常见映射
	mappings := map[string][]string{
		"color": {"colour", "colorname"},
		"size":  {"sizesize", "itempackagequantity"},
		"style": {"stylename", "patternname"},
		"scent": {"scentname", "flavorname"},
	}

	for key, values := range mappings {
		if normalized1 == key {
			for _, value := range values {
				if normalized2 == value {
					return true
				}
			}
		}
		if normalized2 == key {
			for _, value := range values {
				if normalized1 == value {
					return true
				}
			}
		}
	}

	return false
}

// convertToSet 将字符串数组转换为集合
func convertToSet(values []string) map[string]bool {
	set := make(map[string]bool)
	for _, value := range values {
		// 标准化值（去除首尾空格，转为小写进行比较）
		normalizedValue := strings.ToLower(strings.TrimSpace(value))
		set[normalizedValue] = true
	}
	return set
}

// getKeysFromMap 从map中获取所有键
func getKeysFromMap(m map[string]bool) []string {
	var keys []string
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

// isValueSetEqual 检查两个值集合是否相等
func isValueSetEqual(set1, set2 map[string]bool) bool {
	if len(set1) != len(set2) {
		return false
	}

	for value := range set1 {
		if !set2[value] {
			return false
		}
	}

	return true
}

// looksLikeCompleteJson 检查内容是否看起来像完整的JSON
func looksLikeCompleteJson(content string) bool {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		return false
	}
	openBraces := strings.Count(trimmed, "{")
	closeBraces := strings.Count(trimmed, "}")
	openBrackets := strings.Count(trimmed, "[")
	closeBrackets := strings.Count(trimmed, "]")

	return openBraces == closeBraces && openBrackets == closeBrackets
}
