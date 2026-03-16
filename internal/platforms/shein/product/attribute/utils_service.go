// Package attribute 提供SHEIN平台属性选择工具方法
package attribute

import (
	"strings"
)

// AttributeUtils 属性工具类
type AttributeUtils struct{}

// NewAttributeUtils 创建新的属性工具类
func NewAttributeUtils() *AttributeUtils {
	return &AttributeUtils{}
}

// CleanJSONContent 清理JSON内容
func (u *AttributeUtils) CleanJSONContent(content string) string {
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
	}
	return strings.TrimSpace(content)
}

// FixCommonJSONIssues 修复常见的JSON问题
func (u *AttributeUtils) FixCommonJSONIssues(jsonStr string) string {
	// 这里可以添加具体的JSON修复逻辑
	// 目前只是一个占位符，实际实现可能需要根据具体问题来处理
	return jsonStr
}

// TruncateString 截断字符串到指定长度
func (u *AttributeUtils) TruncateString(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen] + "..."
}
