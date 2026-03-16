// Package operation 提供SHEIN平台调度器相关服务
package operation

import (
	"github.com/sirupsen/logrus"
)

// ValidateDropRate 验证并修正降幅参数，确保符合SHEIN API要求（1-99的正整数）
func ValidateDropRate(dropRate int, originalValue float64, logger *logrus.Entry) int {
	if dropRate < 1 {
		if logger != nil {
			logger.Warnf("降幅过小 (计算值: %d%%, 原值: %.2f%%), 调整为最小值1%%", dropRate, originalValue*100)
		}
		return 1
	}

	if dropRate >= 100 {
		if logger != nil {
			logger.Warnf("降幅过大 (计算值: %d%%, 原值: %.2f%%), 调整为最大值99%%", dropRate, originalValue*100)
		}
		return 99
	}

	return dropRate
}

// CalculateDropRateFromDiscount 从折扣率计算降幅，并进行验证
func CalculateDropRateFromDiscount(discountRate float64, logger *logrus.Entry) int {
	dropRate := int(discountRate * 100)

	// 特殊处理：如果折扣率为0或负数，使用默认值10%
	if discountRate <= 0 {
		if logger != nil {
			logger.Warnf("折扣率无效 (%.2f%%), 使用默认值10%%", discountRate*100)
		}
		return 10
	}

	// 特殊处理：如果折扣率过大，使用默认值10%
	if discountRate >= 1.0 {
		if logger != nil {
			logger.Warnf("折扣率过大 (%.2f%%), 使用默认值10%%", discountRate*100)
		}
		return 10
	}

	return ValidateDropRate(dropRate, discountRate, logger)
}
