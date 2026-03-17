// Package format 提供TEMU平台的数值格式化工具。
package format

import (
	"fmt"
	"strconv"
	"strings"
)

// Weight 格式化重量为两位小数
// TEMU API要求重量只能有两位小数
func Weight(weightStr string) string {
	return formatNumericString(weightStr, []string{"lb", "磅"}, 0.22, 999.99, "%.2f")
}

// Dimension 格式化尺寸为一位小数
// TEMU API要求尺寸只能有一位小数
func Dimension(dimensionStr string) string {
	return formatNumericString(dimensionStr, []string{"in", "英寸"}, 3.9, 9999.9, "%.1f")
}

// formatNumericString 通用数值字符串格式化：清理单位→解析→范围限制→格式化
func formatNumericString(s string, units []string, defaultVal, maxVal float64, format string) string {
	defaultStr := fmt.Sprintf(format, defaultVal)
	if s == "" {
		return defaultStr
	}

	clean := strings.TrimSpace(s)
	for _, unit := range units {
		clean = strings.ReplaceAll(clean, unit, "")
	}
	clean = strings.TrimSpace(clean)

	v, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return defaultStr
	}

	if v <= 0 {
		v = defaultVal
	} else if v > maxVal {
		v = maxVal
	}

	return fmt.Sprintf(format, v)
}
