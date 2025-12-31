// Package utils 提供格式化示例。
package utils

import (
	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// ExampleFormatUsage 展示格式化函数的使用示例
func ExampleFormatUsage() {
	log := logger.GetGlobalLogger("format_example")

	// 重量格式化示例
	log.Info("=== 重量格式化示例 ===")
	weights := []string{"1.234567", "2.5 lb", "0", "abc", "999.999"}
	for _, w := range weights {
		formatted := FormatWeight(w)
		log.WithFields(logrus.Fields{
			"original":  w,
			"formatted": formatted,
		}).Info("重量格式化结果")
	}

	// 尺寸格式化示例
	log.Info("=== 尺寸格式化示例 ===")
	dimensions := []string{"10.567", "15.2 in", "0", "xyz", "10000.5"}
	for _, d := range dimensions {
		formatted := FormatDimension(d)
		log.WithFields(logrus.Fields{
			"original":  d,
			"formatted": formatted,
		}).Info("尺寸格式化结果")
	}
}
