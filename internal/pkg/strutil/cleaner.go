package strutil

import (
	"regexp"
	"strings"
)

// CleanProductTitle 清理产品标题，移除特殊字符、表情符号和中文字符
func CleanProductTitle(title string) string {
	if title == "" {
		return ""
	}

	title = removeEmojis(title)
	title = removeChineseCharacters(title)
	title = removeSpecialCharacters(title)
	title = CleanWhitespace(title)

	return strings.TrimSpace(title)
}

// removeEmojis 移除表情符号
func removeEmojis(text string) string {
	re := regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{1F1E0}-\x{1F1FF}]|[\x{2600}-\x{26FF}]|[\x{2700}-\x{27BF}]`)
	return re.ReplaceAllString(text, "")
}

// removeChineseCharacters 移除中文字符（包括中文标点符号）
func removeChineseCharacters(text string) string {
	re := regexp.MustCompile(`[\p{Han}]|[，。！？；：""''（）【】《》、]`)
	return re.ReplaceAllString(text, "")
}

// removeSpecialCharacters 移除特殊字符，保留字母、数字、空格和基本标点
func removeSpecialCharacters(text string) string {
	re := regexp.MustCompile(`[^\p{L}\p{N}\s.,!?()-]`)
	return re.ReplaceAllString(text, "")
}
