// Package updater 提供自动更新器的工具函数
package updater

import "task-processor/internal/pkg/strutil"

// Contains 检查字符串是否包含子串（不区分大小写）
// 已废弃: 请使用 strutil.Contains
func Contains(s, substr string) bool {
	return strutil.Contains(s, substr)
}

// indexOf 查找子字符串位置
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// findSubstring 查找子字符串
// 已废弃: 请使用 strutil.FindSubstring
func findSubstring(s, substr string) bool {
	return strutil.FindSubstring(s, substr)
}

// trimPrefix 移除字符串前缀
func trimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

// splitVersion 分割版本号为数字数组
func splitVersion(version string) []int {
	parts := []int{}
	current := 0
	hasDigit := false

	for i := 0; i < len(version); i++ {
		c := version[i]
		if c >= '0' && c <= '9' {
			current = current*10 + int(c-'0')
			hasDigit = true
		} else if c == '.' || c == '-' {
			if hasDigit {
				parts = append(parts, current)
				current = 0
				hasDigit = false
			}
		}
	}

	if hasDigit {
		parts = append(parts, current)
	}

	return parts
}
