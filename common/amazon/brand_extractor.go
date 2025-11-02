package amazon

import (
	"strings"

	"github.com/playwright-community/playwright-go"
)

// BrandExtractor 品牌提取器
type BrandExtractor struct{}

func (e *BrandExtractor) Extract(page playwright.Page, product *Product) error {
	selectors := []string{
		"#bylineInfo",
		"a#brand",
		".po-brand .po-break-word",
	}

	for _, selector := range selectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			text, _ := element.TextContent()
			brand := strings.TrimSpace(text)
			brand = strings.TrimPrefix(brand, "Visit the ")
			brand = strings.TrimPrefix(brand, "Brand: ")
			brand = strings.TrimSuffix(brand, " Store")
			product.Brand = brand
			return nil
		}
	}

	return nil
}
