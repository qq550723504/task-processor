package amazon

import (
	"strings"

	"github.com/playwright-community/playwright-go"
)

// DescriptionExtractor 描述提取器
type DescriptionExtractor struct{}

func (e *DescriptionExtractor) Extract(page playwright.Page, product *Product) error {
	// 描述选择器
	descriptionSelectors := []string{
		"#feature-bullets ul",
		"#aplus_feature_div",
		"#productDescription",
		"#feature-bullets",
	}

	for _, selector := range descriptionSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			text, _ := element.TextContent()
			description := strings.TrimSpace(text)
			if description != "" && len(description) > 20 {
				product.Description = description
				break
			}
		}
	}

	return nil
}
