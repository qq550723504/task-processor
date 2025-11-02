package amazon

import (
	"strings"

	"github.com/playwright-community/playwright-go"
)

// TitleExtractor 标题提取器
type TitleExtractor struct{}

func (e *TitleExtractor) Extract(page playwright.Page, product *Product) error {
	title, err := page.TextContent("#productTitle")
	if err != nil {
		return err
	}
	product.Title = strings.TrimSpace(title)
	return nil
}
