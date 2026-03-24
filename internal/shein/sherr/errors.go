// Package sherr 提供 SHEIN 平台的错误类型定义
// 独立于其他 shein 子包，避免循环依赖
package sherr

import (
	"errors"
	"fmt"
	"strings"
)

// CookieLoadError Cookie加载失败错误
type CookieLoadError struct {
	TenantID int64
	StoreID  int64
	Reason   string
}

// Error 实现 error 接口
func (e *CookieLoadError) Error() string {
	return fmt.Sprintf("Cookie加载失败 (租户=%d, 店铺=%d): %s", e.TenantID, e.StoreID, e.Reason)
}

// IsCookieLoadError 检查是否是 Cookie 加载错误
func IsCookieLoadError(err error) (*CookieLoadError, bool) {
	var cookieErr *CookieLoadError
	if errors.As(err, &cookieErr) {
		return cookieErr, true
	}
	return nil, false
}

// NewCookieLoadError 创建 Cookie 加载错误
func NewCookieLoadError(tenantID, storeID int64, reason string) *CookieLoadError {
	return &CookieLoadError{
		TenantID: tenantID,
		StoreID:  storeID,
		Reason:   reason,
	}
}

// RetryableError 可重试错误接口
type RetryableError interface {
	error
	IsRetryable() bool
}

// retryableError 可重试错误实现
type retryableError struct {
	message    string
	retryable  bool
	wrappedErr error
}

func (e *retryableError) Error() string {
	if e.wrappedErr != nil {
		return e.message + ": " + e.wrappedErr.Error()
	}
	return e.message
}

func (e *retryableError) IsRetryable() bool {
	return e.retryable
}

func (e *retryableError) Unwrap() error {
	return e.wrappedErr
}

// NewRetryableError 创建可重试错误
func NewRetryableError(message string, err error) error {
	if isAuthenticationExpiredError(err) {
		return err
	}
	return &retryableError{message: message, retryable: true, wrappedErr: err}
}

// NewNonRetryableError 创建不可重试错误
func NewNonRetryableError(message string, err error) error {
	return &retryableError{message: message, retryable: false, wrappedErr: err}
}

// FilteredError 业务过滤错误（非真正的错误，只是不符合筛选条件）
type FilteredError struct {
	message string
}

func (e *FilteredError) Error() string     { return e.message }
func (e *FilteredError) IsRetryable() bool { return false }

// NewFilteredError 创建业务过滤错误
func NewFilteredError(message string) error {
	return &FilteredError{message: message}
}

// IsFilteredError 检查是否为业务过滤错误
func IsFilteredError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*FilteredError); ok {
		return true
	}
	errMsg := err.Error()
	for _, kw := range []string{"低于筛选规则", "高于筛选规则", "超过筛选规则", "筛选规则最低", "筛选规则最高"} {
		if strings.Contains(errMsg, kw) {
			return true
		}
	}
	return false
}

// IsRetryableError 检查错误是否可重试
func IsRetryableError(err error) bool {
	if isAuthenticationExpired(err) {
		return false
	}
	if isNonRetryableError(err) {
		return false
	}
	if re, ok := err.(RetryableError); ok {
		return re.IsRetryable()
	}
	return true
}

func isAuthenticationExpired(err error) bool {
	return isAuthenticationExpiredError(err)
}

func isAuthenticationExpiredError(err error) bool {
	for err != nil {
		msg := err.Error()
		if strings.Contains(msg, "20302") && strings.Contains(msg, "子系统登录重定向") {
			return true
		}
		if strings.Contains(msg, "认证已过期") || strings.Contains(msg, "需要重新登录") {
			return true
		}
		if u, ok := err.(interface{ Unwrap() error }); ok {
			err = u.Unwrap()
		} else {
			break
		}
	}
	return false
}

func isNonRetryableError(err error) bool {
	if err == nil {
		return false
	}
	var re *retryableError
	if errors.As(err, &re) && !re.IsRetryable() {
		return true
	}
	var fe *FilteredError
	if errors.As(err, &fe) {
		return true
	}
	// 优先用类型断言检查已知的非重试错误类型
	var cookieErr *CookieLoadError
	if errors.As(err, &cookieErr) {
		return true
	}

	notFoundPatterns := []string{
		"不是有效的产品页面", "产品页面不存在", "产品页面缺少必要元素",
		"页面不存在(404)", "页面不存在", "页面未准备就绪: 页面不存在",
		"product not found", "Product not found", "404", "not found", "Not Found",
		// 以下业务错误无法用类型断言，保留字符串匹配作为兜底
		"卖家SKU重复", "变体ASIN数量过多",
	}
	for err != nil {
		msg := err.Error()
		for _, p := range notFoundPatterns {
			if strings.Contains(msg, p) {
				return true
			}
		}
		if u, ok := err.(interface{ Unwrap() error }); ok {
			err = u.Unwrap()
		} else {
			break
		}
	}
	return false
}
