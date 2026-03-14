package strutil

import (
	"regexp"
	"strings"
)

// CleanProductText 清洗产品描述文本，保留中文、英文、数字和基本标点。
// 与 CleanProductTitle 不同，此函数保留中文字符，适用于产品描述场景。
func CleanProductText(text string) string {
	if text == "" {
		return ""
	}
	text = filterAllowedRunes(text)
	text = collapseSpaces(text)
	text = collapseNewlines(text)
	return strings.TrimSpace(text)
}

// filterAllowedRunes 过滤不允许的字符，保留中文、英文、数字、空白和基本标点
func filterAllowedRunes(text string) string {
	result := make([]rune, 0, len(text))
	for _, r := range text {
		if isAllowedRune(r) {
			result = append(result, r)
		}
	}
	return string(result)
}

func isAllowedRune(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == ' ' || r == '\n' || r == '\r' || r == '\t' ||
		r == '.' || r == ',' || r == '!' || r == '?' ||
		r == ':' || r == ';' || r == '-' || r == '_' ||
		r == '(' || r == ')' || r == '[' || r == ']' ||
		r == '"' || r == '\'' || r == '/' || r == '\\'
}

// collapseSpaces 将连续空格/制表符压缩为单个空格
func collapseSpaces(text string) string {
	result := make([]rune, 0, len(text))
	lastSpace := false
	for _, r := range text {
		isSpace := r == ' ' || r == '\t'
		if isSpace {
			if !lastSpace {
				result = append(result, ' ')
				lastSpace = true
			}
		} else {
			result = append(result, r)
			lastSpace = false
		}
	}
	return string(result)
}

// collapseNewlines 将连续换行压缩为最多两个换行
func collapseNewlines(text string) string {
	result := make([]rune, 0, len(text))
	count := 0
	for _, r := range text {
		if r == '\n' || r == '\r' {
			count++
			if count <= 2 {
				result = append(result, '\n')
			}
		} else {
			count = 0
			result = append(result, r)
		}
	}
	return string(result)
}

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
