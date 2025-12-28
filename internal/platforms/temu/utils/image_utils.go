// Package utils 提供TEMU平台图片处理通用工具方法
package utils

// IntPtr 返回int指针 - 统一的工具函数
func IntPtr(i int) *int {
	return &i
}

// StringPtr 返回string指针
func StringPtr(s string) *string {
	return &s
}

// Float64Ptr 返回float64指针
func Float64Ptr(f float64) *float64 {
	return &f
}

// Abs 计算浮点数绝对值
func Abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// Min 返回两个整数中的较小值
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max 返回两个整数中的较大值
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// DefaultImageDimensions 默认图片尺寸常量
const (
	DefaultImageWidth  = 1500
	DefaultImageHeight = 1500
	MinImageWidth      = 800
	MinImageHeight     = 800
)
