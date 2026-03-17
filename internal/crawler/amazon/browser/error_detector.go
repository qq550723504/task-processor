// Package browser 提供浏览器错误检测功能
package browser

import (
	"errors"
	"strings"
	"task-processor/internal/model"
)

// containsAny 检查字符串是否包含任意一个模式
func containsAny(s string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// ErrorDetector 错误检测器
type ErrorDetector struct{}

// NewErrorDetector 创建错误检测器
func NewErrorDetector() *ErrorDetector {
	return &ErrorDetector{}
}

// IsBlockedOrSeriousError 检测是否为风控或严重错误
func (ed *ErrorDetector) IsBlockedOrSeriousError(err error) bool {
	if err == nil {
		return false
	}

	var notFoundErr *model.ProductNotFoundError
	if errors.As(err, &notFoundErr) {
		return false
	}

	errorStr := err.Error()

	notFoundPatterns := []string{
		"产品页面不存在", "产品页面缺少必要元素", "页面不存在(404)", "页面不存在",
		"404", "page not found", "Page not found", "页面未准备就绪: 页面不存在", "不是有效的产品页面",
	}
	if containsAny(errorStr, notFoundPatterns) {
		return false
	}

	blockPatterns := []string{
		"SIGN_IN_REQUIRED",
		"timeout", "Timeout", "TIMEOUT",
		"blocked", "Blocked", "BLOCKED",
		"captcha", "CAPTCHA", "Captcha",
		"robot", "Robot", "ROBOT",
		"access denied", "Access Denied", "ACCESS DENIED",
		"forbidden", "Forbidden", "FORBIDDEN",
		"503", "502", "504",
		"connection refused", "Connection refused",
		"network error", "Network error",
		"page crashed", "Page crashed",
		"browser disconnected", "Browser disconnected",
		"context closed", "Context closed",
		"navigation failed", "Navigation failed",
		"Timeout 30000ms exceeded",
	}
	return containsAny(errorStr, blockPatterns)
}

// IsTimeoutError 检测是否为超时错误
func (ed *ErrorDetector) IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return containsAny(err.Error(), []string{
		"timeout", "Timeout", "TIMEOUT", "Timeout 30000ms exceeded", "deadline exceeded",
	})
}

// IsNetworkError 检测是否为网络错误
func (ed *ErrorDetector) IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	return containsAny(err.Error(), []string{
		"network error", "Network error",
		"connection refused", "Connection refused",
		"connection reset", "Connection reset",
		"connection timeout", "Connection timeout",
		"no route to host", "No route to host",
	})
}

// IsCaptchaError 检测是否为验证码错误
func (ed *ErrorDetector) IsCaptchaError(err error) bool {
	if err == nil {
		return false
	}
	return containsAny(err.Error(), []string{
		"captcha", "CAPTCHA", "Captcha",
		"robot", "Robot", "ROBOT",
		"verification", "Verification",
	})
}

// IsAuthenticationError 检测是否为认证错误
func (ed *ErrorDetector) IsAuthenticationError(err error) bool {
	if err == nil {
		return false
	}
	return containsAny(err.Error(), []string{
		"SIGN_IN_REQUIRED",
		"authentication required", "Authentication required",
		"login required", "Login required",
		"unauthorized", "Unauthorized",
		"401",
	})
}

// IsServerError 检测是否为服务器错误
func (ed *ErrorDetector) IsServerError(err error) bool {
	if err == nil {
		return false
	}
	return containsAny(err.Error(), []string{
		"500", "502", "503", "504",
		"internal server error", "Internal Server Error",
		"bad gateway", "Bad Gateway",
		"service unavailable", "Service Unavailable",
		"gateway timeout", "Gateway Timeout",
	})
}

// IsBrowserCrashError 检测是否为浏览器崩溃错误
func (ed *ErrorDetector) IsBrowserCrashError(err error) bool {
	if err == nil {
		return false
	}
	return containsAny(err.Error(), []string{
		"page crashed", "Page crashed",
		"browser disconnected", "Browser disconnected",
		"context closed", "Context closed",
		"browser closed", "Browser closed",
	})
}

// IsProductNotFoundError 检测是否为产品不存在错误
func (ed *ErrorDetector) IsProductNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	var notFoundErr *model.ProductNotFoundError
	if errors.As(err, &notFoundErr) {
		return true
	}

	return containsAny(err.Error(), []string{
		"产品页面不存在", "产品页面缺少必要元素", "页面不存在(404)", "页面不存在",
		"页面未准备就绪: 页面不存在", "不是有效的产品页面",
		"product not found", "Product not found", "404",
	})
}

// GetErrorType 获取错误类型
func (ed *ErrorDetector) GetErrorType(err error) string {
	if err == nil {
		return "none"
	}
	switch {
	case ed.IsProductNotFoundError(err):
		return "product_not_found"
	case ed.IsAuthenticationError(err):
		return "authentication"
	case ed.IsCaptchaError(err):
		return "captcha"
	case ed.IsBrowserCrashError(err):
		return "browser_crash"
	case ed.IsServerError(err):
		return "server_error"
	case ed.IsNetworkError(err):
		return "network"
	case ed.IsTimeoutError(err):
		return "timeout"
	default:
		return "unknown"
	}
}

// ShouldRetry 判断错误是否应该重试
func (ed *ErrorDetector) ShouldRetry(err error) bool {
	if err == nil {
		return false
	}
	switch ed.GetErrorType(err) {
	case "timeout", "network", "server_error":
		return true
	default:
		return false
	}
}
