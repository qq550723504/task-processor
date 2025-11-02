package amazon

import (
	"log"
	"strings"

	"github.com/playwright-community/playwright-go"
)

// Extractor 提取器接口
type Extractor interface {
	Extract(page playwright.Page, product *Product) error
}

// CompositeExtractor 组合提取器
type CompositeExtractor struct {
	extractors []Extractor
}

// NewCompositeExtractor 创建组合提取器
func NewCompositeExtractor() *CompositeExtractor {
	return &CompositeExtractor{
		extractors: []Extractor{
			&TitleExtractor{},
			&AvailabilityExtractor{}, // 先提取可用性，价格提取器需要依赖这个信息
			&PriceExtractor{},
			&BrandExtractor{},
			&RatingExtractor{}, // 包含评分和评论数量提取
			&ImageExtractor{},
			&CategoriesExtractor{},   // 分类提取器
			NewParentAsinExtractor(), // Parent ASIN提取器
			&SellerExtractor{},       // 卖家提取器
			&DescriptionExtractor{},
			&ProductDetailsExtractor{},  // 产品详情提取器
			NewVariationsExtractor(),    // 变体提取器
			NewBestsellerExtractor(),    // 畅销排名提取器
			NewFeatureParserExtractor(), // 特性解析提取器
			&FeaturesExtractor{},        // 基础特性提取器
		},
	}
}

// Extract 提取所有信息
func (ce *CompositeExtractor) Extract(page playwright.Page, product *Product) error {
	for _, extractor := range ce.extractors {
		if err := extractor.Extract(page, product); err != nil {
			log.Printf("提取器执行失败 (%T): %v", extractor, err)

			// 检测是否为风控或严重错误（如超时），如果是则立即返回错误
			if ce.isCriticalError(err) {
				log.Printf("检测到关键错误，停止后续提取器执行: %v", err)
				return err
			}
		}
	}
	return nil
}

// isCriticalError 检测是否为关键错误（风控、超时等）
func (ce *CompositeExtractor) isCriticalError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := err.Error()

	// 检测关键错误模式，这些错误表明浏览器实例可能被风控或出现严重问题
	criticalPatterns := []string{
		"timeout", "Timeout", "TIMEOUT",
		"Timeout 30000ms exceeded", // 特定的playwright超时错误
		"blocked", "Blocked", "BLOCKED",
		"captcha", "CAPTCHA", "Captcha",
		"robot", "Robot", "ROBOT",
		"access denied", "Access Denied", "ACCESS DENIED",
		"forbidden", "Forbidden", "FORBIDDEN",
		"503", "502", "504", // 服务器错误
		"connection refused", "Connection refused",
		"network error", "Network error",
		"page crashed", "Page crashed",
		"browser disconnected", "Browser disconnected",
		"context closed", "Context closed",
		"navigation failed", "Navigation failed",
	}

	for _, pattern := range criticalPatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}

	return false
}
