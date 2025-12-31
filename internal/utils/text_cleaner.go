package utils

import (
	"regexp"
	"strings"
	"unicode"
)

// CleanProductTitle 清理产品标题，移除特殊符号和表情符号
func CleanProductTitle(title string) string {
	if title == "" {
		return title
	}

	// 移除表情符号和其他Unicode符号（包括中文字符）
	title = removeEmojisAndSymbols(title)

	// 移除或替换特殊字符，保留基本标点符号
	title = cleanSpecialCharacters(title)

	// 清理多余的空格
	title = cleanWhitespace(title)

	// 修复逗号前后的空格问题
	title = fixCommaSpacing(title)

	return title
}

// removeEmojisAndSymbols 移除表情符号和特殊Unicode符号
func removeEmojisAndSymbols(text string) string {
	var result strings.Builder

	for _, r := range text {
		// 保留基本的拉丁字符、数字、基本标点符号和中文字符
		if isAllowedCharacter(r) {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// isAllowedCharacter 判断字符是否允许保留（不包括中文）
func isAllowedCharacter(r rune) bool {
	// 基本拉丁字符和数字
	if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
		return true
	}

	// 基本标点符号和空格
	basicPunctuation := " .,!?()-_+=/\\:;\"'[]{}|"
	if strings.ContainsRune(basicPunctuation, r) {
		return true
	}

	// 不允许中文字符（TEMU API不支持）
	if r >= 0x4e00 && r <= 0x9fff {
		return false
	}

	// 其他常用Unicode字符范围（如重音字符等），但排除高位字符
	if unicode.IsLetter(r) && r < 0x1F000 && r < 0x4e00 {
		return true
	}

	return false
}

// cleanSpecialCharacters 清理特殊字符
func cleanSpecialCharacters(text string) string {
	// 移除连续的特殊字符，只保留ASCII字母、数字和基本标点
	// 注意：不使用 \p{L} 因为它会匹配中文字符
	re := regexp.MustCompile(`[^a-zA-Z0-9\s.,!?()\-_+=/:;"'\[\]{}|]+`)
	text = re.ReplaceAllString(text, " ")

	// 替换多个连续的破折号或下划线
	re = regexp.MustCompile(`[-_]{2,}`)
	text = re.ReplaceAllString(text, "-")

	return text
}

// cleanWhitespace 清理空白字符
func cleanWhitespace(text string) string {
	// 移除多余的空格
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")

	// 移除首尾空格
	text = strings.TrimSpace(text)

	return text
}

// fixCommaSpacing 修复逗号前后的空格问题
func fixCommaSpacing(text string) string {
	// 1. 移除逗号前的空格 (TEMU API要求: "Product name should not have a space before the mark ','")
	re := regexp.MustCompile(`\s+,`)
	text = re.ReplaceAllString(text, ",")

	// 2. 确保逗号后有空格（如果后面不是空格或字符串结尾）
	re = regexp.MustCompile(`,(\S)`)
	text = re.ReplaceAllString(text, ", $1")

	// 3. 清理逗号后的多余空格
	re = regexp.MustCompile(`,\s{2,}`)
	text = re.ReplaceAllString(text, ", ")

	return text
}
