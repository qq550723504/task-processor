package extractor

import (
	"strings"
	"task-processor/internal/model"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// AvailabilityExtractor 可用性提取器
type AvailabilityExtractor struct{}

func (e *AvailabilityExtractor) Extract(page playwright.Page, product *model.Product) error {
	availability, err := e.getAvailability(page)
	if err != nil {
		logrus.Errorf("提取库存信息失败: %v", err)
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
			return e.normalizeAvailabilityText(text), nil
		}
	}

	// 如果没有找到库存信息，尝试从页面文本中查找
	pageText, err := page.TextContent("body")
	if err == nil {
		if availability := e.extractAvailabilityFromText(pageText); availability != "" {
			logrus.WithFields(logrus.Fields{
				"text": availability,
			}).Info("✅ 从页面文本中提取到库存信息")
			return availability, nil
		}
	}

	logrus.Warn("⚠️ 未找到库存信息，返回 Unknown")
	return "Unknown", nil
}

// isValidAvailabilityText 检查文本是否是有效的库存信息（多语言支持）
func (e *AvailabilityExtractor) isValidAvailabilityText(text string) bool {
	text = strings.ToLower(text)

	// 包含库存相关关键词（多语言）
	stockKeywords := []string{
		// 英语
		"in stock", "out of stock", "available", "unavailable",
		"ships", "delivery", "arrives", "sold out",
		"temporarily out", "currently unavailable",
		"only", "left in stock", "more on the way",
		"usually ships", "in stock soon",
		// 西班牙语
		"disponible", "no disponible", "agotado",
		"envío", "entrega", "llega",
		"en stock", "sin stock", "temporalmente agotado",
		"quedan", "en camino",
		// 日语
		"在庫あり", "在庫切れ", "一時的に在庫切れ",
		"配送", "お届け", "発送",
		// 德语
		"auf lager", "nicht auf lager", "ausverkauft",
		"versand", "lieferung",
		// 法语
		"en stock", "rupture de stock", "épuisé",
		"expédition", "livraison",
		// 意大利语
		"disponibile", "non disponibile", "esaurito",
		"spedizione", "consegna",
		// 阿拉伯语
		"متوفر", "غير متوفر", "نفذت الكمية",
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

// isAvailable 根据可用性文本判断产品是否可用（多语言支持）
func (e *AvailabilityExtractor) isAvailable(availabilityText string) bool {
	lowerText := strings.ToLower(strings.TrimSpace(availabilityText))

	// 不可用的关键词（多语言）
	unavailableKeywords := []string{
		// 英语
		"currently unavailable",
		"unavailable",
		"out of stock",
		"temporarily out of stock",
		"not available",
		"discontinued",
		"sold out",
		// 西班牙语
		"no disponible",
		"agotado",
		"sin stock",
		"temporalmente agotado",
		"actualmente no disponible",
		// 日语
		"在庫切れ",
		"一時的に在庫切れ",
		"取り扱い終了",
		"現在お取り扱いできません",
		// 德语
		"nicht auf lager",
		"ausverkauft",
		"derzeit nicht verfügbar",
		// 法语
		"rupture de stock",
		"épuisé",
		"actuellement indisponible",
		// 意大利语
		"non disponibile",
		"esaurito",
		"attualmente non disponibile",
		// 阿拉伯语
		"غير متوفر",
		"نفذت الكمية",
	}

	for _, keyword := range unavailableKeywords {
		if strings.Contains(lowerText, keyword) {
			logrus.WithFields(logrus.Fields{
				"keyword": keyword,
				"result":  "不可用",
			}).Info("❌ 匹配到不可用关键词")
			return false
		}
	}

	// 可用的关键词（多语言）
	availableKeywords := []string{
		// 英语
		"in stock",
		"available",
		"ships",
		"delivery",
		"arrives",
		"left in stock",
		"more on the way",
		"usually ships",
		"in stock soon",
		// 西班牙语
		"disponible",
		"en stock",
		"envío",
		"entrega",
		"llega",
		"quedan",
		"en camino",
		// 日语
		"在庫あり",
		"配送",
		"お届け",
		"発送",
		// 德语
		"auf lager",
		"versand",
		"lieferung",
		"verfügbar",
		// 法语
		"en stock",
		"disponible",
		"expédition",
		"livraison",
		// 意大利语
		"disponibile",
		"spedizione",
		"consegna",
		// 阿拉伯语
		"متوفر",
		"التوصيل",
	}

	for _, keyword := range availableKeywords {
		if strings.Contains(lowerText, keyword) {
			return true
		}
	}

	// 如果没有明确的关键词，默认为不可用
	logrus.WithFields(logrus.Fields{
		"text": availabilityText,
	}).Warn("⚠️ 未匹配到任何关键词，默认判断为不可用")
	return false
}
