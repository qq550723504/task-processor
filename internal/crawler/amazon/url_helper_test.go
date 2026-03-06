package amazon

import (
	"strings"
	"testing"
)

// TestNewURLHelper 测试创建URL辅助工具
func TestNewURLHelper(t *testing.T) {
	helper := NewURLHelper()
	if helper == nil {
		t.Fatal("NewURLHelper 返回 nil")
	}
}

// TestGetCurrencyFromURL 测试从URL获取货币
func TestGetCurrencyFromURL(t *testing.T) {
	helper := NewURLHelper()

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		// 15个Amazon市场
		{
			name:     "美国市场",
			url:      "https://www.amazon.com/dp/B08N5WRWNW",
			expected: "USD",
		},
		{
			name:     "英国市场",
			url:      "https://www.amazon.co.uk/dp/B08N5WRWNW",
			expected: "GBP",
		},
		{
			name:     "德国市场",
			url:      "https://www.amazon.de/dp/B08N5WRWNW",
			expected: "EUR",
		},
		{
			name:     "法国市场",
			url:      "https://www.amazon.fr/dp/B08N5WRWNW",
			expected: "EUR",
		},
		{
			name:     "意大利市场",
			url:      "https://www.amazon.it/dp/B08N5WRWNW",
			expected: "EUR",
		},
		{
			name:     "西班牙市场",
			url:      "https://www.amazon.es/dp/B08N5WRWNW",
			expected: "EUR",
		},
		{
			name:     "加拿大市场",
			url:      "https://www.amazon.ca/dp/B08N5WRWNW",
			expected: "CAD",
		},
		{
			name:     "日本市场",
			url:      "https://www.amazon.co.jp/dp/B08N5WRWNW",
			expected: "JPY",
		},
		{
			name:     "澳大利亚市场",
			url:      "https://www.amazon.com.au/dp/B08N5WRWNW",
			expected: "AUD",
		},
		{
			name:     "印度市场",
			url:      "https://www.amazon.in/dp/B08N5WRWNW",
			expected: "INR",
		},
		{
			name:     "巴西市场",
			url:      "https://www.amazon.com.br/dp/B08N5WRWNW",
			expected: "BRL",
		},
		{
			name:     "墨西哥市场",
			url:      "https://www.amazon.com.mx/dp/B08N5WRWNW",
			expected: "MXN",
		},
		{
			name:     "荷兰市场",
			url:      "https://www.amazon.nl/dp/B08N5WRWNW",
			expected: "EUR",
		},
		{
			name:     "瑞典市场",
			url:      "https://www.amazon.se/dp/B08N5WRWNW",
			expected: "SEK",
		},
		{
			name:     "波兰市场",
			url:      "https://www.amazon.pl/dp/B08N5WRWNW",
			expected: "PLN",
		},
		// 边界情况
		{
			name:     "无效URL",
			url:      "not-a-valid-url",
			expected: "USD", // 默认美元
		},
		{
			name:     "未知域名",
			url:      "https://www.example.com/product",
			expected: "USD", // 默认美元
		},
		{
			name:     "大写域名",
			url:      "https://WWW.AMAZON.COM/dp/B08N5WRWNW",
			expected: "USD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helper.GetCurrencyFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("GetCurrencyFromURL(%q) = %q, 期望 %q", tt.url, result, tt.expected)
			}
		})
	}
}

