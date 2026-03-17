// Package extractor 提供Amazon原价提取功能
package extractor

import (
	"regexp"
	"strings"
	"task-processor/internal/model"

	"github.com/playwright-community/playwright-go"
)

// ListPriceExtractor 原价提取器
type ListPriceExtractor struct {
	marketplace string
	parser      *PriceParser
}

// NewListPriceExtractor 创建原价提取器
func NewListPriceExtractor(marketplace string) *ListPriceExtractor {
	return &ListPriceExtractor{
		marketplace: marketplace,
		parser:      NewPriceParser(marketplace),
	}
}

// ExtractListPrice 提取原价（list price）
func (l *ListPriceExtractor) ExtractListPrice(page playwright.Page, product *model.Product) {
	// 优先检查包含明确原价标识的选择器，限制在主产品区域
	prioritySelectors := []string{
		"#apex_desktop span.a-size-small.aok-offscreen", // 限制在主产品区域
		"#centerCol span.a-size-small.aok-offscreen",    // 中心列区域
		"#dp-container span.a-size-small.aok-offscreen", // 产品详情容器
	}

	// 首先尝试优先选择器，这些通常包含明确的原价标识
	for _, selector := range prioritySelectors {
		if l.extractFromSelector(page, selector, product, true) {
			return
		}
	}

	// 如果优先选择器没有找到，尝试其他选择器，但需要更严格的验证
	fallbackSelectors := []string{
		"#apex_desktop .a-price.a-text-price .a-offscreen",
		"#centerCol .a-price.a-text-price .a-offscreen",
		"#dp-container .a-price.a-text-price .a-offscreen",
		"#apex_desktop #priceblock_listprice",
		"#centerCol #priceblock_listprice",
		"#apex_desktop .a-text-strike .a-offscreen",
		"#centerCol .a-text-strike .a-offscreen",
		"#apex_desktop .basisPrice",
		"#centerCol .basisPrice",
	}

	for _, selector := range fallbackSelectors {
		if l.extractFromSelector(page, selector, product, false) {
			return
		}
	}
}

// extractFromSelector 从指定选择器提取原价
func (l *ListPriceExtractor) extractFromSelector(page playwright.Page, selector string, product *model.Product, isPriority bool) bool {
	locator := page.Locator(selector)
	count, err := locator.Count()
	if err != nil || count == 0 {
		return false
	}

	for i := 0; i < count; i++ {
		element := locator.Nth(i)

		// 检查元素是否在赞助内容区域
		if l.isInSponsoredContent(element) {
			continue
		}

		// 检查元素是否与当前产品ASIN相关
		if !l.isRelatedToCurrentProduct(element, product.Asin) {
			continue
		}

		text, err := element.TextContent()
		if err != nil {
			continue
		}

		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		var isValidPrice bool
		var extractedText string

		if isPriority {
			// 对于优先选择器，检查是否包含明确的原价标识
			isValidPrice, extractedText = l.validatePriorityPrice(text)
		} else {
			// 对于备用选择器，需要更严格的上下文验证
			isValidPrice, extractedText = l.validateFallbackPrice(element, text)
		}

		if isValidPrice && extractedText != "" {
			listPrice := l.parser.ParsePrice(extractedText)
			if listPrice > 0 && listPrice != product.FinalPrice && listPrice > product.FinalPrice {
				product.PricesBreakdown.ListPrice = &listPrice
				return true
			}
		}
	}

	return false
}

// validatePriorityPrice 验证优先选择器中的价格
func (l *ListPriceExtractor) validatePriorityPrice(text string) (bool, string) {
	// 检查是否包含明确的原价标识
	if strings.Contains(text, "Typical price") {
		// 提取 "Typical price: $XX.XX" 中的价格部分
		parts := strings.Split(text, "Typical price")
		if len(parts) > 1 {
			priceText := strings.TrimSpace(parts[1])
			// 移除可能的冒号
			priceText = strings.TrimPrefix(priceText, ":")
			priceText = strings.TrimSpace(priceText)
			return true, priceText
		}
	}

	if strings.Contains(text, "List Price") {
		// 提取 "List Price: $XX.XX" 中的价格部分
		parts := strings.Split(text, "List Price")
		if len(parts) > 1 {
			priceText := strings.TrimSpace(parts[1])
			// 移除可能的冒号
			priceText = strings.TrimPrefix(priceText, ":")
			priceText = strings.TrimSpace(priceText)
			return true, priceText
		}
	}

	return false, ""
}

// validateFallbackPrice 验证备用选择器中的价格
func (l *ListPriceExtractor) validateFallbackPrice(element playwright.Locator, text string) (bool, string) {
	if !strings.Contains(text, "$") {
		return false, ""
	}

	// 如果文本本身包含原价标识，直接返回
	if strings.Contains(text, "Typical price") || strings.Contains(text, "List Price") {
		return l.validatePriorityPrice(text)
	}

	// 检查是否是删除线价格（通常表示原价）
	className, err := element.GetAttribute("class")
	if err == nil && strings.Contains(className, "a-text-strike") {
		return true, text
	}

	// 检查父元素是否包含原价相关的上下文
	if l.hasListPriceContext(element) {
		return true, text
	}

	// 如果没有找到明确的原价上下文，返回false
	return false, ""
}

