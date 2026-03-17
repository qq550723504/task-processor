package extractor

import (
	"regexp"
	"strings"
	"task-processor/internal/model"

	"github.com/playwright-community/playwright-go"
)

// DeliveryExtractor 配送信息提取器
type DeliveryExtractor struct{}

// Extract 提取配送信息
func (e *DeliveryExtractor) Extract(page playwright.Page, product *model.Product) error {
	// 使用 JavaScript 提取配送信息
	result, err := page.Evaluate(`() => {
		const deliveryTexts = [];
		
		// 主要配送信息选择器
		const selectors = [
			'#deliveryBlockMessage',
			'#mir-layout-DELIVERY_BLOCK',
			'#delivery-message',
			'#ddmDeliveryMessage',
			'#contextualIngressPtLabel',
			'#primeDeliveryMessage',
			'span[data-csa-c-delivery-price]',
			'div[data-feature-name="delivery"]',
			'#buybox-delivery-message'
		];
		
		// 从主要选择器提取
		for (const selector of selectors) {
			const element = document.querySelector(selector);
			if (element) {
				const text = element.textContent.trim();
				if (text && text.length < 300) {
					deliveryTexts.push(text);
				}
			}
		}
		
		// 配送关键词（多语言）
		const keywords = [
			'FREE delivery', 'FREE Delivery', 'Fastest delivery',
			'Get it by', 'Arrives', 'Or fastest',
			'無料配送', '最速', 'お届け予定日',
			'Envío GRATIS', 'Entrega GRATIS',
			'Livraison GRATUITE', 'Lieferung GRATIS',
			'Consegna GRATUITA'
		];
		
		// 查找包含关键词的元素
		const allElements = document.querySelectorAll('div, span');
		for (const element of allElements) {
			const text = element.textContent.trim();
			
			// 检查是否包含关键词且长度合理
			if (text.length > 10 && text.length < 200) {
				for (const keyword of keywords) {
					if (text.includes(keyword)) {
						// 排除包含地址、JSON 等无用信息
						if (!text.includes('Deliver to') && 
						    !text.includes('届け先') &&
						    !text.includes('Enviar a') &&
						    !text.includes('{') && 
						    !text.includes('merchantId')) {
							deliveryTexts.push(text);
							break;
						}
					}
				}
			}
		}
		
		return deliveryTexts;
	}`)

	if err != nil {
		return err
	}

	// 转换结果
	var deliveryInfo []string
	if resultSlice, ok := result.([]any); ok {
		for _, item := range resultSlice {
			if text, ok := item.(string); ok {
				deliveryInfo = append(deliveryInfo, text)
			}
		}
	}

	// 清理和去重
	product.Delivery = cleanAndDeduplicateDelivery(deliveryInfo)

	return nil
}

// cleanAndDeduplicateDelivery 清理和去重配送信息
func cleanAndDeduplicateDelivery(items []string) []string {
	seen := make(map[string]bool)
	var result []string

	// 过滤规则
	excludePatterns := []*regexp.Regexp{
		regexp.MustCompile(`Deliver to`),
		regexp.MustCompile(`届け先`),
		regexp.MustCompile(`Enviar a`),
		regexp.MustCompile(`\{.*merchantId.*\}`),
		regexp.MustCompile(`^\d{3}-\d{4}$`), // 邮编格式
	}

	for _, item := range items {
		// 清理多余空白字符
		cleaned := strings.Join(strings.Fields(item), " ")

		// 跳过空字符串
		if cleaned == "" {
			continue
		}

		// 检查是否匹配排除规则
		shouldExclude := false
		for _, pattern := range excludePatterns {
			if pattern.MatchString(cleaned) {
				shouldExclude = true
				break
			}
		}

		if shouldExclude {
			continue
		}

		// 去重：检查是否已存在或是其他项的子串
		isDuplicate := false
		for _, existing := range result {
			// 如果当前项是已存在项的子串，跳过
			if strings.Contains(existing, cleaned) {
				isDuplicate = true
				break
			}
			// 如果已存在项是当前项的子串，替换
			if strings.Contains(cleaned, existing) {
				// 移除旧的较短项
				for i, r := range result {
					if r == existing {
						result = append(result[:i], result[i+1:]...)
						break
					}
				}
				break
			}
		}

		if !isDuplicate && !seen[cleaned] {
			seen[cleaned] = true
			result = append(result, cleaned)
		}
	}

	return result
}
