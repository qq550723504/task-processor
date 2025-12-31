// Package product 提供产品数据解析功能
package product

import (
	"encoding/json"
	"fmt"
	"strings"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// DataParser 数据解析器
type DataParser struct {
	logger *logrus.Entry
}

// NewDataParser 创建数据解析器
func NewDataParser(logger *logrus.Entry) *DataParser {
	return &DataParser{
		logger: logger.WithField("component", "DataParser"),
	}
}

// ParseAmazonProduct 解析Amazon产品JSON数据
func (p *DataParser) ParseAmazonProduct(jsonData string) (*model.Product, error) {
	if jsonData == "" {
		return nil, fmt.Errorf("JSON数据为空")
	}

	// 首先尝试解析为单个对象
	var product model.Product
	if err := json.Unmarshal([]byte(jsonData), &product); err == nil {
		// 重新计算 IsAvailable 字段（修复历史数据中的错误）
		product.IsAvailable = p.recalculateIsAvailable(&product)
		return &product, nil
	}

	// 如果解析单个对象失败，尝试解析为数组并取第一个元素
	var products []model.Product
	if err := json.Unmarshal([]byte(jsonData), &products); err == nil {
		if len(products) > 0 {
			// 重新计算 IsAvailable 字段（修复历史数据中的错误）
			products[0].IsAvailable = p.recalculateIsAvailable(&products[0])
			return &products[0], nil
		}
		return nil, fmt.Errorf("JSON数组为空")
	}

	return nil, fmt.Errorf("解析JSON数据失败")
}

// recalculateIsAvailable 重新计算产品是否可用
func (p *DataParser) recalculateIsAvailable(product *model.Product) bool {
	lowerText := strings.ToLower(strings.TrimSpace(product.Availability))

	// 不可用的关键词（优先检查）
	unavailableKeywords := []string{
		"currently unavailable", "unavailable", "out of stock",
		"temporarily out of stock", "not available", "discontinued", "sold out",
		"no disponible", "agotado", "sin stock", "temporalmente agotado",
		"actualmente no disponible", "在庫切れ", "一時的に在庫切れ",
		"取り扱い終了", "現在お取り扱いできません",
	}

	for _, keyword := range unavailableKeywords {
		if strings.Contains(lowerText, keyword) {
			p.logger.WithFields(logrus.Fields{
				"asin":         product.Asin,
				"availability": product.Availability,
				"keyword":      keyword,
			}).Debug("❌ 匹配到不可用关键词，判定为不可用")
			return false
		}
	}

	// 可用的关键词
	availableKeywords := []string{
		"in stock", "available", "ships", "delivery", "arrives",
		"left in stock", "more on the way", "usually ships", "in stock soon",
		"disponible", "en stock", "envío", "entrega", "llega",
		"在庫あり", "配送", "お届け", "発送",
	}

	for _, keyword := range availableKeywords {
		if strings.Contains(lowerText, keyword) {
			p.logger.WithFields(logrus.Fields{
				"asin":         product.Asin,
				"availability": product.Availability,
				"keyword":      keyword,
			}).Debug("✅ 匹配到可用关键词，判定为可用")
			return true
		}
	}

	// 无法明确判断时，保持原有值
	p.logger.WithFields(logrus.Fields{
		"asin":           product.Asin,
		"availability":   product.Availability,
		"original_value": product.IsAvailable,
	}).Debug("⚠️ 无法明确判断可用性，保持原有值")
	return product.IsAvailable
}

// ValidateProductData 验证产品数据完整性
func (p *DataParser) ValidateProductData(product *model.Product) error {
	if product == nil {
		return fmt.Errorf("产品数据为空")
	}

	if product.Asin == "" {
		return fmt.Errorf("产品ASIN为空")
	}

	if product.Title == "" {
		return fmt.Errorf("产品标题为空")
	}

	return nil
}

// NormalizeProductData 标准化产品数据
func (p *DataParser) NormalizeProductData(product *model.Product) {
	if product == nil {
		return
	}

	// 标准化字符串字段
	product.Title = strings.TrimSpace(product.Title)
	product.Description = strings.TrimSpace(product.Description)
	product.Availability = strings.TrimSpace(product.Availability)

	// 标准化ASIN
	product.Asin = strings.ToUpper(strings.TrimSpace(product.Asin))

	// 重新计算可用性
	product.IsAvailable = p.recalculateIsAvailable(product)

	p.logger.Debugf("✅ 产品数据标准化完成: ASIN=%s", product.Asin)
}
