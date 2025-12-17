package extractor

import (
	"strings"
	"task-processor/internal/common/amazon/model"

	"github.com/playwright-community/playwright-go"
)

// BrandExtractor 品牌提取器
type BrandExtractor struct{}

func (e *BrandExtractor) Extract(page playwright.Page, product *model.Product) error {
	selectors := []string{
		"#bylineInfo",
		"a#brand",
		".po-brand .po-break-word",
	}

	// 多语言品牌前缀列表
	prefixes := []string{
		"Visit the ",           // 英语
		"Visita la tienda de ", // 西班牙语（墨西哥、西班牙等）
		"Visitar a loja de ",   // 葡萄牙语（巴西）
		"Visiter la boutique ", // 法语
		"Besuche den ",         // 德语
		"Visita lo store di ",  // 意大利语
		"ブランド: ",               // 日语 Brand:
		"ストアにアクセス ",            // 日语 访问商店
		"Brand: ",              // 通用
	}

	// 多语言品牌后缀列表
	suffixes := []string{
		" Store",    // 英语
		" tienda",   // 西班牙语
		" loja",     // 葡萄牙语
		" boutique", // 法语
		"ストア",       // 日语 Store
		" ストア",      // 日语 Store（带空格）
	}

	for _, selector := range selectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			text, _ := element.TextContent()
			brand := strings.TrimSpace(text)

			// 移除所有可能的前缀
			for _, prefix := range prefixes {
				brand = strings.TrimPrefix(brand, prefix)
			}

			// 移除所有可能的后缀
			for _, suffix := range suffixes {
				brand = strings.TrimSuffix(brand, suffix)
			}

			product.Brand = strings.TrimSpace(brand)
			return nil
		}
	}

	return nil
}
