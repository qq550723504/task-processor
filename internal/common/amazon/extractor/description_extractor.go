package extractor

import (
	"regexp"
	"strings"
	"task-processor/internal/common/amazon/model"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// DescriptionExtractor 描述提取器
type DescriptionExtractor struct{}

func (e *DescriptionExtractor) Extract(page playwright.Page, product *model.Product) error {
	// 提取description
	description := e.extractDescription(page)
	if description != "" {
		product.Description = description
	}

	// 提取product_description
	productDescription := e.extractProductDescription(page)
	if productDescription != "" {
		product.ProductDescription = []model.Description{
			{
				Text: productDescription,
				Type: "text",
			},
		}
	}

	// 提取features（如果FeaturesExtractor没有提取到）
	if len(product.Features) == 0 {
		features := e.extractFeatures(page)
		if len(features) > 0 {
			product.Features = features
		}
	}

	return nil
}

// extractDescription 提取商品描述
func (e *DescriptionExtractor) extractDescription(page playwright.Page) string {
	selectors := []string{
		"#feature-bullets ul li span.a-list-item",
		"#feature-bullets .a-list-item",
		"[data-feature-name='featurebullets'] ul li span",
		"#productDescription p",
	}

	for _, selector := range selectors {
		elements, err := page.QuerySelectorAll(selector)
		if err != nil {
			continue
		}

		var descriptions []string
		for _, element := range elements {
			// 使用InnerText而不是TextContent，避免获取隐藏元素
			text, err := element.InnerText()
			if err != nil {
				continue
			}

			cleaned := e.cleanDescriptionText(text)
			if e.isValidDescription(cleaned) && len(cleaned) > 10 {
				descriptions = append(descriptions, cleaned)
			}
		}

		if len(descriptions) > 0 {
			result := strings.Join(descriptions, " ")
			if len(result) > 50 {
				return result
			}
		}
	}

	return ""
}

// extractProductDescription 提取产品详细描述
func (e *DescriptionExtractor) extractProductDescription(page playwright.Page) string {
	logrus.Info("开始提取产品详细描述")

	// 第一组：精确的产品描述选择器 - 只提取特定元素的innerText
	primarySelectors := []string{
		"#productDescription p",
		"#productDescription div.a-section",
		"[data-feature-name='productDescription'] p",
		"#product-description-section p",
		".product-description p",
	}

	for _, selector := range primarySelectors {
		elements, err := page.QuerySelectorAll(selector)
		if err != nil {
			continue
		}

		var descriptions []string
		for _, element := range elements {
			// 使用innerText而不是textContent，避免获取隐藏元素
			text, err := element.InnerText()
			if err != nil {
				continue
			}

			cleaned := e.cleanDescriptionText(text)
			if e.isValidProductDescription(cleaned) && len(cleaned) > 20 {
				descriptions = append(descriptions, cleaned)
			}
		}

		if len(descriptions) > 0 {
			result := strings.Join(descriptions, " ")
			if len(result) > 50 {
				return result
			}
		}
	}

	// 第二组：从 A+ Content 中提取
	aplusSelectors := []string{
		"#aplus .aplus-module p",
		"#aplus_feature_div .celwidget p",
		".aplus-v2 .aplus-module-content p",
	}

	for _, selector := range aplusSelectors {
		elements, err := page.QuerySelectorAll(selector)
		if err != nil {
			continue
		}

		var descriptions []string
		for _, element := range elements {
			text, err := element.InnerText()
			if err != nil {
				continue
			}

			cleaned := e.cleanDescriptionText(text)
			if e.isValidProductDescription(cleaned) && len(cleaned) > 30 {
				descriptions = append(descriptions, cleaned)
			}
		}

		if len(descriptions) > 0 {
			result := strings.Join(descriptions, " ")
			if len(result) > 50 {
				return result
			}
		}
	}

	// 第三组：从 feature bullets 中提取（作为最后的备选）
	if description := e.extractDescription(page); description != "" && len(description) > 100 {
		return description
	}

	logrus.Info("未能提取到产品详细描述")
	return ""
}

// extractFeatures 提取产品特性列表
func (e *DescriptionExtractor) extractFeatures(page playwright.Page) []string {
	var features []string

	// 首先尝试从"About this item"部分提取特性
	aboutSelectors := []string{
		"#feature-bullets ul li",
		"[data-feature-name='aboutThisItem'] ul li",
		"div[data-feature-name='featurebullets'] ul li",
		"#feature-bullets ul li span",
		"[data-feature-name='aboutThisItem'] ul li span",
		"div[data-feature-name='featurebullets'] ul li span",
		"#productDetails_feature_div ul li",
		".a-expander-content span",
		"[data-feature-name='aboutThisItem'] span",
	}

	for _, selector := range aboutSelectors {
		elements, err := page.QuerySelectorAll(selector)
		if err != nil {
			continue
		}

		for _, element := range elements {
			text, err := element.TextContent()
			if err != nil {
				continue
			}

			cleaned := e.cleanDescriptionText(text)
			// 检查是否是"About this item"相关的特性描述
			if e.isAboutItemFeature(cleaned) {
				features = append(features, cleaned)
			}
		}

		if len(features) > 0 {
			break
		}
	}

	return e.deduplicateFeatures(features)
}

// cleanDescriptionText 清理描述文本
func (e *DescriptionExtractor) cleanDescriptionText(text string) string {
	if text == "" {
		return ""
	}

	// 首先移除所有script和style标签及其内容
	text = regexp.MustCompile(`(?s)<script[^>]*>.*?</script>`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?s)<style[^>]*>.*?</style>`).ReplaceAllString(text, "")

	// 移除HTML标签
	text = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(text, "")

	// 移除CSS样式块（包括内联样式）
	text = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?s)\{[^}]*:[^}]*\}`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`[.#][a-zA-Z][a-zA-Z0-9_-]*\s*\{[^}]*\}`).ReplaceAllString(text, "")

	// 移除JavaScript代码模式
	text = regexp.MustCompile(`(?s)function\s*\([^)]*\)\s*\{.*?\}`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?s)\(function\([^)]*\)\s*\{.*?\}\)`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`var\s+\w+\s*=.*?;`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`P\.(when|_namespace|execute|declarative)\([^)]*\)`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(window|document)\.\w+[^;]*;`).ReplaceAllString(text, "")

	// 移除价格和购买相关信息
	text = regexp.MustCompile(`\$\s*[0-9]+\.[0-9]+`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?i)(with\s+)?[0-9]+\s+percent\s+savings`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?i)(In\s+Stock|Out\s+of\s+Stock|Add\s+to\s+Cart|Buy\s+now)`).ReplaceAllString(text, "")

	// 移除评论相关内容
	text = regexp.MustCompile(`(?i)Reviewed\s+in\s+(the\s+)?\w+\s+on\s+[A-Za-z]+\s+[0-9]+,\s+[0-9]+`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?i)(Verified\s+Purchase|Helpful\s+Report|Read\s+more)`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`[0-9.]+\s+out\s+of\s+5\s+stars`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`[0-9]+\s+star[0-9]*\s+star`).ReplaceAllString(text, "")

	// 移除Amazon特定的无用内容
	text = regexp.MustCompile(`(?i)(Clothing,\s+Shoes\s+&\s+Jewelry|Best\s+Sellers\s+Rank:|Date\s+First\s+Available|ASIN\s*:)`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?i)(See\s+Top\s+100\s+in|Visit\s+the\s+\w+\s+Store)`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`#[0-9,]+\s+in\s+[A-Za-z\s&]+`).ReplaceAllString(text, "")

	// 移除特殊的Amazon代码和标识符
	text = regexp.MustCompile(`(TraeAI|premium-module|comparison-table|aplus-v2)-[0-9a-zA-Z-]+`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`\[[0-9]+:[0-9]+\]`).ReplaceAllString(text, "")

	// 移除多余的空白字符
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// 移除特殊Unicode字符
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = strings.ReplaceAll(text, "\u200b", "")
	text = strings.ReplaceAll(text, "\u2028", " ")
	text = strings.ReplaceAll(text, "\u2029", " ")

	// 移除HTML实体
	htmlEntities := map[string]string{
		"&amp;":  "&",
		"&lt;":   "<",
		"&gt;":   ">",
		"&quot;": "\"",
		"&apos;": "'",
		"&nbsp;": " ",
		"&#39;":  "'",
		"&#34;":  "\"",
		"&#38;":  "&",
		"&#60;":  "<",
		"&#62;":  ">",
	}

	for entity, replacement := range htmlEntities {
		text = strings.ReplaceAll(text, entity, replacement)
	}

	// 移除常见的无用前缀
	prefixes := []string{
		"•", "●", "◦", "▪", "▫", "‣", "⁃",
		"Make sure this fits by entering your model number.",
		"This fits your .",
	}

	for _, prefix := range prefixes {
		text = strings.TrimPrefix(text, prefix)
		text = strings.TrimSpace(text)
	}

	// 最终清理
	text = strings.TrimSpace(text)

	// 如果文本太短，返回空字符串
	if len(text) < 10 {
		return ""
	}

	// 检查是否只包含符号和数字
	letterCount := 0
	for _, char := range text {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			letterCount++
		}
	}

	// 如果字母少于5个，认为不是有效文本
	if letterCount < 5 {
		return ""
	}

	return text
}

