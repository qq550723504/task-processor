package amazon

import (
	"github.com/playwright-community/playwright-go"
)

// ShipsFromExtractor 发货地信息提取器
type ShipsFromExtractor struct{}

// Extract 提取发货地信息
func (e *ShipsFromExtractor) Extract(page playwright.Page, product *Product) error {
	// 使用 JavaScript 提取 Ships from 信息
	// 支持多语言：通过查找包含关键词的标签，而不是精确匹配
	result, err := page.Evaluate(`() => {
		// 多语言关键词列表
		const keywords = [
			'Ships from',      // 英语
			'Envío desde',     // 西班牙语（墨西哥）
			'Enviado desde',   // 西班牙语（其他）
			'出荷元',          // 日语
			'Expédié depuis',  // 法语
			'Versand durch',   // 德语
			'Spedito da',      // 意大利语
		];
		
		// 方法1: 查找 heading + 下一个元素
		const headings = Array.from(document.querySelectorAll('h4'));
		for (const heading of headings) {
			const text = heading.textContent.trim();
			if (keywords.some(kw => text.includes(kw))) {
				if (heading.nextElementSibling) {
					const value = heading.nextElementSibling.textContent.trim();
					if (value) return value;
				}
			}
		}
		
		// 方法2: 查找 offer-display-feature 结构
		const labels = Array.from(document.querySelectorAll('span.a-size-small.a-color-tertiary'));
		for (const label of labels) {
			const text = label.textContent.trim();
			if (keywords.some(kw => text.includes(kw))) {
				const container = label.closest('.offer-display-feature-label');
				if (container && container.nextElementSibling) {
					const valueContainer = container.nextElementSibling;
					const valueSpan = valueContainer.querySelector('.offer-display-feature-text-message');
					if (valueSpan) {
						const value = valueSpan.textContent.trim();
						if (value) return value;
					}
				}
			}
		}
		
		// 方法3: 查找 buybox 区域中的 Ships from
		const buybox = document.querySelector('#buybox');
		if (buybox) {
			const allSpans = Array.from(buybox.querySelectorAll('span'));
			for (let i = 0; i < allSpans.length; i++) {
				const span = allSpans[i];
				const text = span.textContent.trim();
				if (keywords.some(kw => text.includes(kw))) {
					// 查找下一个包含实际值的 span
					if (i + 1 < allSpans.length) {
						const nextSpan = allSpans[i + 1];
						const value = nextSpan.textContent.trim();
						if (value && !keywords.some(kw => value.includes(kw))) {
							return value;
						}
					}
				}
			}
		}
		
		return null;
	}`)

	if err == nil && result != nil {
		if shipsFrom, ok := result.(string); ok && shipsFrom != "" {
			product.ShipsFrom = shipsFrom
			return nil
		}
	}

	return nil
}
