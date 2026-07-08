package sherr_test

import (
	"errors"
	"fmt"
	"testing"

	"task-processor/internal/shein/sherr"
)

// TestIsFilteredError 验证业务过滤错误识别
func TestIsFilteredError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil 不是过滤错误", nil, false},
		{"普通错误不是过滤错误", errors.New("some error"), false},
		{"NewFilteredError 返回过滤错误", sherr.NewFilteredError("低于筛选规则最低价格"), true},
		{"包含关键词'低于筛选规则'", errors.New("价格低于筛选规则最低限制"), true},
		{"包含关键词'高于筛选规则'", errors.New("价格高于筛选规则最高限制"), true},
		{"包含关键词'超过筛选规则'", errors.New("数量超过筛选规则"), true},
		{"包含关键词'筛选规则最低'", errors.New("筛选规则最低价格"), true},
		{"包含关键词'筛选规则最高'", errors.New("筛选规则最高价格"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sherr.IsFilteredError(tt.err)
			if got != tt.want {
				t.Errorf("IsFilteredError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// TestIsRetryableError 验证可重试错误识别
func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"可重试错误", sherr.NewRetryableError("网络超时", errors.New("timeout")), true},
		{"不可重试错误", sherr.NewNonRetryableError("业务错误", errors.New("invalid")), false},
		{"过滤错误不可重试", sherr.NewFilteredError("低于筛选规则"), false},
		{"Cookie加载错误不可重试", sherr.NewCookieLoadError(1, 2, "cookie失效"), false},
		{"显式可重试错误不会被内部404字符串覆盖", sherr.NewRetryableError("fetch product data failed", errors.New("driver download failed: 404 Not Found")), true},
		// 普通 error 默认可重试（兜底策略）
		{"普通错误默认可重试", errors.New("unknown error"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sherr.IsRetryableError(tt.err)
			if got != tt.want {
				t.Errorf("IsRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// TestNewRetryableError_AuthExpired 认证过期错误不被包装为可重试
func TestNewRetryableError_AuthExpired(t *testing.T) {
	authErr := errors.New("20302 子系统登录重定向")
	wrapped := sherr.NewRetryableError("包装认证错误", authErr)

	// 认证过期错误应原样返回，不被包装
	if !errors.Is(wrapped, authErr) {
		t.Error("auth expired error should be returned as-is without wrapping")
	}
	if sherr.IsRetryableError(wrapped) {
		t.Error("auth expired error should not be retryable")
	}
}

// TestCookieLoadError 验证 CookieLoadError 的创建和识别
func TestCookieLoadError(t *testing.T) {
	err := sherr.NewCookieLoadError(100, 200, "cookie已过期")

	// 验证错误消息格式
	if err.Error() == "" {
		t.Error("CookieLoadError.Error() should not be empty")
	}

	// 验证 IsCookieLoadError 识别
	cookieErr, ok := sherr.IsCookieLoadError(err)
	if !ok {
		t.Fatal("IsCookieLoadError should return true for CookieLoadError")
	}
	if cookieErr.TenantID != 100 {
		t.Errorf("TenantID: want 100, got %d", cookieErr.TenantID)
	}
	if cookieErr.StoreID != 200 {
		t.Errorf("StoreID: want 200, got %d", cookieErr.StoreID)
	}

	// 普通错误不是 CookieLoadError
	_, ok = sherr.IsCookieLoadError(errors.New("other"))
	if ok {
		t.Error("IsCookieLoadError should return false for non-CookieLoadError")
	}
}

// TestNewFilteredError_ErrorMessage 验证过滤错误消息
func TestNewFilteredError_ErrorMessage(t *testing.T) {
	msg := "价格低于筛选规则最低限制"
	err := sherr.NewFilteredError(msg)

	if err.Error() != msg {
		t.Errorf("FilteredError.Error() = %q, want %q", err.Error(), msg)
	}
}

// TestRetryableError_Unwrap 验证错误链 unwrap
func TestRetryableError_Unwrap(t *testing.T) {
	cause := errors.New("root cause")
	wrapped := sherr.NewRetryableError("outer message", cause)

	if !errors.Is(wrapped, cause) {
		t.Error("wrapped retryable error should unwrap to the original cause")
	}
}

// TestNonRetryableError_Message 验证不可重试错误消息包含原始错误
func TestNonRetryableError_Message(t *testing.T) {
	cause := errors.New("db connection failed")
	err := sherr.NewNonRetryableError("数据库错误", cause)

	msg := err.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
	// 消息应包含原始错误信息
	if !containsStr(msg, "db connection failed") {
		t.Errorf("error message %q should contain cause message", msg)
	}
}

// TestIsRetryableError_AuthExpiredVariants 验证多种认证过期消息格式
func TestIsRetryableError_AuthExpiredVariants(t *testing.T) {
	authErrors := []error{
		fmt.Errorf("20302 子系统登录重定向"),
		fmt.Errorf("认证已过期，请重新登录"),
		fmt.Errorf("需要重新登录"),
	}

	for _, err := range authErrors {
		t.Run(err.Error(), func(t *testing.T) {
			if sherr.IsRetryableError(err) {
				t.Errorf("auth error should not be retryable: %v", err)
			}
		})
	}
}

func TestIsAuthenticationExpiredError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "code and redirect message", err: fmt.Errorf("20302 子系统登录重定向"), want: true},
		{name: "expired message", err: fmt.Errorf("认证已过期，请重新登录"), want: true},
		{name: "rewrapped auth message", err: fmt.Errorf("outer: %w", fmt.Errorf("需要重新登录")), want: true},
		{name: "generic error", err: fmt.Errorf("temporary network error"), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sherr.IsAuthenticationExpiredError(tc.err); got != tc.want {
				t.Fatalf("IsAuthenticationExpiredError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
