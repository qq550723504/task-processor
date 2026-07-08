package extractor

import (
	"strings"
	"task-processor/internal/model"
	"time"

	"github.com/mxschmitt/playwright-go"
)

// BrandExtractor 品牌提取器
type BrandExtractor struct{}

func (e *BrandExtractor) Extract(page playwright.Page, product *model.Product) error {
	if page != nil {
		_, _ = page.WaitForSelector(strings.Join(brandSelectors(), ", "), playwright.PageWaitForSelectorOptions{
			State:   playwright.WaitForSelectorStateAttached,
			Timeout: playwright.Float(float64((1500 * time.Millisecond).Milliseconds())),
		})
	}

	for _, selector := range brandSelectors() {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			text, _ := element.InnerText()
			if strings.TrimSpace(text) == "" {
				text, _ = element.TextContent()
			}
			if brand := normalizeBrandText(text); brand != "" {
				product.Brand = brand
				return nil
			}
		}
	}

	return nil
}

func brandSelectors() []string {
	return []string{
		"#bylineInfo",
		"a#brand",
		".po-brand .po-break-word",
		"#productBrandLogo_feature_div",
		"#productBrandLogo_feature_div a",
		"#productBrandLogo_feature_div .a-link-normal",
	}
}

func normalizeBrandText(value string) string {
	brand := strings.TrimSpace(value)

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

	suffixes := []string{
		" Store",    // 英语
		" tienda",   // 西班牙语
		" loja",     // 葡萄牙语
		" boutique", // 法语
		"ストア",       // 日语 Store
		" ストア",      // 日语 Store（带空格）
	}

	for _, prefix := range prefixes {
		brand = strings.TrimPrefix(brand, prefix)
	}

	for _, suffix := range suffixes {
		brand = strings.TrimSuffix(brand, suffix)
	}

	return strings.TrimSpace(brand)
}
