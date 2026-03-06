package temu

import (
	"fmt"
	"task-processor/internal/pkg/mathutil"
)

// absInt 返回 int 的绝对值
// 已废弃: 请使用 mathutil.AbsInt
func absInt(x int) int {
	return mathutil.AbsInt(x)
}

// abs 返回浮点数的绝对值
// 已废弃: 请使用 mathutil.Abs
func abs(x float64) float64 {
	return mathutil.Abs(x)
}

// parseStock 解析库存字符串为整数
func parseStock(stock string) int {
	if stock == "" {
		return 0
	}
	var result int
	fmt.Sscanf(stock, "%d", &result)
	return result
}

// parsePrice 解析价格字符串为浮点数
func parsePrice(price string) float64 {
	if price == "" {
		return 0.0
	}
	var result float64
	fmt.Sscanf(price, "%f", &result)
	return result
}