// TestGetMarketplaceFromURL 测试从URL获取市场
func TestGetMarketplaceFromURL(t *testing.T) {
	helper := NewURLHelper()

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		// 15个Amazon市场
		{
			name:     "美国市场",
			url:      "https://www.amazon.com/dp/B08N5WRWNW",
			expected: "US",
		},
		{
			name:     "英国市场",
			url:      "https://www.amazon.co.uk/dp/B08N5WRWNW",
			expected: "UK",
		},
		{
			name:     "德国市场",
			url:      "https://www.amazon.de/dp/B08N5WRWNW",
			expected: "DE",
		},
		{
			name:     "法国市场",
			url:      "https://www.amazon.fr/dp/B08N5WRWNW",
			expected: "FR",
		},
		{
			name:     "意大利市场",
			url:      "https://www.amazon.it/dp/B08N5WRWNW",
			expected: "IT",
		},
		{
			name:     "西班牙市场",
			url:      "https://www.amazon.es/dp/B08N5WRWNW",
			expected: "ES",
		},
		{
			name:     "加拿大市场",
			url:      "https://www.amazon.ca/dp/B08N5WRWNW",
			expected: "CA",
		},
		{
			name:     "日本市场",
			url:      "https://www.amazon.co.jp/dp/B08N5WRWNW",
			expected: "JP",
		},
		{
			name:     "澳大利亚市场",
			url:      "https://www.amazon.com.au/dp/B08N5WRWNW",
			expected: "AU",
		},
		{
			name:     "印度市场",
			url:      "https://www.amazon.in/dp/B08N5WRWNW",
			expected: "IN",
		},
		{
			name:     "巴西市场",
			url:      "https://www.amazon.com.br/dp/B08N5WRWNW",
			expected: "BR",
		},
		{
			name:     "墨西哥市场",
			url:      "https://www.amazon.com.mx/dp/B08N5WRWNW",
			expected: "MX",
		},
		{
			name:     "荷兰市场",
			url:      "https://www.amazon.nl/dp/B08N5WRWNW",
			expected: "NL",
		},
		{
			name:     "瑞典市场",
			url:      "https://www.amazon.se/dp/B08N5WRWNW",
			expected: "SE",
		},
		{
			name:     "波兰市场",
			url:      "https://www.amazon.pl/dp/B08N5WRWNW",
			expected: "PL",
		},
		// 边界情况
		{
			name:     "无效URL",
			url:      "not-a-valid-url",
			expected: "US", // 默认美国
		},
		{
			name:     "未知域名",
			url:      "https://www.example.com/product",
			expected: "US", // 默认美国
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helper.GetMarketplaceFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("GetMarketplaceFromURL(%q) = %q, 期望 %q", tt.url, result, tt.expected)
			}
		})
	}
}

// TestExtractASINFromURL 测试从URL提取ASIN
func TestExtractASINFromURL(t *testing.T) {
	helper := NewURLHelper()

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "标准dp格式",
			url:      "https://www.amazon.com/dp/B08N5WRWNW",
			expected: "B08N5WRWNW",
		},
		{
			name:     "带产品名称的dp格式",
			url:      "https://www.amazon.com/Product-Name/dp/B08N5WRWNW",
			expected: "B08N5WRWNW",
		},
		{
			name:     "gp/product格式",
			url:      "https://www.amazon.com/gp/product/B08N5WRWNW",
			expected: "B08N5WRWNW",
		},
		{
			name:     "带查询参数",
			url:      "https://www.amazon.com/dp/B08N5WRWNW?ref=xxx",
			expected: "B08N5WRWNW",
		},
		{
			name:     "带锚点",
			url:      "https://www.amazon.com/dp/B08N5WRWNW#reviews",
			expected: "B08N5WRWNW",
		},
		{
			name:     "复杂URL",
			url:      "https://www.amazon.com/Some-Product-Name/dp/B08N5WRWNW/ref=sr_1_1?keywords=test",
			expected: "B08N5WRWNW",
		},
		{
			name:     "无ASIN",
			url:      "https://www.amazon.com/",
			expected: "",
		},
		{
			name:     "无效ASIN长度",
			url:      "https://www.amazon.com/dp/B08N5",
			expected: "",
		},
		{
			name:     "空URL",
			url:      "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helper.ExtractASINFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractASINFromURL(%q) = %q, 期望 %q", tt.url, result, tt.expected)
			}
		})
	}
}

// TestIsValidAmazonURL 测试Amazon URL验证
func TestIsValidAmazonURL(t *testing.T) {
	helper := NewURLHelper()

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "有效的amazon.com URL",
			url:      "https://www.amazon.com/dp/B08N5WRWNW",
			expected: true,
		},
		{
			name:     "有效的amazon.co.uk URL",
			url:      "https://www.amazon.co.uk/dp/B08N5WRWNW",
			expected: true,
		},
		{
			name:     "有效的amazon.de URL",
			url:      "https://www.amazon.de/dp/B08N5WRWNW",
			expected: true,
		},
		{
			name:     "无效的非Amazon URL",
			url:      "https://www.example.com/product",
			expected: false,
		},
		{
			name:     "空URL",
			url:      "",
			expected: false,
		},
		{
			name:     "无效的URL格式",
			url:      "not-a-valid-url",
			expected: false,
		},
		{
			name:     "HTTP协议",
			url:      "http://www.amazon.com/dp/B08N5WRWNW",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helper.IsValidAmazonURL(tt.url)
			if result != tt.expected {
				t.Errorf("IsValidAmazonURL(%q) = %v, 期望 %v", tt.url, result, tt.expected)
			}
		})
	}
}

