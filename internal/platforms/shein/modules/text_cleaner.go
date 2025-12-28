// Package modules 提供SHEIN平台的文本清理功能
package modules

import (
	"fmt"
	"regexp"
	"strings"
)

// TextCleaner 文本清理器
type TextCleaner struct{}

// NewTextCleaner 创建新的文本清理器
func NewTextCleaner() *TextCleaner {
	return &TextCleaner{}
}

// RemoveBrandFromText 从文本中移除品牌词
func (c *TextCleaner) RemoveBrandFromText(text, brand string) string {
	if text == "" {
		return text
	}

	// 如果品牌为空，直接返回原文本
	if brand == "" {
		return text
	}

	// 使用正则表达式移除品牌词（不区分大小写）
	brandPattern := fmt.Sprintf(`(?i)\b%s\b`, regexp.QuoteMeta(brand))
	re := regexp.MustCompile(brandPattern)
	cleanedText := re.ReplaceAllString(text, "")

	// 清理多余的空格
	cleanedText = regexp.MustCompile(`\s+`).ReplaceAllString(cleanedText, " ")
	cleanedText = strings.TrimSpace(cleanedText)

	// 如果清理后的文本为空，返回原始文本
	if cleanedText == "" {
		return text
	}

	return cleanedText
}

// RemoveSpecialCharacters 移除特殊字符
func (c *TextCleaner) RemoveSpecialCharacters(text string) string {
	// 移除特殊字符，保留字母、数字、空格和基本标点
	re := regexp.MustCompile(`[^\p{L}\p{N}\s.,!?()-]`)
	cleaned := re.ReplaceAllString(text, "")

	// 清理多余的空格
	cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")
	return strings.TrimSpace(cleaned)
}

// RemoveEmojis 移除表情符号
func (c *TextCleaner) RemoveEmojis(text string) string {
	// 移除表情符号的正则表达式
	emojiPattern := `[\x{1F600}-\x{1F64F}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{1F1E0}-\x{1F1FF}]|[\x{2600}-\x{26FF}]|[\x{2700}-\x{27BF}]`
	re := regexp.MustCompile(emojiPattern)
	return re.ReplaceAllString(text, "")
}

// CleanWhitespace 清理多余的空白字符
func (c *TextCleaner) CleanWhitespace(text string) string {
	// 替换多个连续的空白字符为单个空格
	re := regexp.MustCompile(`\s+`)
	cleaned := re.ReplaceAllString(text, " ")
	return strings.TrimSpace(cleaned)
}

// RemoveHTMLTags 移除HTML标签
func (c *TextCleaner) RemoveHTMLTags(text string) string {
	// 移除HTML标签的正则表达式
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(text, "")
}

// NormalizeText 标准化文本（综合清理）
func (c *TextCleaner) NormalizeText(text string) string {
	// 移除HTML标签
	text = c.RemoveHTMLTags(text)

	// 移除表情符号
	text = c.RemoveEmojis(text)

	// 移除特殊字符
	text = c.RemoveSpecialCharacters(text)

	// 清理空白字符
	text = c.CleanWhitespace(text)

	return text
}

// RemoveForbiddenWords 移除禁用词汇
func (c *TextCleaner) RemoveForbiddenWords(text string) string {
	// 定义禁用词汇列表
	forbiddenWords := []string{
		"best", "perfect", "amazing", "incredible", "unbelievable",
		"guarantee", "guaranteed", "promise", "promises",
		"free shipping", "free delivery", "no risk",
		"medical", "therapeutic", "cure", "treatment",
	}

	for _, word := range forbiddenWords {
		// 不区分大小写地移除禁用词
		pattern := fmt.Sprintf(`(?i)\b%s\b`, regexp.QuoteMeta(word))
		re := regexp.MustCompile(pattern)
		text = re.ReplaceAllString(text, "")
	}

	// 清理多余的空格
	return c.CleanWhitespace(text)
}

// ValidateTextLength 验证文本长度
func (c *TextCleaner) ValidateTextLength(text string, minLength, maxLength int) error {
	length := len(text)

	if length < minLength {
		return fmt.Errorf("文本长度不能少于%d个字符，当前长度：%d", minLength, length)
	}

	if length > maxLength {
		return fmt.Errorf("文本长度不能超过%d个字符，当前长度：%d", maxLength, length)
	}

	return nil
}

// TruncateAtWordBoundary 在单词边界处截断文本
func (c *TextCleaner) TruncateAtWordBoundary(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}

	// 截断到最大长度
	truncated := text[:maxLength]

	// 查找最后一个空格位置
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > 0 && lastSpace > maxLength-50 {
		// 如果找到空格且位置合理，在空格处截断
		truncated = truncated[:lastSpace]
	}

	return strings.TrimSpace(truncated)
}

// TruncateAtSentenceBoundary 在句子边界处截断文本
func (c *TextCleaner) TruncateAtSentenceBoundary(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}

	// 截断到最大长度
	truncated := text[:maxLength]

	// 查找最后一个句号、问号或感叹号
	lastPeriod := strings.LastIndexAny(truncated, ".!?")
	if lastPeriod > 0 && lastPeriod > maxLength-200 {
		// 如果找到句号且位置合理，在句号处截断
		truncated = truncated[:lastPeriod+1]
	}

	return strings.TrimSpace(truncated)
}
