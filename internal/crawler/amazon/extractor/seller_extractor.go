package extractor

import (
	"strings"
	"task-processor/internal/model"

	"github.com/mxschmitt/playwright-go"
)

// SellerExtractor 卖家信息提取器
type SellerExtractor struct{}

func (e *SellerExtractor) Extract(page playwright.Page, product *model.Product) error {
	// 使用 JavaScript 提取 Sold by 信息
	// 支持多语言：通过查找包含关键词的标签，而不是精确匹配
	result, err := page.Evaluate(`() => {
		// 多语言关键词列表
		const keywords = [
			'Sold by',         // 英语
			'販売元',          // 日语
			'Vendido por',     // 西班牙语
			'Vendu par',       // 法语
			'Verkauft von',    // 德语
			'Venduto da',      // 意大利语
		];
		
		// 查找所有标签元素
		const labels = Array.from(document.querySelectorAll('span.a-size-small.a-color-tertiary'));
		const soldByLabel = labels.find(el => {
			const text = el.textContent.trim();
			return keywords.some(keyword => text.includes(keyword));
		});
		
		if (soldByLabel) {
			// 找到标签的父容器
			const container = soldByLabel.closest('.offer-display-feature-label');
			if (container && container.nextElementSibling) {
				// 下一个兄弟元素包含实际值
				const valueContainer = container.nextElementSibling;
				const valueSpan = valueContainer.querySelector('.offer-display-feature-text-message');
				if (valueSpan) {
					return valueSpan.textContent.trim();
				}
			}
		}
		return null;
	}`)

	if err == nil && result != nil {
		if sellerName, ok := result.(string); ok && sellerName != "" {
			product.SellerName = sellerName
		}
	}

	// 尝试提取卖家ID（如果有链接）
	sellerLink, err := page.QuerySelector("a[href*='/sp?seller=']")
	if err == nil && sellerLink != nil {
		href, _ := sellerLink.GetAttribute("href")
		if href != "" && strings.Contains(href, "seller=") {
			parts := strings.Split(href, "seller=")
			if len(parts) > 1 {
				sellerID := strings.Split(parts[1], "&")[0]
				product.SellerID = sellerID
			}
		}
	}

	return nil
}
