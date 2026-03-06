package sku

import (
	"github.com/sirupsen/logrus"
)

// SkuPriceCalculator SKU价格计算器
type SkuPriceCalculator struct {
	logger *logrus.Entry
}

// NewSkuPriceCalculator 创建新的价格计算器
func NewSkuPriceCalculator(logger *logrus.Entry) *SkuPriceCalculator {
	return &SkuPriceCalculator{
		logger: logger,
	}
}
