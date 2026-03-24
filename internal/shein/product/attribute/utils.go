// Package attribute 提供SHEIN平台属性选择工具方法
package attribute

import (
	"task-processor/internal/pkg/jsonx"
)

// AttributeUtils 属性工具类
type AttributeUtils struct{}

// NewAttributeUtils 创建新的属性工具类
func NewAttributeUtils() *AttributeUtils {
	return &AttributeUtils{}
}

// CleanJSONContent 清理JSON内容
func (u *AttributeUtils) CleanJSONContent(content string) string {
	return jsonx.CleanLLMResponse(content)
}

// FixCommonJSONIssues 修复常见的JSON问题（占位实现，可按需扩展）
func (u *AttributeUtils) FixCommonJSONIssues(jsonStr string) string {
	return jsonStr
}
