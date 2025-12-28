// Package extractor 提供Amazon文本验证功能
package extractor

import (
	"regexp"
	"strings"
)

// TextValidator 文本验证器
type TextValidator struct{}

// NewTextValidator 创建文本验证器
func NewTextValidator() *TextValidator {
	return &TextValidator{}
}

// IsValidDescription 验证描述是否有效
func (v *TextValidator) IsValidDescription(text string) bool {
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

// IsValidProductDescription 验证产品描述是否有效（比IsValidDescription更严格）
func (v *TextValidator) IsValidProductDescription(text string) bool {
	if text == "" || len(text) < 10 {
		return false
	}

	// 首先使用基本验证
	if !v.IsValidDescription(text) {
		return false
	}

	// 过滤掉JavaScript和CSS相关内容
	if v.containsJavaScriptOrCSS(text) {
		return false
	}

	// 过滤掉其他无关内容
	if v.containsOtherInvalidContent(text) {
		return false
	}

	// 检查是否包含过多的特殊字符
	if v.hasTooManySpecialChars(text) {
		return false
	}

	return true
}

// IsAboutItemFeature 检查是否为"About this item"相关的特性描述
func (v *TextValidator) IsAboutItemFeature(text string) bool {
	if len(text) < 10 || len(text) > 500 {
		return false
	}

	// 过滤掉一些无用的文本
	if v.containsInvalidFeaturePatterns(text) {
		return false
	}

	// 检查是否包含产品特性相关的关键词
	if v.containsFeatureKeywords(text) {
		return true
	}

	// 如果文本较长且不包含明显的无效模式，也认为是有效的特性
	return len(text) > 20
}

// containsJavaScriptOrCSS 检查是否包含JavaScript或CSS内容
func (v *TextValidator) containsJavaScriptOrCSS(text string) bool {
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
			return true
		}
	}
	return false
}

// containsOtherInvalidContent 检查是否包含其他无效内容
func (v *TextValidator) containsOtherInvalidContent(text string) bool {
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
			return true
		}
	}
	return false
}

// hasTooManySpecialChars 检查是否包含过多特殊字符
func (v *TextValidator) hasTooManySpecialChars(text string) bool {
	specialCharCount := 0
	for _, char := range text {
		if !v.isNormalChar(char) {
			specialCharCount++
		}
	}

	// 如果特殊字符超过20%，认为不是有效的产品描述
	return float64(specialCharCount)/float64(len(text)) > 0.2
}

// isNormalChar 检查是否为正常字符
func (v *TextValidator) isNormalChar(char rune) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9') || char == ' ' || char == '.' ||
		char == ',' || char == ';' || char == ':' || char == '!' ||
		char == '?' || char == '-' || char == '_' || char == '(' ||
		char == ')' || char == '[' || char == ']' || char == '\'' ||
		char == '"' || char == '&' || char == '+' || char == '/' ||
		char == '\\' || char == '\n' || char == '\r' || char == '\t'
}

// containsInvalidFeaturePatterns 检查是否包含无效的特性模式
func (v *TextValidator) containsInvalidFeaturePatterns(text string) bool {
	invalidPatterns := []string{
		"see more", "show more", "read more", "click here", "learn more",
		"view details", "customer reviews", "product description",
		"technical details", "additional information", "about this item",
		"›", "‹", "»", "«", "$", "price", "add to cart", "buy now",
		"quantity", "shipping", "return", "warranty", "free 30-day",
		"refund", "replacement", "transaction is secure", "recycled",
		"climate pledge", "certified", "supply chain", "independently verified",
		"contains at least", "percent", "by weight", "4 star", "5 star",
		"1 star", "2 star", "3 star",
	}

	lowerText := strings.ToLower(text)
	for _, pattern := range invalidPatterns {
		if strings.Contains(lowerText, pattern) {
			return true
		}
	}
	return false
}

// containsFeatureKeywords 检查是否包含特性关键词
func (v *TextValidator) containsFeatureKeywords(text string) bool {
	featureKeywords := []string{
		"design", "material", "quality", "durable", "comfortable",
		"adjustable", "waterproof", "lightweight", "perfect", "ideal",
		"suitable", "gift", "occasion", "style", "elegant", "classic",
		"modern", "vintage", "handmade", "crafted", "premium", "luxury",
		"service", "support", "guarantee", "satisfaction",
	}

	lowerText := strings.ToLower(text)
	for _, keyword := range featureKeywords {
		if strings.Contains(lowerText, keyword) {
			return true
		}
	}
	return false
}
