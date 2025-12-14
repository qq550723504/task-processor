// Package utils 提供TEMU平台的格式验证和转换工具。
package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatWeight 格式化重量为两位小数
// TEMU API要求重量只能有两位小数
func FormatWeight(weightStr string) string {
	if weightStr == "" {
		return "0.22" // 默认重量
	}

	// 清理字符串，移除单位和空格
	cleanWeight := strings.TrimSpace(weightStr)
	cleanWeight = strings.ReplaceAll(cleanWeight, "lb", "")
	cleanWeight = strings.ReplaceAll(cleanWeight, "磅", "")
	cleanWeight = strings.TrimSpace(cleanWeight)

	// 解析为浮点数
	weight, err := strconv.ParseFloat(cleanWeight, 64)
	if err != nil {
		return "0.22" // 解析失败时使用默认值
	}

	// 确保重量为正数且合理范围内
	if weight <= 0 {
		weight = 0.22
	} else if weight > 999.99 {
		weight = 999.99 // 限制最大重量
	}

	// 格式化为两位小数
	return fmt.Sprintf("%.2f", weight)
}

// FormatDimension 格式化尺寸为一位小数
// TEMU API要求尺寸只能有一位小数
func FormatDimension(dimensionStr string) string {
	if dimensionStr == "" {
		return "3.9" // 默认尺寸
	}

	// 清理字符串，移除单位和空格
	cleanDimension := strings.TrimSpace(dimensionStr)
	cleanDimension = strings.ReplaceAll(cleanDimension, "in", "")
	cleanDimension = strings.ReplaceAll(cleanDimension, "英寸", "")
	cleanDimension = strings.TrimSpace(cleanDimension)

	// 解析为浮点数
	dimension, err := strconv.ParseFloat(cleanDimension, 64)
	if err != nil {
		return "3.9" // 解析失败时使用默认值
	}

	// 确保尺寸为正数且合理范围内
	if dimension <= 0 {
		dimension = 3.9
	} else if dimension > 9999.9 {
		dimension = 9999.9 // 限制最大尺寸
	}

	// 格式化为一位小数
	return fmt.Sprintf("%.1f", dimension)
}
