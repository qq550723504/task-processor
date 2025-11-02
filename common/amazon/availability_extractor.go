package amazon

import (
	"log"
	"strings"

	"github.com/playwright-community/playwright-go"
)

// AvailabilityExtractor 可用性提取器
type AvailabilityExtractor struct{}

func (e *AvailabilityExtractor) Extract(page playwright.Page, product *Product) error {
	availability, err := e.getAvailability(page)
	if err != nil {
		log.Printf("提取库存信息失败: %v", err)
		return err
	}
	product.Availability = availability

	// 设置IsAvailable字段
	product.IsAvailable = e.isAvailable(availability)

	return nil
}

// getAvailability 获取库存状态
func (e *AvailabilityExtractor) getAvailability(page playwright.Page) (string, error) {
	// Amazon 库存信息的常见选择器
	selectors := []string{
		"#availability span",
		"#availability .a-size-medium",
		"#availability-feature .a-size-medium",
		"#availability-feature span",
		"[data-feature-name='availability'] span",
		"#buybox-availability span",
		"#buybox .a-size-medium",
		".a-accordion-row[data-a-accordion-row-name='availability'] span",
		"#merchant-info",
		"#availability",
		"#availability-feature",
		".availability-feature span",
		".availability span",
		"#buybox-availability",
		"#buybox .availability",
		"[data-testid='availability-text']",
		".a-offscreen:has-text('availability')",
	}

	for _, selector := range selectors {
		element, err := page.QuerySelector(selector)
		if err != nil || element == nil {
			continue
		}

		text, err := element.TextContent()
		if err != nil {
			continue
		}

		text = strings.TrimSpace(text)
		if text != "" && e.isValidAvailabilityText(text) {
			log.Printf("找到库存信息: %s (选择器: %s)", text, selector)
			return e.normalizeAvailabilityText(text), nil
		}
	}

	// 如果没有找到库存信息，尝试从页面文本中查找
	pageText, err := page.TextContent("body")
	if err == nil {
		if availability := e.extractAvailabilityFromText(pageText); availability != "" {
			log.Printf("从页面文本中提取到库存信息: %s", availability)
			return availability, nil
		}
	}

	log.Printf("未找到库存信息")
	return "Unknown", nil
}

// isValidAvailabilityText 检查文本是否是有效的库存信息
func (e *AvailabilityExtractor) isValidAvailabilityText(text string) bool {
	text = strings.ToLower(text)

	// 包含库存相关关键词
	stockKeywords := []string{
		"in stock", "out of stock", "available", "unavailable",
		"ships", "delivery", "arrives", "sold out",
		"temporarily out", "currently unavailable",
		"only", "left in stock", "more on the way",
		"usually ships", "in stock soon",
	}

	for _, keyword := range stockKeywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}

	return false
}

// normalizeAvailabilityText 标准化库存文本
func (e *AvailabilityExtractor) normalizeAvailabilityText(text string) string {
	text = strings.TrimSpace(text)

	// 移除多余的空白字符
	text = strings.Join(strings.Fields(text), " ")

	// 限制长度
	if len(text) > 200 {
		text = text[:200] + "..."
	}

	return text
}

// extractAvailabilityFromText 从页面文本中提取库存信息
func (e *AvailabilityExtractor) extractAvailabilityFromText(pageText string) string {
	lines := strings.Split(pageText, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if e.isValidAvailabilityText(line) && len(line) < 200 {
			return e.normalizeAvailabilityText(line)
		}
	}

	return ""
}

// isAvailable 根据可用性文本判断产品是否可用
func (e *AvailabilityExtractor) isAvailable(availabilityText string) bool {
	lowerText := strings.ToLower(strings.TrimSpace(availabilityText))

	// 不可用的关键词
	unavailableKeywords := []string{
		"currently unavailable",
		"unavailable",
		"out of stock",
		"temporarily out of stock",
		"not available",
		"discontinued",
		"sold out",
	}

	for _, keyword := range unavailableKeywords {
		if strings.Contains(lowerText, keyword) {
			return false
		}
	}

	// 可用的关键词
	availableKeywords := []string{
		"in stock",
		"available",
		"ships",
		"delivery",
		"arrives",
		"left in stock",
		"more on the way",
		"usually ships",
		"in stock soon",
	}

	for _, keyword := range availableKeywords {
		if strings.Contains(lowerText, keyword) {
			return true
		}
	}

	// 如果没有明确的关键词，默认为不可用
	return false
}
