package api

import "fmt"

// APIError API错误类型
type APIError struct {
	StatusCode int
	Message    string
	URL        string
}

// Error 实现error接口
func (e *APIError) Error() string {
	return e.Message
}

// AuthenticationExpiredError 认证过期错误类型
type AuthenticationExpiredError struct {
	TenantID int64
	ShopID   int64
	Code     string
	Message  string
}

// Error 实现error接口
func (e *AuthenticationExpiredError) Error() string {
	return e.Message
}

// CookieError Cookie获取失败错误类型
type CookieError struct {
	TenantID int64
	ShopID   int64
	Code     string
	Message  string
}

// Error 实现error接口
func (e *CookieError) Error() string {
	return e.Message
}

// RedisCookieError 保留用于向后兼容
type RedisCookieError = CookieError

// IsAuthenticationExpired 检查是否为认证过期错误
func IsAuthenticationExpired(err error) (*AuthenticationExpiredError, bool) {
	// 直接检查是否为认证过期错误
	if authErr, ok := err.(*AuthenticationExpiredError); ok {
		return authErr, true
	}

	// 检查是否为Cookie错误
	if cookieErr, ok := err.(*CookieError); ok {
		// 将Cookie错误转换为认证过期错误
		return &AuthenticationExpiredError{
			TenantID: cookieErr.TenantID,
			ShopID:   cookieErr.ShopID,
			Code:     "COOKIE_FAILED",
			Message:  fmt.Sprintf("Cookie获取失败，需要重新登录: %s", cookieErr.Message),
		}, true
	}

	// 检查错误消息是否包含认证过期信息
	if err != nil {
		errMsg := err.Error()
		if contains(errMsg, "20302") && contains(errMsg, "子系统登录重定向") {
			// 创建一个认证过期错误对象
			return &AuthenticationExpiredError{
				Code:    "20302",
				Message: "子系统登录重定向",
			}, true
		}
		if contains(errMsg, "认证已过期") || contains(errMsg, "需要重新登录") {
			return &AuthenticationExpiredError{
				Code:    "AUTH_EXPIRED",
				Message: errMsg,
			}, true
		}
		// 检查Cookie获取失败的错误消息
		if contains(errMsg, "从内存获取Cookie失败") || contains(errMsg, "Cookie不存在") {
			return &AuthenticationExpiredError{
				Code:    "COOKIE_FAILED",
				Message: "Cookie获取失败，需要重新登录: " + errMsg,
			}, true
		}
	}

	// 递归检查包装的错误
	for err != nil {
		if authErr, ok := err.(*AuthenticationExpiredError); ok {
			return authErr, true
		}

		// 检查是否为Cookie错误
		if cookieErr, ok := err.(*CookieError); ok {
			return &AuthenticationExpiredError{
				TenantID: cookieErr.TenantID,
				ShopID:   cookieErr.ShopID,
				Code:     "COOKIE_FAILED",
				Message:  fmt.Sprintf("Cookie获取失败，需要重新登录: %s", cookieErr.Message),
			}, true
		}

		// 检查错误消息
		errMsg := err.Error()
		if contains(errMsg, "20302") && contains(errMsg, "子系统登录重定向") {
			return &AuthenticationExpiredError{
				Code:    "20302",
				Message: "子系统登录重定向",
			}, true
		}
		if contains(errMsg, "认证已过期") || contains(errMsg, "需要重新登录") {
			return &AuthenticationExpiredError{
				Code:    "AUTH_EXPIRED",
				Message: errMsg,
			}, true
		}
		// 检查Cookie获取失败的错误消息
		if contains(errMsg, "从内存获取Cookie失败") || contains(errMsg, "Cookie不存在") {
			return &AuthenticationExpiredError{
				Code:    "COOKIE_FAILED",
				Message: "Cookie获取失败，需要重新登录: " + errMsg,
			}, true
		}

		// 检查是否实现了 Unwrap 方法
		if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
			err = unwrapper.Unwrap()
		} else {
			break
		}
	}

	return nil, false
}

// contains 检查字符串是否包含子字符串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && findSubstring(s, substr)))
}

// findSubstring 查找子字符串
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// APIResponse 通用API响应结构
type APIResponse struct {
	Code string      `json:"code"`
	Msg  string      `json:"msg"`
	Info interface{} `json:"info"`
	BBL  interface{} `json:"bbl"`
}

// Success 检查API调用是否成功
func (r *APIResponse) Success() bool {
	return r.Code == "0"
}
