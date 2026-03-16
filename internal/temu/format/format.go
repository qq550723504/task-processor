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
	if weightStr == "" {
		return "0.22"
	}

	clean := strings.TrimSpace(weightStr)
	clean = strings.ReplaceAll(clean, "lb", "")
	clean = strings.ReplaceAll(clean, "磅", "")
	clean = strings.TrimSpace(clean)

	w, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return "0.22"
	}

	if w <= 0 {
		w = 0.22
	} else if w > 999.99 {
		w = 999.99
	}

	return fmt.Sprintf("%.2f", w)
}

// Dimension 格式化尺寸为一位小数
// TEMU API要求尺寸只能有一位小数
func Dimension(dimensionStr string) string {
	if dimensionStr == "" {
		return "3.9"
	}

	clean := strings.TrimSpace(dimensionStr)
	clean = strings.ReplaceAll(clean, "in", "")
	clean = strings.ReplaceAll(clean, "英寸", "")
	clean = strings.TrimSpace(clean)

	d, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return "3.9"
	}

	if d <= 0 {
		d = 3.9
	} else if d > 9999.9 {
		d = 9999.9
	}

	return fmt.Sprintf("%.1f", d)
}
