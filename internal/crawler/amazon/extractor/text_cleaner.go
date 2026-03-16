// Package extractor 提供Amazon文本清理功能
package extractor

import (
	"regexp"
	"strings"
)

// TextCleaner 文本清理器
type TextCleaner struct{}

// NewTextCleaner 创建文本清理器
func NewTextCleaner() *TextCleaner {
	return &TextCleaner{}
}

// CleanDescriptionText 清理描述文本
func (c *TextCleaner) CleanDescriptionText(text string) string {
	if text == "" {
		return ""
	}

	// 首先移除所有script和style标签及其内容
	text = regexp.MustCompile(`(?s)<script[^>]*>.*?</script>`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?s)<style[^>]*>.*?</style>`).ReplaceAllString(text, "")

	// 移除HTML标签
	text = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(text, "")

	// 移除CSS样式块（包括内联样式）
	text = c.removeCSSContent(text)

	// 移除JavaScript代码模式
	text = c.removeJavaScriptContent(text)

	// 移除价格和购买相关信息
	text = c.removePriceAndPurchaseInfo(text)

	// 移除评论相关内容
	text = c.removeReviewContent(text)

	// 移除Amazon特定的无用内容
	text = c.removeAmazonSpecificContent(text)

	// 移除特殊的Amazon代码和标识符
	text = c.removeAmazonCodes(text)

	// 移除多余的空白字符
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// 移除特殊Unicode字符
	text = c.removeSpecialUnicodeChars(text)

	// 移除HTML实体
	text = c.removeHTMLEntities(text)

	// 移除常见的无用前缀
	text = c.removeUselessPrefixes(text)

	// 最终清理
	text = strings.TrimSpace(text)

	// 如果文本太短，返回空字符串
	if len(text) < 10 {
		return ""
	}

	// 检查是否只包含符号和数字
	if !c.hasEnoughLetters(text) {
		return ""
	}

	return text
}

// JoinDescriptions 连接描述列表
func (c *TextCleaner) JoinDescriptions(descriptions []string) string {
	return strings.Join(descriptions, " ")
}

// removeCSSContent 移除CSS内容
func (c *TextCleaner) removeCSSContent(text string) string {
	text = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?s)\{[^}]*:[^}]*\}`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`[.#][a-zA-Z][a-zA-Z0-9_-]*\s*\{[^}]*\}`).ReplaceAllString(text, "")
	return text
}

// removeJavaScriptContent 移除JavaScript内容
func (c *TextCleaner) removeJavaScriptContent(text string) string {
	text = regexp.MustCompile(`(?s)function\s*\([^)]*\)\s*\{.*?\}`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?s)\(function\([^)]*\)\s*\{.*?\}\)`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`var\s+\w+\s*=.*?;`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`P\.(when|_namespace|execute|declarative)\([^)]*\)`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(window|document)\.\w+[^;]*;`).ReplaceAllString(text, "")
	return text
}

// removePriceAndPurchaseInfo 移除价格和购买信息
func (c *TextCleaner) removePriceAndPurchaseInfo(text string) string {
	text = regexp.MustCompile(`\$\s*[0-9]+\.[0-9]+`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?i)(with\s+)?[0-9]+\s+percent\s+savings`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?i)(In\s+Stock|Out\s+of\s+Stock|Add\s+to\s+Cart|Buy\s+now)`).ReplaceAllString(text, "")
	return text
}

// removeReviewContent 移除评论内容
func (c *TextCleaner) removeReviewContent(text string) string {
	text = regexp.MustCompile(`(?i)Reviewed\s+in\s+(the\s+)?\w+\s+on\s+[A-Za-z]+\s+[0-9]+,\s+[0-9]+`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?i)(Verified\s+Purchase|Helpful\s+Report|Read\s+more)`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`[0-9.]+\s+out\s+of\s+5\s+stars`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`[0-9]+\s+star[0-9]*\s+star`).ReplaceAllString(text, "")
	return text
}

// removeAmazonSpecificContent 移除Amazon特定内容
func (c *TextCleaner) removeAmazonSpecificContent(text string) string {
	text = regexp.MustCompile(`(?i)(Clothing,\s+Shoes\s+&\s+Jewelry|Best\s+Sellers\s+Rank:|Date\s+First\s+Available|ASIN\s*:)`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?i)(See\s+Top\s+100\s+in|Visit\s+the\s+\w+\s+Store)`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`#[0-9,]+\s+in\s+[A-Za-z\s&]+`).ReplaceAllString(text, "")
	return text
}

// removeAmazonCodes 移除Amazon代码
func (c *TextCleaner) removeAmazonCodes(text string) string {
	text = regexp.MustCompile(`(TraeAI|premium-module|comparison-table|aplus-v2)-[0-9a-zA-Z-]+`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`\[[0-9]+:[0-9]+\]`).ReplaceAllString(text, "")
	return text
}

// removeSpecialUnicodeChars 移除特殊Unicode字符
func (c *TextCleaner) removeSpecialUnicodeChars(text string) string {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = strings.ReplaceAll(text, "\u200b", "")
	text = strings.ReplaceAll(text, "\u2028", " ")
	text = strings.ReplaceAll(text, "\u2029", " ")
	return text
}

// removeHTMLEntities 移除HTML实体
func (c *TextCleaner) removeHTMLEntities(text string) string {
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
	return text
}

// removeUselessPrefixes 移除无用前缀
func (c *TextCleaner) removeUselessPrefixes(text string) string {
	prefixes := []string{
		"•", "●", "◦", "▪", "▫", "‣", "⁃",
		"Make sure this fits by entering your model number.",
		"This fits your .",
	}

	for _, prefix := range prefixes {
		text = strings.TrimPrefix(text, prefix)
		text = strings.TrimSpace(text)
	}
	return text
}

// hasEnoughLetters 检查是否有足够的字母
func (c *TextCleaner) hasEnoughLetters(text string) bool {
	letterCount := 0
	for _, char := range text {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			letterCount++
		}
	}
	// 如果字母少于5个，认为不是有效文本
	return letterCount >= 5
}
