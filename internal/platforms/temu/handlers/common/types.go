// Package common 提供TEMU平台处理器的共享类型定义
package common

// ImageRequirement 图片要求配置
type ImageRequirement struct {
	MaxSizeMB     float64 // 最大文件大小（MB）
	MinWidth      int     // 最小宽度
	MinHeight     int     // 最小高度
	AspectRatio   float64 // 期望宽高比（严格要求，不允许偏差）
	MinImageCount int     // 最小图片数量
	MaxImageCount int     // 最大图片数量
}
