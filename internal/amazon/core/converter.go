// Package core 提供Amazon平台工具方法
package core

import (
	"fmt"
	"strings"
)

// Converter 数据转换器
type Converter struct{}

// NewConverter 创建转换器
func NewConverter() *Converter {
	return &Converter{}
}

// FormatPrice 格式化价格
func (c *Converter) FormatPrice(price float64, currency string) string {
	return fmt.Sprintf("%.2f %s", price, currency)
}

// ParsePrice 解析价格字符串
func (c *Converter) ParsePrice(priceStr string) (float64, error) {
	var price float64
	_, err := fmt.Sscanf(priceStr, "%f", &price)
	return price, err
}

// NormalizeASIN 规范化ASIN
func (c *Converter) NormalizeASIN(asin string) string {
	return strings.ToUpper(strings.TrimSpace(asin))
}

// NormalizeSKU 规范化SKU
func (c *Converter) NormalizeSKU(sku string) string {
	return strings.TrimSpace(sku)
}

// ValidateASIN 验证ASIN格式
func (c *Converter) ValidateASIN(asin string) bool {
	asin = c.NormalizeASIN(asin)
	return len(asin) == 10
}

// ValidateSKU 验证SKU格式
func (c *Converter) ValidateSKU(sku string) bool {
	sku = c.NormalizeSKU(sku)
	return len(sku) > 0 && len(sku) <= 40
}
