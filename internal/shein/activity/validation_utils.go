// Package activity 提供SHEIN平台活动报名相关服务
package activity

import (
	"github.com/sirupsen/logrus"
)

const (
	sheinMaximumActivityDiscountRate = 0.8
	sheinMaximumActivityDropRate     = 80
)

// ValidateDropRate 验证并修正降幅参数，确保符合SHEIN活动降幅规则（1-80的正整数）
func ValidateDropRate(dropRate int, originalValue float64, logger *logrus.Entry) int {
	if dropRate < 1 {
		if logger != nil {
			logger.Warnf("降幅过小 (计算值: %d%%, 原值: %.2f%%), 调整为最小值1%%", dropRate, originalValue*100)
		}
		return 1
	}

	if dropRate > sheinMaximumActivityDropRate {
		if logger != nil {
			logger.Warnf("降幅超过SHEIN活动上限 (计算值: %d%%, 原值: %.2f%%), 调整为最大值%d%%", dropRate, originalValue*100, sheinMaximumActivityDropRate)
		}
		return sheinMaximumActivityDropRate
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
