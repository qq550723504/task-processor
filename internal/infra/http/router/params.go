// Package router 提供路由辅助工具
package router

import (
	"net/http"
	"strings"
)

// ExtractPathParam 从 URL 路径中提取参数
// 例如: /api/v1/tasks/123 -> ExtractPathParam(r, "/api/v1/tasks/") -> "123"
//
// 参数:
//   - r: HTTP 请求
//   - prefix: 路径前缀
//
// 返回值:
//   - string: 提取的参数值
func ExtractPathParam(r *http.Request, prefix string) string {
	path := r.URL.Path
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.TrimPrefix(path, prefix)
}

// ExtractPathSegment 提取路径中的指定段
// 例如: /api/v1/tasks/123/status -> ExtractPathSegment(r, 3) -> "123"
//
// 参数:
//   - r: HTTP 请求
//   - index: 段索引（从 0 开始）
//
// 返回值:
//   - string: 提取的段值
func ExtractPathSegment(r *http.Request, index int) string {
	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if index < 0 || index >= len(segments) {
		return ""
	}
	return segments[index]
}
