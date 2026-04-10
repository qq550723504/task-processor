// Package extractor 提供Amazon原价提取功能
package extractor

import (
	"regexp"
	"strings"
	"task-processor/internal/model"

	"github.com/playwright-community/playwright-go"
)

const (
	maxListPriceCandidatesPerSelector = 6
	maxListPriceParentDepth           = 4
)

// ListPriceExtractor 原价提取器
type ListPriceExtractor struct {
	marketplace string
	parser      *PriceParser
}

var listPriceSignalSelectors = []string{
	"#corePriceDisplay_desktop_feature_div .basisPrice",
	"#corePriceDisplay_desktop_feature_div .a-price.a-text-price",
	"#corePriceDisplay_desktop_feature_div #priceblock_listprice",
	"#corePriceDisplay_desktop_feature_div .a-text-strike .a-offscreen",
	"#corePrice_feature_div .basisPrice",
	"#corePrice_feature_div .a-price.a-text-price",
	"#apex_desktop .basisPrice",
	"#apex_desktop .a-price.a-text-price",
	"#apex_desktop #priceblock_listprice",
	"#apex_desktop .a-text-strike .a-offscreen",
}

var listPriceSignalContainers = []string{
	"#corePriceDisplay_desktop_feature_div",
	"#corePrice_feature_div",
	"#apex_desktop",
}

// NewListPriceExtractor 创建原价提取器
func NewListPriceExtractor(marketplace string) *ListPriceExtractor {
	return &ListPriceExtractor{
		marketplace: marketplace,
		parser:      NewPriceParser(marketplace),
	}
}

// ShouldExtract 判断当前页面是否值得执行原价提取。
func (l *ListPriceExtractor) ShouldExtract(page playwright.Page, product *model.Product) bool {
	if page == nil || product == nil || product.FinalPrice <= 0 {
		return false
	}

	for _, selector := range listPriceSignalSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			return true
		}
	}

	for _, selector := range listPriceSignalContainers {
		container, err := page.QuerySelector(selector)
		if err != nil || container == nil {
			continue
		}
		text, err := container.TextContent()
		if err != nil {
			continue
		}
		if containsListPriceMarker(text) {
			return true
		}
	}

	return false
}

