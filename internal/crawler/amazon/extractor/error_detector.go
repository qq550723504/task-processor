// Package extractor 提供错误检测功能
package extractor

import (
	"regexp"
	"strings"
	"task-processor/internal/domain/model"
)

// ErrorType 错误类型
type ErrorType int

const (
	// ErrorTypeNone 无错误
	ErrorTypeNone ErrorType = iota
	// ErrorTypeProductNotFound 产品不存在
	ErrorTypeProductNotFound
	// ErrorTypeTimeout 超时错误
	ErrorTypeTimeout
	// ErrorTypeBlocked 被阻止/风控
	ErrorTypeBlocked
	// ErrorTypeCaptcha 验证码错误
	ErrorTypeCaptcha
	// ErrorTypeNetwork 网络错误
	ErrorTypeNetwork
	// ErrorTypeBrowser 浏览器错误
	ErrorTypeBrowser
	// ErrorTypeServer 服务器错误
	ErrorTypeServer
)

// ErrorDetector 错误检测器
type ErrorDetector struct {
	timeoutPattern  *regexp.Regexp
	blockedPattern  *regexp.Regexp
	captchaPattern  *regexp.Regexp
	networkPattern  *regexp.Regexp
	browserPattern  *regexp.Regexp
	serverPattern   *regexp.Regexp
	notFoundPattern *regexp.Regexp
}

// NewErrorDetector 创建错误检测器
func NewErrorDetector() *ErrorDetector {
	return &ErrorDetector{
		timeoutPattern:  regexp.MustCompile(`(?i)(timeout|timed?\s*out|exceeded)`),
		blockedPattern:  regexp.MustCompile(`(?i)(blocked?|access\s*denied|forbidden)`),
		captchaPattern:  regexp.MustCompile(`(?i)(captcha|robot|verification)`),
		networkPattern:  regexp.MustCompile(`(?i)(network\s*error|connection\s*(refused|reset|closed))`),
		browserPattern:  regexp.MustCompile(`(?i)(page\s*crashed|browser\s*disconnected|context\s*closed|navigation\s*failed)`),
		serverPattern:   regexp.MustCompile(`(?i)(50[234]|server\s*error|service\s*unavailable)`),
		notFoundPattern: regexp.MustCompile(`(?i)(产品页面不存在|产品页面缺少必要元素|not\s*found)`),
	}
}

// DetectErrorType 检测错误类型
func (ed *ErrorDetector) DetectErrorType(err error) ErrorType {
	if err == nil {
		return ErrorTypeNone
	}

	// 检查是否为产品不存在错误
	if _, ok := err.(*model.ProductNotFoundError); ok {
		return ErrorTypeProductNotFound
	}

	errorStr := err.Error()

	// 按优先级检测错误类型
	if ed.notFoundPattern.MatchString(errorStr) {
		return ErrorTypeProductNotFound
	}
	if ed.timeoutPattern.MatchString(errorStr) {
		return ErrorTypeTimeout
	}
	if ed.blockedPattern.MatchString(errorStr) {
		return ErrorTypeBlocked
	}
	if ed.captchaPattern.MatchString(errorStr) {
		return ErrorTypeCaptcha
	}
	if ed.browserPattern.MatchString(errorStr) {
		return ErrorTypeBrowser
	}
	if ed.networkPattern.MatchString(errorStr) {
		return ErrorTypeNetwork
	}
	if ed.serverPattern.MatchString(errorStr) {
		return ErrorTypeServer
	}

	return ErrorTypeNone
}

// IsCriticalError 检测是否为关键错误（需要重建浏览器实例）
func (ed *ErrorDetector) IsCriticalError(err error) bool {
	errorType := ed.DetectErrorType(err)

	// 产品不存在不是关键错误
	if errorType == ErrorTypeProductNotFound {
		return false
	}

	// 以下错误类型是关键错误
	criticalTypes := []ErrorType{
		ErrorTypeTimeout,
		ErrorTypeBlocked,
		ErrorTypeCaptcha,
		ErrorTypeBrowser,
		ErrorTypeNetwork,
		ErrorTypeServer,
	}

	for _, criticalType := range criticalTypes {
		if errorType == criticalType {
			return true
		}
	}

	return false
}

// IsRetryableError 检测是否为可重试错误
func (ed *ErrorDetector) IsRetryableError(err error) bool {
	errorType := ed.DetectErrorType(err)

	// 以下错误类型可以重试
	retryableTypes := []ErrorType{
		ErrorTypeTimeout,
		ErrorTypeNetwork,
		ErrorTypeServer,
	}

	for _, retryableType := range retryableTypes {
		if errorType == retryableType {
			return true
		}
	}

	return false
}

// GetErrorMessage 获取友好的错误消息
func (ed *ErrorDetector) GetErrorMessage(err error) string {
	if err == nil {
		return "无错误"
	}

	errorType := ed.DetectErrorType(err)

	switch errorType {
	case ErrorTypeProductNotFound:
		return "产品不存在或页面缺少必要元素"
	case ErrorTypeTimeout:
		return "请求超时"
	case ErrorTypeBlocked:
		return "访问被阻止或拒绝"
	case ErrorTypeCaptcha:
		return "需要验证码验证"
	case ErrorTypeNetwork:
		return "网络连接错误"
	case ErrorTypeBrowser:
		return "浏览器实例错误"
	case ErrorTypeServer:
		return "服务器错误"
	default:
		return err.Error()
	}
}

// isCriticalErrorLegacy 旧版本的关键错误检测（保持向后兼容）
// 已弃用：请使用 ErrorDetector.IsCriticalError
func isCriticalErrorLegacy(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否为产品不存在错误（不是关键错误）
	if _, ok := err.(*model.ProductNotFoundError); ok {
		return false
	}

	errorStr := err.Error()

	// 如果错误信息包含"产品页面不存在"，不是关键错误
	if strings.Contains(errorStr, "产品页面不存在") || strings.Contains(errorStr, "产品页面缺少必要元素") {
		return false
	}

	// 检测关键错误模式
	criticalPatterns := []string{
		"timeout", "Timeout", "TIMEOUT",
		"Timeout 30000ms exceeded",
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
	}

	for _, pattern := range criticalPatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}

	return false
}
