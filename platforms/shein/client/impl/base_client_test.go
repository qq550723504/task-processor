package impl

import (
	"testing"

	"github.com/imroc/req/v3"
)

func TestBaseAPIClient_GetBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		inputURL    string
		expectedURL string
	}{
		{
			name:        "正常的 HTTPS URL",
			inputURL:    "https://sellerhub.shein.com",
			expectedURL: "https://sellerhub.shein.com",
		},
		{
			name:        "正常的 HTTP URL",
			inputURL:    "http://sellerhub.shein.com",
			expectedURL: "http://sellerhub.shein.com",
		},
		{
			name:        "缺少协议的 URL",
			inputURL:    "sellerhub.shein.com",
			expectedURL: "https://sellerhub.shein.com",
		},
		{
			name:        "以斜杠开头的 URL（错误格式）",
			inputURL:    "/sellerhub.shein.com",
			expectedURL: "https://sellerhub.shein.com",
		},
		{
			name:        "空字符串",
			inputURL:    "",
			expectedURL: "",
		},
		{
			name:        "另一个域名",
			inputURL:    "sso.geiwohuo.com",
			expectedURL: "https://sso.geiwohuo.com",
		},
		{
			name:        "带端口的 URL",
			inputURL:    "localhost:8080",
			expectedURL: "https://localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewBaseAPIClient(tt.inputURL, 1, 1, req.C())
			result := client.GetBaseURL()

			if result != tt.expectedURL {
				t.Errorf("GetBaseURL() = %v, 期望 %v", result, tt.expectedURL)
			}
		})
	}
}

func TestBaseAPIClient_ExtractBaseURL(t *testing.T) {
	client := NewBaseAPIClient("https://sellerhub.shein.com", 1, 1, req.C())

	tests := []struct {
		name        string
		fullURL     string
		expectedURL string
	}{
		{
			name:        "完整的 HTTPS URL",
			fullURL:     "https://sellerhub.shein.com/api/product/list",
			expectedURL: "https://sellerhub.shein.com",
		},
		{
			name:        "完整的 HTTP URL",
			fullURL:     "http://example.com/api/test",
			expectedURL: "http://example.com",
		},
		{
			name:        "缺少协议的 URL",
			fullURL:     "sellerhub.shein.com/api/test",
			expectedURL: "https://sellerhub.shein.com",
		},
		{
			name:        "带端口的 URL",
			fullURL:     "https://localhost:8080/api/test",
			expectedURL: "https://localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.extractBaseURL(tt.fullURL)

			if result != tt.expectedURL {
				t.Errorf("extractBaseURL() = %v, 期望 %v", result, tt.expectedURL)
			}
		})
	}
}