// hasListPriceContext 检查元素是否在有效的原价上下文中
func (l *ListPriceExtractor) hasListPriceContext(element playwright.Locator) bool {
	// 检查父元素的文本内容
	for i := 0; i < 3; i++ { // 向上检查3层父元素
		parent := element.Locator("..")
		if parent == nil {
			break
		}

		parentText, err := parent.TextContent()
		if err == nil {
			parentText = strings.TrimSpace(parentText)
			if strings.Contains(parentText, "Typical price") ||
				strings.Contains(parentText, "List Price") ||
				strings.Contains(parentText, "Was:") ||
				strings.Contains(parentText, "Originally:") {
				return true
			}
		}

		element = parent
	}

	return false
}

// isInSponsoredContent 检查元素是否在赞助内容区域
func (l *ListPriceExtractor) isInSponsoredContent(element playwright.Locator) bool {
	// 检查元素及其父元素是否包含赞助内容标识
	current := element
	for i := 0; i < 5; i++ { // 向上检查5层父元素
		// 检查class属性
		className, err := current.GetAttribute("class")
		if err == nil && className != "" {
			if strings.Contains(className, "sponsored") ||
				strings.Contains(className, "sp-") ||
				strings.Contains(className, "ads-") ||
				strings.Contains(className, "adplacements") {
				return true
			}
		}

		// 检查data属性
		dataComponent, err := current.GetAttribute("data-component-type")
		if err == nil && dataComponent != "" {
			if strings.Contains(dataComponent, "sp-") ||
				strings.Contains(dataComponent, "sponsored") {
				return true
			}
		}

		// 检查id属性
		id, err := current.GetAttribute("id")
		if err == nil && id != "" {
			if strings.Contains(id, "sponsored") ||
				strings.Contains(id, "sp-") ||
				strings.Contains(id, "ads") {
				return true
			}
		}

		// 检查文本内容是否包含赞助标识
		text, err := current.TextContent()
		if err == nil {
			if strings.Contains(text, "Sponsored") ||
				strings.Contains(text, "Ad") ||
				strings.Contains(text, "Customers who viewed this item also viewed") ||
				strings.Contains(text, "Frequently bought together") ||
				strings.Contains(text, "Compare with similar items") {
				return true
			}
		}

		// 向上移动到父元素
		parent := current.Locator("..")
		if parent == nil {
			break
		}
		current = parent
	}

	return false
}

// isRelatedToCurrentProduct 检查元素是否与当前产品ASIN相关
func (l *ListPriceExtractor) isRelatedToCurrentProduct(element playwright.Locator, currentASIN string) bool {
	if currentASIN == "" {
		return true // 如果没有ASIN信息，默认认为相关
	}

	// 检查元素及其父元素是否包含当前产品的ASIN
	current := element
	for i := 0; i < 8; i++ { // 向上检查8层父元素
		// 检查各种可能包含ASIN的属性
		attributes := []string{"data-asin", "data-parent-asin", "data-original-asin", "id", "class"}

		for _, attr := range attributes {
			value, err := current.GetAttribute(attr)
			if err == nil && value != "" {
				if strings.Contains(value, currentASIN) {
					return true
				}
				// 检查是否包含其他ASIN（如果包含其他ASIN但不是当前ASIN，可能是其他产品）
				if l.containsOtherASIN(value, currentASIN) {
					return false
				}
			}
		}

		// 检查href属性中的ASIN
		href, err := current.GetAttribute("href")
		if err == nil && href != "" {
			if strings.Contains(href, "/dp/"+currentASIN) ||
				strings.Contains(href, "/gp/product/"+currentASIN) {
				return true
			}
			// 检查是否指向其他产品
			if l.containsOtherProductURL(href, currentASIN) {
				return false
			}
		}

		// 向上移动到父元素
		parent := current.Locator("..")
		if parent == nil {
			break
		}
		current = parent
	}

	return true // 如果没有找到明确的ASIN信息，默认认为相关
}

// containsOtherASIN 检查字符串是否包含其他ASIN
func (l *ListPriceExtractor) containsOtherASIN(value, currentASIN string) bool {
	// ASIN格式通常是B开头的10位字符
	asinPattern := regexp.MustCompile(`B[0-9A-Z]{9}`)
	matches := asinPattern.FindAllString(value, -1)

	for _, match := range matches {
		if match != currentASIN {
			return true // 找到其他ASIN
		}
	}
	return false
}

// containsOtherProductURL 检查URL是否指向其他产品
func (l *ListPriceExtractor) containsOtherProductURL(url, currentASIN string) bool {
	// 检查URL中的产品路径
	dpPattern := regexp.MustCompile(`/dp/([B0-9A-Z]{10})`)
	gpPattern := regexp.MustCompile(`/gp/product/([B0-9A-Z]{10})`)

	matches := dpPattern.FindStringSubmatch(url)
	if len(matches) > 1 && matches[1] != currentASIN {
		return true
	}

	matches = gpPattern.FindStringSubmatch(url)
	if len(matches) > 1 && matches[1] != currentASIN {
		return true
	}

	return false
}

