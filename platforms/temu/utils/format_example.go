// Package utils 提供格式化示例。
package utils

import "fmt"

// ExampleFormatUsage 展示格式化函数的使用示例
func ExampleFormatUsage() {
	// 重量格式化示例
	fmt.Println("=== 重量格式化示例 ===")
	weights := []string{"1.234567", "2.5 lb", "0", "abc", "999.999"}
	for _, w := range weights {
		formatted := FormatWeight(w)
		fmt.Printf("原始: %-10s -> 格式化: %s\n", w, formatted)
	}

	// 尺寸格式化示例
	fmt.Println("\n=== 尺寸格式化示例 ===")
	dimensions := []string{"10.567", "15.2 in", "0", "xyz", "10000.5"}
	for _, d := range dimensions {
		formatted := FormatDimension(d)
		fmt.Printf("原始: %-10s -> 格式化: %s\n", d, formatted)
	}
}
