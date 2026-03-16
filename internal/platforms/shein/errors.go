// Package shein 提供SHEIN平台的错误定义
package shein

import (
	"errors"
	"fmt"
)

// CookieLoadError Cookie加载失败错误
type CookieLoadError struct {
	TenantID int64
	StoreID  int64
	Reason   string
}

// Error 实现error接口
func (e *CookieLoadError) Error() string {
	return fmt.Sprintf("Cookie加载失败 (租户=%d, 店铺=%d): %s", e.TenantID, e.StoreID, e.Reason)
}

// IsCookieLoadError 检查是否是Cookie加载错误
func IsCookieLoadError(err error) (*CookieLoadError, bool) {
	var cookieErr *CookieLoadError
	if errors.As(err, &cookieErr) {
		return cookieErr, true
	}
	return nil, false
}

// NewCookieLoadError 创建Cookie加载错误
func NewCookieLoadError(tenantID, storeID int64, reason string) *CookieLoadError {
	return &CookieLoadError{
		TenantID: tenantID,
		StoreID:  storeID,
		Reason:   reason,
	}
}
