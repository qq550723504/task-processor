// Package api 提供SHEIN API的通用类型定义
package api

import (
	"fmt"
	"strings"
)

// APIResponse 通用API响应结构
type APIResponse struct {
	Code string `json:"code"` // 响应码
	Msg  string `json:"msg"`  // 响应消息
}

// APIError API错误类型
type APIError struct {
	StatusCode int    // HTTP状态码
	Message    string // 错误消息
	URL        string // 请求URL
}

// Error 实现error接口
func (e *APIError) Error() string {
	return fmt.Sprintf("API错误 [%d]: %s (URL: %s)", e.StatusCode, e.Message, e.URL)
}

// AuthenticationExpiredError 认证过期错误
type AuthenticationExpiredError struct {
	TenantID int64  // 租户ID
	ShopID   int64  // 店铺ID
	Code     string // 错误码
	Message  string // 错误消息
}

// Error 实现error接口
func (e *AuthenticationExpiredError) Error() string {
	return fmt.Sprintf("认证过期 [%s]: %s (TenantID: %d, ShopID: %d)", e.Code, e.Message, e.TenantID, e.ShopID)
}

// CookieError Cookie相关错误
type CookieError struct {
	TenantID int64  // 租户ID
	ShopID   int64  // 店铺ID
	Code     string // 错误码
	Message  string // 错误消息
}

// Error 实现error接口
func (e *CookieError) Error() string {
	return fmt.Sprintf("Cookie错误 [%s]: %s (TenantID: %d, ShopID: %d)", e.Code, e.Message, e.TenantID, e.ShopID)
}

// IsAuthenticationExpired 检查错误是否为认证过期错误
func IsAuthenticationExpired(err error) (*AuthenticationExpiredError, bool) {
	// 直接检查 AuthenticationExpiredError 类型
	if authErr, ok := err.(*AuthenticationExpiredError); ok {
		return authErr, true
	}

	// 检查 APIError 中是否包含认证过期信息
	if apiErr, ok := err.(*APIError); ok {
		// 检查状态码是否为302（重定向）
		if apiErr.StatusCode == 302 {
			return &AuthenticationExpiredError{
				Code:    "20302",
				Message: apiErr.Message,
			}, true
		}
		// 检查错误消息中是否包含认证过期关键词
		if strings.Contains(apiErr.Message, "20302") ||
			strings.Contains(apiErr.Message, "子系统登录重定向") ||
			strings.Contains(apiErr.Message, "认证已过期") {
			return &AuthenticationExpiredError{
				Code:    "20302",
				Message: apiErr.Message,
			}, true
		}
	}

	return nil, false
}
