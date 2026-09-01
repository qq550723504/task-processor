package strx

import "fmt"

// ParseInt 解析字符串为整数，失败返回0
func ParseInt(s string) int {
	if s == "" {
		return 0
	}
	var result int
	_, _ = fmt.Sscanf(s, "%d", &result)
	return result
}

// ParseFloat 解析字符串为浮点数，失败返回0.0
func ParseFloat(s string) float64 {
	if s == "" {
		return 0.0
	}
	var result float64
	_, _ = fmt.Sscanf(s, "%f", &result)
	return result
}
