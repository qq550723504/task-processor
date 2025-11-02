package amazon

import (
	"strings"

	"github.com/playwright-community/playwright-go"
)

// ProductDetailsExtractor 产品详情提取器
type ProductDetailsExtractor struct{}

func (e *ProductDetailsExtractor) Extract(page playwright.Page, product *Product) error {
	var details []ProductDetail

	// 产品详情表格选择器
	tableSelectors := []string{
		"#technicalSpecifications_section_1 tr",
		"table.a-keyvalue tr",
		"#productDetails_techSpec_section_1 tr",
		"#productDetails_detailBullets_sections1 tr",
	}

	for _, selector := range tableSelectors {
		rows, err := page.QuerySelectorAll(selector)
		if err != nil {
			continue
		}

		for _, row := range rows {
			cells, err := row.QuerySelectorAll("th, td")
			if err != nil || len(cells) < 2 {
				continue
			}

			keyText, err := cells[0].TextContent()
			if err != nil {
				continue
			}
			valueText, err := cells[1].TextContent()
			if err != nil {
				continue
			}

			key := strings.TrimSpace(keyText)
			value := strings.TrimSpace(valueText)

			if key != "" && value != "" {
				details = append(details, ProductDetail{
					Type:  key,
					Value: value,
				})
			}
		}

		if len(details) > 0 {
			break
		}
	}

	product.ProductDetails = details
	return nil
}
