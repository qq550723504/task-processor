// Package extractor 提供Amazon特性提取功能
package extractor

import (
	"strings"

	"github.com/mxschmitt/playwright-go"
)

// FeatureExtractor 特性提取器
type FeatureExtractor struct {
	textCleaner   *TextCleaner
	textValidator *TextValidator
}

// NewFeatureExtractor 创建特性提取器
func NewFeatureExtractor() *FeatureExtractor {
	return &FeatureExtractor{
		textCleaner:   NewTextCleaner(),
		textValidator: NewTextValidator(),
	}
}

// ExtractFeatures 提取产品特性列表
func (e *FeatureExtractor) ExtractFeatures(page playwright.Page) []string {
	var features []string

	// 首先尝试从"About this item"部分提取特性
	aboutSelectors := []string{
		"#feature-bullets ul li",
		"[data-feature-name='aboutThisItem'] ul li",
		"div[data-feature-name='featurebullets'] ul li",
		"#feature-bullets ul li span",
		"[data-feature-name='aboutThisItem'] ul li span",
		"div[data-feature-name='featurebullets'] ul li span",
		"#productDetails_feature_div ul li",
		".a-expander-content span",
		"[data-feature-name='aboutThisItem'] span",
	}

	for _, selector := range aboutSelectors {
		elements, err := page.QuerySelectorAll(selector)
		if err != nil {
			continue
		}

		for _, element := range elements {
			text, err := element.TextContent()
			if err != nil {
				continue
			}

			cleaned := e.textCleaner.CleanDescriptionText(text)
			// 检查是否是"About this item"相关的特性描述
			if e.textValidator.IsAboutItemFeature(cleaned) {
				features = append(features, cleaned)
			}
		}

		if len(features) > 0 {
			break
		}
	}

	return e.deduplicateFeatures(features)
}

// deduplicateFeatures 去重特性列表
func (e *FeatureExtractor) deduplicateFeatures(features []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, feature := range features {
		normalized := strings.ToLower(strings.TrimSpace(feature))
		if !seen[normalized] && normalized != "" {
			seen[normalized] = true
			result = append(result, feature)
		}
	}

	return result
}
