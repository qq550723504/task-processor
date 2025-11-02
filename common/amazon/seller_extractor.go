package amazon

import (
	"strings"

	"github.com/playwright-community/playwright-go"
)

// SellerExtractor 卖家信息提取器
type SellerExtractor struct{}

func (e *SellerExtractor) Extract(page playwright.Page, product *Product) error {
	// 卖家名称选择器
	sellerSelectors := []string{
		"#merchant-info a",
		"#sellerProfileTriggerId",
		"a[href*='/sp?seller=']",
		"#merchant-info",
	}

	for _, selector := range sellerSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			text, _ := element.TextContent()
			sellerName := strings.TrimSpace(text)
			if sellerName != "" {
				product.SellerName = sellerName

				// 尝试提取卖家ID
				if href, err := element.GetAttribute("href"); err == nil && href != "" {
					if strings.Contains(href, "seller=") {
						parts := strings.Split(href, "seller=")
						if len(parts) > 1 {
							sellerID := strings.Split(parts[1], "&")[0]
							product.SellerID = sellerID
						}
					}
				}
				break
			}
		}
	}

	return nil
}