// TestNormalizeURL 测试URL标准化
func TestNormalizeURL(t *testing.T) {
	helper := NewURLHelper()

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "标准化带产品名称的URL",
			url:      "https://www.amazon.com/Product-Name/dp/B08N5WRWNW/ref=xxx",
			expected: "https://www.amazon.com/dp/B08N5WRWNW",
		},
		{
			name:     "标准化带查询参数的URL",
			url:      "https://www.amazon.com/dp/B08N5WRWNW?ref=xxx&keywords=test",
			expected: "https://www.amazon.com/dp/B08N5WRWNW",
		},
		{
			name:     "标准化gp/product格式",
			url:      "https://www.amazon.com/gp/product/B08N5WRWNW",
			expected: "https://www.amazon.com/dp/B08N5WRWNW",
		},
		{
			name:     "已经标准化的URL",
			url:      "https://www.amazon.com/dp/B08N5WRWNW",
			expected: "https://www.amazon.com/dp/B08N5WRWNW",
		},
		{
			name:     "非Amazon URL保持不变",
			url:      "https://www.example.com/product",
			expected: "https://www.example.com/product",
		},
		{
			name:     "无ASIN的URL保持不变",
			url:      "https://www.amazon.com/",
			expected: "https://www.amazon.com/",
		},
		{
			name:     "无效URL保持不变",
			url:      "not-a-valid-url",
			expected: "not-a-valid-url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helper.NormalizeURL(tt.url)
			if result != tt.expected {
				t.Errorf("NormalizeURL(%q) = %q, 期望 %q", tt.url, result, tt.expected)
			}
		})
	}
}

// TestAddLanguageParam 测试添加语言参数
func TestAddLanguageParam(t *testing.T) {
	helper := NewURLHelper()

	tests := []struct {
		name     string
		url      string
		contains string // 期望包含的字符串
	}{
		{
			name:     "添加语言参数",
			url:      "https://www.amazon.com/dp/B08N5WRWNW",
			contains: "language=en_US",
		},
		{
			name:     "已有语言参数不重复添加",
			url:      "https://www.amazon.com/dp/B08N5WRWNW?language=de_DE",
			contains: "language=de_DE",
		},
		{
			name:     "带其他参数",
			url:      "https://www.amazon.com/dp/B08N5WRWNW?ref=xxx",
			contains: "language=en_US",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helper.AddLanguageParam(tt.url)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("AddLanguageParam(%q) = %q, 应该包含 %q", tt.url, result, tt.contains)
			}
		})
	}
}

// TestMarketplaceInfoMap 测试市场信息映射表的完整性
func TestMarketplaceInfoMap(t *testing.T) {
	// 验证映射表包含所有15个市场
	expectedMarkets := []string{
		"amazon.com", "amazon.co.uk", "amazon.de", "amazon.fr", "amazon.it",
		"amazon.es", "amazon.ca", "amazon.co.jp", "amazon.com.au", "amazon.in",
		"amazon.com.br", "amazon.com.mx", "amazon.nl", "amazon.se", "amazon.pl",
	}

	for _, market := range expectedMarkets {
		if _, exists := marketplaceInfoMap[market]; !exists {
			t.Errorf("marketplaceInfoMap 缺少市场: %s", market)
		}
	}

	// 验证映射表的大小
	if len(marketplaceInfoMap) != 15 {
		t.Errorf("marketplaceInfoMap 大小 = %d, 期望 15", len(marketplaceInfoMap))
	}

	// 验证每个市场都有货币和市场代码
	for domain, info := range marketplaceInfoMap {
		if info.Marketplace == "" {
			t.Errorf("市场 %s 缺少 Marketplace 代码", domain)
		}
		if info.Currency == "" {
			t.Errorf("市场 %s 缺少 Currency 代码", domain)
		}
	}
}

// BenchmarkGetCurrencyFromURL 性能基准测试
func BenchmarkGetCurrencyFromURL(b *testing.B) {
	helper := NewURLHelper()
	url := "https://www.amazon.com/dp/B08N5WRWNW"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		helper.GetCurrencyFromURL(url)
	}
}

// BenchmarkExtractASINFromURL 性能基准测试
func BenchmarkExtractASINFromURL(b *testing.B) {
	helper := NewURLHelper()
	url := "https://www.amazon.com/Product-Name/dp/B08N5WRWNW/ref=sr_1_1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		helper.ExtractASINFromURL(url)
	}
}

// BenchmarkNormalizeURL 性能基准测试
func BenchmarkNormalizeURL(b *testing.B) {
	helper := NewURLHelper()
	url := "https://www.amazon.com/Product-Name/dp/B08N5WRWNW/ref=sr_1_1?keywords=test"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		helper.NormalizeURL(url)
	}
}
