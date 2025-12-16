package extractor

import (
	"task-processor/common/amazon/model"

	"github.com/playwright-community/playwright-go"
)

// ShipsFromExtractor 发货地信息提取器
type ShipsFromExtractor struct{}

// Extract 提取发货地信息
func (e *ShipsFromExtractor) Extract(page playwright.Page, product *model.Product) error {
	// 使用 JavaScript 提取 Ships from 信息
	// 支持多语言：通过查找包含关键词的标签，而不是精确匹配
	result, err := page.Evaluate(`() => {
		// 多语言关键词列表
		const keywords = [
			'Ships from',      // 英语
			'Shipper',         // 英语（新格式）
			'Dispatched from', // 英语（英国站）
			'Envío desde',     // 西班牙语（墨西哥）
			'Enviado desde',   // 西班牙语（其他）
			'Remitente',       // 西班牙语（墨西哥站）
			'出荷元',          // 日语
			'発送元',          // 日语（另一种）
			'Expédié par',     // 法语
			'Expédié depuis',  // 法语
			'Expéditeur',      // 法语（新格式）
			'Versand durch',   // 德语
			'Versender',       // 德语（新格式）
			'Spedito da',      // 意大利语
			'Mittente',        // 意大利语（新格式）
			'Enviado por',     // 葡萄牙语（巴西站）
			'Remetente',       // 葡萄牙语（新格式）
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
		
		// 方法4: 专门针对合并格式 "Shipper / Seller" 等多语言版本
		const combinedPatterns = [
			/Shipper\s*\/\s*Seller/i,           // 英语（美区新格式）
			/Ships from\s*\/\s*Sold by/i,       // 英语（另一种格式）
			/Remitente\s*\/\s*Vendedor/i,       // 西班牙语（墨西哥站）
			/Envío\s*\/\s*Vendido/i,            // 西班牙语（另一种格式）
			/出荷元\s*\/\s*販売元/i,             // 日语
			/Expéditeur\s*\/\s*Vendeur/i,       // 法语
			/Expédié\s*\/\s*Vendu/i,            // 法语（另一种格式）
			/Versand\s*\/\s*Verkauf/i,          // 德语
			/Versender\s*\/\s*Verkäufer/i,      // 德语（另一种格式）
			/Spedito\s*\/\s*Venduto/i,          // 意大利语
			/Mittente\s*\/\s*Venditore/i,       // 意大利语（另一种格式）
			/Enviado\s*\/\s*Vendido/i,          // 葡萄牙语（巴西站）
			/Remetente\s*\/\s*Vendedor/i,       // 葡萄牙语（另一种格式）
		];
		
		const allElements = Array.from(document.querySelectorAll('*'));
		for (const element of allElements) {
			const text = element.textContent;
			if (text && combinedPatterns.some(pattern => pattern.test(text))) {
				// 查找包含 "Amazon" 的相邻元素
				const parent = element.parentElement;
				if (parent) {
					const siblings = Array.from(parent.children);
					const currentIndex = siblings.indexOf(element);
					
					// 检查后续兄弟元素
					for (let i = currentIndex + 1; i < siblings.length; i++) {
						const sibling = siblings[i];
						const siblingText = sibling.textContent.trim();
						if (siblingText && siblingText.includes('Amazon')) {
							return siblingText;
						}
					}
					
					// 检查父元素的后续兄弟元素
					const parentSiblings = Array.from(parent.parentElement?.children || []);
					const parentIndex = parentSiblings.indexOf(parent);
					for (let i = parentIndex + 1; i < parentSiblings.length; i++) {
						const parentSibling = parentSiblings[i];
						const siblingText = parentSibling.textContent.trim();
						if (siblingText && siblingText.includes('Amazon')) {
							return siblingText;
						}
					}
				}
			}
		}
		
		// 方法5: 通用查找 - 在 tabular-buybox 区域查找
		const tabularBuybox = document.querySelector('#tabular-buybox, #tabular-buybox-container');
		if (tabularBuybox) {
			const rows = tabularBuybox.querySelectorAll('tr, .tabular-buybox-text');
			for (const row of rows) {
				const rowText = row.textContent || '';
				// 检查是否包含 Shipper/Seller 或 Ships from 相关文本
				if (/Shipper|Ships from|Remitente/i.test(rowText)) {
					// 查找包含 Amazon 的文本
					const amazonMatch = rowText.match(/Amazon[^,\n]*/i);
					if (amazonMatch) {
						return amazonMatch[0].trim();
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
