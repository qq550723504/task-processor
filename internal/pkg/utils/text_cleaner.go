package utils

import (
	"regexp"
	"strings"
)

// CleanProductTitle 清理产品标题，移除特殊字符、表情符号和中文字符
func CleanProductTitle(title string) string {
	if title == "" {
		return ""
	}

	// 1. 移除表情符号
	title = removeEmojis(title)

	// 2. 移除中文字符
	title = removeChineseCharacters(title)

	// 3. 移除特殊字符，保留字母、数字、空格和基本标点
	title = removeSpecialCharacters(title)

	// 4. 清理多余的空白字符
	title = cleanWhitespace(title)

	return strings.TrimSpace(title)
}

// removeEmojis 移除表情符号
func removeEmojis(text string) string {
	// 移除表情符号的正则表达式
	emojiPattern := `[\x{1F600}-\x{1F64F}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{1F1E0}-\x{1F1FF}]|[\x{2600}-\x{26FF}]|[\x{2700}-\x{27BF}]`
	re := regexp.MustCompile(emojiPattern)
	return re.ReplaceAllString(text, "")
}

// removeChineseCharacters 移除中文字符
func removeChineseCharacters(text string) string {
	// 移除中文字符（包括中文标点符号）
	re := regexp.MustCompile(`[\p{Han}]|[，。！？；：""''（）【】《》、]`)
	return re.ReplaceAllString(text, "")
}

// removeSpecialCharacters 移除特殊字符
func removeSpecialCharacters(text string) string {
	// 移除特殊字符，保留字母、数字、空格和基本标点
	re := regexp.MustCompile(`[^\p{L}\p{N}\s.,!?()-]`)
	return re.ReplaceAllString(text, "")
}

// cleanWhitespace 清理多余的空白字符
func cleanWhitespace(text string) string {
	// 替换多个连续的空白字符为单个空格
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(text, " ")
}
