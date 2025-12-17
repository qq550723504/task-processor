package temu

import "fmt"

// absInt 返回 int 的绝对值
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// abs 返回浮点数的绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
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