// isValidDescription 验证描述是否有效
func (e *DescriptionExtractor) isValidDescription(text string) bool {
	if text == "" || len(text) < 10 {
		return false
	}

	// 过滤掉无意义的文本
	invalidPatterns := []string{
		"^\\s*$",
		"^[\\s\\p{P}]*$",
		"^(\\s*[•●◦▪▫‣⁃]\\s*)+$",
		"^\\s*see\\s+more\\s*$",
		"^\\s*read\\s+more\\s*$",
		"^\\s*show\\s+more\\s*$",
		"^\\s*click\\s+here\\s*$",
	}

	for _, pattern := range invalidPatterns {
		matched, _ := regexp.MatchString("(?i)"+pattern, text)
		if matched {
			return false
		}
	}

	return true
}

// isValidProductDescription 验证产品描述是否有效（比isValidDescription更严格）
func (e *DescriptionExtractor) isValidProductDescription(text string) bool {
	if text == "" || len(text) < 10 {
		return false
	}

	// 首先使用基本验证
	if !e.isValidDescription(text) {
		return false
	}

	// 过滤掉JavaScript和CSS相关内容
	invalidPatterns := []string{
		"function\\s*\\(",
		"var\\s+\\w+\\s*=",
		"window\\.",
		"document\\.",
		"\\$\\(",
		"jQuery",
		"\\.css\\(",
		"\\.addClass\\(",
		"\\.removeClass\\(",
		"addEventListener",
		"onclick\\s*=",
		"onload\\s*=",
		"setTimeout",
		"setInterval",
		"console\\.",
		"P\\._namespace",
		"guardFatal",
		"logShoppableMetrics",
		"\\{[^}]*color\\s*:",
		"\\{[^}]*background\\s*:",
		"\\{[^}]*font\\s*:",
		"\\{[^}]*margin\\s*:",
		"\\{[^}]*padding\\s*:",
		"@media\\s*\\(",
		"@import\\s+",
		"@keyframes\\s+",
		"\\.\\w+\\s*\\{",
		"#\\w+\\s*\\{",
		"rgb\\s*\\(",
		"rgba\\s*\\(",
		"#[0-9A-Fa-f]{3,6}",
		"\\d+px",
		"\\d+em",
		"\\d+rem",
		"calc\\s*\\(",
		"url\\s*\\(",
		"linear-gradient\\s*\\(",
		"var\\s*\\(",
	}

	for _, pattern := range invalidPatterns {
		matched, _ := regexp.MatchString("(?i)"+pattern, text)
		if matched {
			return false
		}
	}

	// 过滤掉其他无关内容
	otherInvalidPatterns := []string{
		"logShoppableMetrics",
		"premium-module",
		"comparison-table",
		"aplus-v2",
		"aplus-review",
		"padding-right",
		"Customer Reviews",
		"out of 5 stars",
		"\\$[0-9]+\\.[0-9]+",
		"Add to Cart",
		"P\\._namespace",
		"guardFatal",
		"P\\.when\\(",
		"\\.execute\\(",
		"init\\(\\)",
		"ready\\)",
		"TraeAI-",
		"\\[0:0\\]",
	}

	for _, pattern := range otherInvalidPatterns {
		matched, _ := regexp.MatchString("(?i)"+pattern, text)
		if matched {
			return false
		}
	}

	// 检查是否包含过多的特殊字符
	specialCharCount := 0
	for _, char := range text {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == ' ' || char == '.' ||
			char == ',' || char == ';' || char == ':' || char == '!' ||
			char == '?' || char == '-' || char == '_' || char == '(' ||
			char == ')' || char == '[' || char == ']' || char == '\'' ||
			char == '"' || char == '&' || char == '+' || char == '/' ||
			char == '\\' || char == '\n' || char == '\r' || char == '\t') {
			specialCharCount++
		}
	}

	// 如果特殊字符超过20%，认为不是有效的产品描述
	if float64(specialCharCount)/float64(len(text)) > 0.2 {
		return false
	}

	return true
}

