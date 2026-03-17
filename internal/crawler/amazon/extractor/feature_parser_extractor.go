package extractor

import (
	"strings"
	"task-processor/internal/model"

	"github.com/playwright-community/playwright-go"
)

// FeatureParserExtractor 特性解析提取器
type FeatureParserExtractor struct{}

func NewFeatureParserExtractor() *FeatureParserExtractor {
	return &FeatureParserExtractor{}
}

func (e *FeatureParserExtractor) Extract(page playwright.Page, product *model.Product) error {
	// 这个提取器主要用于解析和标准化已提取的特性
	// 在基础特性提取器之后运行，对特性进行进一步处理

	// 如果产品已有特性，进行标准化处理
	if len(product.Features) > 0 {
		product.Features = e.normalizeFeatures(product.Features)
	}

	return nil
}

// normalizeFeatures 标准化特性列表
func (e *FeatureParserExtractor) normalizeFeatures(features []string) []string {
	var normalized []string

	for _, feature := range features {
		// 移除多余的空白字符
		normalized = append(normalized, strings.TrimSpace(feature))
	}

	return normalized
}
