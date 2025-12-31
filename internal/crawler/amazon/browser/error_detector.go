// Package browser 提供浏览器错误检测功能
package browser

import (
	"strings"
	"task-processor/internal/domain/model"
)

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

	// 检查是否为产品不存在错误（不应触发浏览器重建）
	if _, ok := err.(*model.ProductNotFoundError); ok {
		return false
	}

	errorStr := err.Error()

	// 如果错误信息包含"产品页面不存在"或"产品页面缺少必要元素"，不触发重建
	if strings.Contains(errorStr, "产品页面不存在") || strings.Contains(errorStr, "产品页面缺少必要元素") {
		return false
	}

	// 检测常见的风控和严重错误模式
	blockPatterns := []string{
		"SIGN_IN_REQUIRED", // 需要登录才能更新位置
		"timeout", "Timeout", "TIMEOUT",
		"blocked", "Blocked", "BLOCKED",
		"captcha", "CAPTCHA", "Captcha",
		"robot", "Robot", "ROBOT",
		"access denied", "Access Denied", "ACCESS DENIED",
		"forbidden", "Forbidden", "FORBIDDEN",
		"503", "502", "504", // 服务器错误
		"connection refused", "Connection refused",
		"network error", "Network error",
		"page crashed", "Page crashed",
		"browser disconnected", "Browser disconnected",
		"context closed", "Context closed",
		"navigation failed", "Navigation failed",
		"Timeout 30000ms exceeded", // 特定的超时错误
	}

	for _, pattern := range blockPatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}

	return false
}

// IsTimeoutError 检测是否为超时错误
func (ed *ErrorDetector) IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := err.Error()
	timeoutPatterns := []string{
		"timeout", "Timeout", "TIMEOUT",
		"Timeout 30000ms exceeded",
		"deadline exceeded",
	}

	for _, pattern := range timeoutPatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}

	return false
}

// IsNetworkError 检测是否为网络错误
func (ed *ErrorDetector) IsNetworkError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := err.Error()
	networkPatterns := []string{
		"network error", "Network error",
		"connection refused", "Connection refused",
		"connection reset", "Connection reset",
		"connection timeout", "Connection timeout",
		"no route to host", "No route to host",
	}

	for _, pattern := range networkPatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}

	return false
}

// IsCaptchaError 检测是否为验证码错误
func (ed *ErrorDetector) IsCaptchaError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := err.Error()
	captchaPatterns := []string{
		"captcha", "CAPTCHA", "Captcha",
		"robot", "Robot", "ROBOT",
		"verification", "Verification",
	}

	for _, pattern := range captchaPatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}

	return false
}

// IsAuthenticationError 检测是否为认证错误
func (ed *ErrorDetector) IsAuthenticationError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := err.Error()
	authPatterns := []string{
		"SIGN_IN_REQUIRED",
		"authentication required", "Authentication required",
		"login required", "Login required",
		"unauthorized", "Unauthorized",
		"401",
	}

	for _, pattern := range authPatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}

	return false
}

// IsServerError 检测是否为服务器错误
func (ed *ErrorDetector) IsServerError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := err.Error()
	serverErrorPatterns := []string{
		"500", "502", "503", "504",
		"internal server error", "Internal Server Error",
		"bad gateway", "Bad Gateway",
		"service unavailable", "Service Unavailable",
		"gateway timeout", "Gateway Timeout",
	}

	for _, pattern := range serverErrorPatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}

	return false
}

// IsBrowserCrashError 检测是否为浏览器崩溃错误
func (ed *ErrorDetector) IsBrowserCrashError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := err.Error()
	crashPatterns := []string{
		"page crashed", "Page crashed",
		"browser disconnected", "Browser disconnected",
		"context closed", "Context closed",
		"browser closed", "Browser closed",
	}

	for _, pattern := range crashPatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}

	return false
}

// IsProductNotFoundError 检测是否为产品不存在错误
func (ed *ErrorDetector) IsProductNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否为ProductNotFoundError类型
	if _, ok := err.(*model.ProductNotFoundError); ok {
		return true
	}

	errorStr := err.Error()
	notFoundPatterns := []string{
		"产品页面不存在",
		"产品页面缺少必要元素",
		"product not found", "Product not found",
		"404",
	}

	for _, pattern := range notFoundPatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}

	return false
}

// GetErrorType 获取错误类型
func (ed *ErrorDetector) GetErrorType(err error) string {
	if err == nil {
		return "none"
	}

	if ed.IsProductNotFoundError(err) {
		return "product_not_found"
	}

	if ed.IsAuthenticationError(err) {
		return "authentication"
	}

	if ed.IsCaptchaError(err) {
		return "captcha"
	}

	if ed.IsBrowserCrashError(err) {
		return "browser_crash"
	}

	if ed.IsServerError(err) {
		return "server_error"
	}

	if ed.IsNetworkError(err) {
		return "network"
	}

	if ed.IsTimeoutError(err) {
		return "timeout"
	}

	return "unknown"
}

// ShouldRetry 判断错误是否应该重试
func (ed *ErrorDetector) ShouldRetry(err error) bool {
	if err == nil {
		return false
	}

	errorType := ed.GetErrorType(err)

	// 产品不存在错误不应该重试
	if errorType == "product_not_found" {
		return false
	}

	// 其他错误类型可以重试
	retryableTypes := []string{
		"timeout",
		"network",
		"server_error",
	}

	for _, retryableType := range retryableTypes {
		if errorType == retryableType {
			return true
		}
	}

	return false
}