// isAboutItemFeature 检查是否为"About this item"相关的特性描述
func (e *DescriptionExtractor) isAboutItemFeature(text string) bool {
	if len(text) < 10 || len(text) > 500 {
		return false
	}

	// 过滤掉一些无用的文本
	invalidPatterns := []string{
		"see more",
		"show more",
		"read more",
		"click here",
		"learn more",
		"view details",
		"customer reviews",
		"product description",
		"technical details",
		"additional information",
		"about this item",
		"›",
		"‹",
		"»",
		"«",
		"$",
		"price",
		"add to cart",
		"buy now",
		"quantity",
		"shipping",
		"return",
		"warranty",
		"free 30-day",
		"refund",
		"replacement",
		"transaction is secure",
		"recycled",
		"climate pledge",
		"certified",
		"supply chain",
		"independently verified",
		"contains at least",
		"percent",
		"by weight",
		"4 star",
		"5 star",
		"1 star",
		"2 star",
		"3 star",
	}

	lowerText := strings.ToLower(text)
	for _, pattern := range invalidPatterns {
		if strings.Contains(lowerText, pattern) {
			return false
		}
	}

	// 检查是否包含产品特性相关的关键词
	featureKeywords := []string{
		"design", "material", "quality", "durable", "comfortable",
		"adjustable", "waterproof", "lightweight", "perfect", "ideal",
		"suitable", "gift", "occasion", "style", "elegant", "classic",
		"modern", "vintage", "handmade", "crafted", "premium", "luxury",
		"service", "support", "guarantee", "satisfaction",
	}

	for _, keyword := range featureKeywords {
		if strings.Contains(lowerText, keyword) {
			return true
		}
	}

	// 如果文本较长且不包含明显的无效模式，也认为是有效的特性
	return len(text) > 20
}

// deduplicateFeatures 去重特性列表
func (e *DescriptionExtractor) deduplicateFeatures(features []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, feature := range features {
		normalized := strings.ToLower(strings.TrimSpace(feature))
		if !seen[normalized] && normalized != "" {
			seen[normalized] = true
			result = append(result, feature)
		}
	}

	return result
}
