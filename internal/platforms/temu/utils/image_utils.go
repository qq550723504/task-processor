// Package utils 提供TEMU平台图片处理通用工具方法
package utils

import (
	"task-processor/internal/pkg/mathutil"
	"task-processor/internal/pkg/ptrutil"
)

// IntPtr 返回int指针 - 统一的工具函数
// 已废弃: 请使用 ptrutil.IntPtr
func IntPtr(i int) *int {
	return ptrutil.IntPtr(i)
}

// StringPtr 返回string指针
// 已废弃: 请使用 ptrutil.StringPtr
func StringPtr(s string) *string {
	return ptrutil.StringPtr(s)
}

// Float64Ptr 返回float64指针
// 已废弃: 请使用 ptrutil.Float64Ptr
func Float64Ptr(f float64) *float64 {
	return ptrutil.Float64Ptr(f)
}

// Abs 计算浮点数绝对值
// 已废弃: 请使用 mathutil.Abs
func Abs(x float64) float64 {
	return mathutil.Abs(x)
}

// Min 返回两个整数中的较小值
// 已废弃: 请使用 mathutil.Min
func Min(a, b int) int {
	return mathutil.Min(a, b)
}

// Max 返回两个整数中的较大值
// 已废弃: 请使用 mathutil.Max
func Max(a, b int) int {
	return mathutil.Max(a, b)
}

// DefaultImageDimensions 默认图片尺寸常量
const (
	DefaultImageWidth  = 1500
	DefaultImageHeight = 1500
	MinImageWidth      = 800
	MinImageHeight     = 800
)
