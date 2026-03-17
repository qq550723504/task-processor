// Package extractor 提供Amazon描述提取核心功能
package extractor

import (
	"task-processor/internal/model"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// DescriptionExtractor 描述提取器
type DescriptionExtractor struct {
	textCleaner   *TextCleaner
	textValidator *TextValidator
}

// NewDescriptionExtractor 创建描述提取器
func NewDescriptionExtractor() *DescriptionExtractor {
	return &DescriptionExtractor{
		textCleaner:   NewTextCleaner(),
		textValidator: NewTextValidator(),
	}
}

// Extract 提取描述信息
func (e *DescriptionExtractor) Extract(page playwright.Page, product *model.Product) error {
	// 提取description
	description := e.extractDescription(page)
	if description != "" {
		product.Description = description
	}

	// 提取product_description
	productDescription := e.extractProductDescription(page)
	if productDescription != "" {
		product.ProductDescription = []model.Description{
			{
				Text: productDescription,
				Type: "text",
			},
		}
	}

	// 提取features（如果FeaturesExtractor没有提取到）
	// 注意：特性提取现在由 FeaturesExtractor 专门处理
	// 这里不再重复提取，避免依赖混乱

	return nil
}

// extractDescription 提取商品描述
func (e *DescriptionExtractor) extractDescription(page playwright.Page) string {
	selectors := []string{
		"#feature-bullets ul li span.a-list-item",
		"#feature-bullets .a-list-item",
		"[data-feature-name='featurebullets'] ul li span",
		"#productDescription p",
	}

	return e.extractTextFromSelectors(page, selectors, 50, e.textValidator.IsValidDescription)
}

// extractProductDescription 提取产品详细描述
func (e *DescriptionExtractor) extractProductDescription(page playwright.Page) string {
	logrus.Info("开始提取产品详细描述")

	// 第一组：精确的产品描述选择器
	primarySelectors := []string{
		"#productDescription p",
		"#productDescription div.a-section",
		"[data-feature-name='productDescription'] p",
		"#product-description-section p",
		".product-description p",
	}

	if result := e.extractTextFromSelectors(page, primarySelectors, 50, e.textValidator.IsValidProductDescription); result != "" {
		return result
	}

	// 第二组：从 A+ Content 中提取
	aplusSelectors := []string{
		"#aplus .aplus-module p",
		"#aplus_feature_div .celwidget p",
		".aplus-v2 .aplus-module-content p",
	}

	if result := e.extractTextFromSelectors(page, aplusSelectors, 50, e.textValidator.IsValidProductDescription); result != "" {
		return result
	}

	// 第三组：从 feature bullets 中提取（作为最后的备选）
	if description := e.extractDescription(page); description != "" && len(description) > 100 {
		return description
	}

	logrus.Info("未能提取到产品详细描述")
	return ""
}

// extractTextFromSelectors 从选择器列表中提取文本
func (e *DescriptionExtractor) extractTextFromSelectors(page playwright.Page, selectors []string, minLength int, validator func(string) bool) string {
	for _, selector := range selectors {
		elements, err := page.QuerySelectorAll(selector)
		if err != nil {
			continue
		}

		var descriptions []string
		for _, element := range elements {
			text, err := element.InnerText()
			if err != nil {
				continue
			}

			cleaned := e.textCleaner.CleanDescriptionText(text)
			if validator(cleaned) && len(cleaned) > 10 {
				descriptions = append(descriptions, cleaned)
			}
		}

		if len(descriptions) > 0 {
			result := e.textCleaner.JoinDescriptions(descriptions)
			if len(result) > minLength {
				return result
			}
		}
	}

	return ""
}

