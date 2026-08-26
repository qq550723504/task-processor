// Package format 提供TEMU平台的数值格式化工具。
package format

import temupublishing "task-processor/internal/marketplace/temu/publishing"

// Weight 格式化重量为两位小数
// TEMU API要求重量只能有两位小数
func Weight(weightStr string) string {
	return temupublishing.FormatWeight(weightStr)
}

// Dimension 格式化尺寸为一位小数
// TEMU API要求尺寸只能有一位小数
func Dimension(dimensionStr string) string {
	return temupublishing.FormatDimension(dimensionStr)
}
