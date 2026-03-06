package strutil

import (
	"regexp"
	"strings"
)

// ContainsIgnoreCase 不区分大小写的字符串包含检查
func ContainsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// FindSubstring 查找子字符串
func FindSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	return strings.Contains(s, substr)
}

// TruncateString 截断字符串到指定长度（按字符数，非字节数）
func TruncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

// CleanWhitespace 清理多余的空白字符
func CleanWhitespace(text string) string {
	// 移除多余的空格
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(text, " "))
}

// Contains 检查字符串是否包含子串
func Contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// ToLower 转换为小写
func ToLower(s string) string {
	return strings.ToLower(s)
}

// ToUpper 转换为大写
func ToUpper(s string) string {
	return strings.ToUpper(s)
}
