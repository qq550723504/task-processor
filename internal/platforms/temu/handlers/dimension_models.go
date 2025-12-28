// Package handlers 提供TEMU平台图片尺寸标注相关的数据结构定义
package handlers

// DimensionInfo 尺寸信息
type DimensionInfo struct {
	Length string // 长度（英寸）
	Width  string // 宽度（英寸）
	Height string // 高度（英寸）
}
