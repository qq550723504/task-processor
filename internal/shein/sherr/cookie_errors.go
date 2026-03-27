// Package sherr 提供 SHEIN 平台的错误类型定义
// 独立于其他 shein 子包，避免循环依赖
package sherr

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
