package client

import (
	"testing"
)

func TestParseCookieString(t *testing.T) {
	// 创建一个带有logger的CookieManager用于测试
	cm := NewCookieManager(1, nil)

	tests := []struct {
		name        string
		cookieStr   string
		expectedLen int
	}{
		{
			name:        "空字符串",
			cookieStr:   "",
			expectedLen: 0,
		},
		{
			name:        "单个cookie",
			cookieStr:   "session_id=abc123",
			expectedLen: 1,
		},
		{
			name:        "多个cookie",
			cookieStr:   "session_id=abc123; user_token=xyz789; lang=zh-CN",
			expectedLen: 3,
		},
		{
			name:        "包含空格的cookie",
			cookieStr:   " session_id = abc123 ; user_token = xyz789 ",
			expectedLen: 2,
		},
		{
			name:        "无效格式的cookie",
			cookieStr:   "session_id=abc123; invalid_cookie; user_token=xyz789",
			expectedLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cookies, err := cm.parseCookieString(tt.cookieStr)
			if err != nil {
				t.Errorf("parseCookieString() error = %v", err)
				return
			}

			if len(cookies) != tt.expectedLen {
				t.Errorf("parseCookieString() got %d cookies, want %d", len(cookies), tt.expectedLen)
			}

			// 验证cookie的基本属性
			for _, cookie := range cookies {
				if cookie.Name == "" {
					t.Error("Cookie name should not be empty")
				}
				if cookie.Domain != ".temu.com" {
					t.Errorf("Cookie domain should be .temu.com, got %s", cookie.Domain)
				}
				if cookie.Path != "/" {
					t.Errorf("Cookie path should be /, got %s", cookie.Path)
				}
			}
		})
	}
}