// ExtractListPrice 提取原价（list price）
func (l *ListPriceExtractor) ExtractListPrice(page playwright.Page, product *model.Product) {
	if !l.ShouldExtract(page, product) {
		return
	}

	// 优先检查包含明确原价标识的选择器，限制在主产品区域
	prioritySelectors := []string{
		"#corePriceDisplay_desktop_feature_div span.a-size-small.aok-offscreen",
		"#corePrice_feature_div span.a-size-small.aok-offscreen",
		"#apex_desktop span.a-size-small.aok-offscreen", // 限制在主产品区域（含 "Was:" / "Typical price:" 等标识）
	}

	// 首先尝试优先选择器，这些通常包含明确的原价标识
	for _, selector := range prioritySelectors {
		if l.extractFromSelector(page, selector, product, true) {
			return
		}
	}

	// 如果优先选择器没有找到，尝试其他选择器，但需要更严格的验证
	fallbackSelectors := []string{
		"#corePriceDisplay_desktop_feature_div .a-price.a-text-price .a-offscreen",
		"#corePriceDisplay_desktop_feature_div #priceblock_listprice",
		"#corePriceDisplay_desktop_feature_div .a-text-strike .a-offscreen",
		"#corePriceDisplay_desktop_feature_div .basisPrice .a-offscreen",
		"#corePriceDisplay_desktop_feature_div .basisPrice",
		"#corePrice_feature_div .a-price.a-text-price .a-offscreen",
		"#corePrice_feature_div #priceblock_listprice",
		"#corePrice_feature_div .a-text-strike .a-offscreen",
		"#corePrice_feature_div .basisPrice .a-offscreen",
		"#corePrice_feature_div .basisPrice",
		// 美国站常见原价选择器（优先）
		"#apex_desktop .a-price.a-text-price .a-offscreen",
		"#apex_desktop #priceblock_listprice",
		"#apex_desktop .a-text-strike .a-offscreen",
		// 加拿大站 "Was:" 原价容器（兜底）
		"#apex_desktop .basisPrice .a-offscreen",
		"#apex_desktop .basisPrice",
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

	if count > maxListPriceCandidatesPerSelector {
		count = maxListPriceCandidatesPerSelector
	}

	for i := 0; i < count; i++ {
		element := locator.Nth(i)

		isVisible, err := element.IsVisible()
		if err != nil || !isVisible {
			continue
		}

		// 检查元素是否在赞助内容区域
		if l.isInSponsoredContent(element) {
			continue
		}

		if l.isInExcludedListPriceContainer(element) {
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

		if l.shouldSkipCandidateByText(text, product.FinalPrice) {
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

func (l *ListPriceExtractor) shouldSkipCandidateByText(text string, finalPrice float64) bool {
	if strings.TrimSpace(text) == "" {
		return true
	}

	if finalPrice <= 0 {
		return false
	}

	parsed := l.parser.ParsePrice(text)
	return parsed > 0 && parsed == finalPrice
}

// validatePriorityPrice 验证优先选择器中的价格
func (l *ListPriceExtractor) validatePriorityPrice(text string) (bool, string) {
	for _, marker := range listPriceMarkers {
		if strings.Contains(text, marker) {
			parts := strings.SplitN(text, marker, 2)
			if len(parts) > 1 {
				priceText := strings.TrimSpace(parts[1])
				priceText = strings.TrimPrefix(priceText, ":")
				priceText = strings.TrimSpace(priceText)
				if priceText != "" {
					return true, priceText
				}
			}
		}
	}

	return false, ""
}

// validateFallbackPrice 验证备用选择器中的价格
func (l *ListPriceExtractor) validateFallbackPrice(element playwright.Locator, text string) (bool, string) {
	// 文本本身包含原价标识，直接走优先逻辑
	if containsListPriceMarker(text) {
		return l.validatePriorityPrice(text)
	}

	// 必须包含货币符号才继续
	if !strings.Contains(text, "$") && !strings.Contains(text, "£") &&
		!strings.Contains(text, "€") && !strings.Contains(text, "¥") {
		return false, ""
	}

	// 删除线价格通常就是原价
	className, err := element.GetAttribute("class")
	if err == nil && strings.Contains(className, "a-text-strike") {
		return true, text
	}

	// 检查父元素是否包含原价相关上下文（含 "Was:"）
	if l.hasListPriceContext(element) {
		return true, text
	}

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
			if containsListPriceMarker(parentText) ||
				strings.Contains(parentText, "Originally:") {
				return true
			}
		}

		element = parent
	}

	return false
}

func (l *ListPriceExtractor) isInExcludedListPriceContainer(element playwright.Locator) bool {
	current := element
	for i := 0; i < maxListPriceParentDepth; i++ {
		className, err := current.GetAttribute("class")
		if err == nil && isExcludedListPriceContainerClass(className) {
			return true
		}

		parent := current.Locator("..")
		if parent == nil {
			break
		}
		current = parent
	}

	return false
}

func isExcludedListPriceContainerClass(className string) bool {
	className = strings.ToLower(strings.TrimSpace(className))
	if className == "" {
		return false
	}

	excludedMarkers := []string{
		"productdetailscontainer",
		"multi-brand-video",
		"comparison",
		"twister-plus-buying-options",
	}

	for _, marker := range excludedMarkers {
		if strings.Contains(className, marker) {
			return true
		}
	}

	return false
}

var listPriceMarkers = []string{"Typical price", "List Price", "Was:"}

func containsListPriceMarker(text string) bool {
	for _, marker := range listPriceMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// isInSponsoredContent 检查元素是否在赞助内容区域
func (l *ListPriceExtractor) isInSponsoredContent(element playwright.Locator) bool {
	// 检查元素及其父元素是否包含赞助内容标识
	current := element
	for i := 0; i < maxListPriceParentDepth; i++ { // 向上检查有限层级，避免大页面深链路扫描
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
	for i := 0; i < maxListPriceParentDepth; i++ { // 向上检查有限层级，避免多次深层属性扫描
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
