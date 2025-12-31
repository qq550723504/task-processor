// Package amazon 提供Amazon URL处理辅助功能
package amazon

import (
	"net/url"
	"regexp"
	"strings"
)

// URLHelper URL处理辅助工具
type URLHelper struct{}

// NewURLHelper 创建URL辅助工具
func NewURLHelper() *URLHelper {
	return &URLHelper{}
}

// AddLanguageParam 为URL添加语言参数
func (uh *URLHelper) AddLanguageParam(originalURL string) string {
	if strings.Contains(originalURL, "language=") {
		return originalURL
	}

	parsedURL, err := url.Parse(originalURL)
	if err != nil {
		return originalURL
	}

	query := parsedURL.Query()
	query.Set("language", "en_US")
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String()
}

// ExtractASINFromURL 从URL中提取ASIN
func (uh *URLHelper) ExtractASINFromURL(productURL string) string {
	// 匹配Amazon ASIN的正则表达式
	asinRegex := regexp.MustCompile(`/([A-Z0-9]{10})(?:[/?]|$)`)
	matches := asinRegex.FindStringSubmatch(productURL)
	if len(matches) > 1 {
		return matches[1]
	}

	// 尝试从dp/后面提取
	dpRegex := regexp.MustCompile(`/dp/([A-Z0-9]{10})`)
	matches = dpRegex.FindStringSubmatch(productURL)
	if len(matches) > 1 {
		return matches[1]
	}

	// 尝试从gp/product/后面提取
	gpRegex := regexp.MustCompile(`/gp/product/([A-Z0-9]{10})`)
	matches = gpRegex.FindStringSubmatch(productURL)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// GetCurrencyFromURL 从URL中获取货币信息
func (uh *URLHelper) GetCurrencyFromURL(productURL string) string {
	parsedURL, err := url.Parse(productURL)
	if err != nil {
		return "USD" // 默认美元
	}

	host := strings.ToLower(parsedURL.Host)

	// 根据域名判断货币
	switch {
	case strings.Contains(host, "amazon.com"):
		return "USD"
	case strings.Contains(host, "amazon.co.uk"):
		return "GBP"
	case strings.Contains(host, "amazon.de"):
		return "EUR"
	case strings.Contains(host, "amazon.fr"):
		return "EUR"
	case strings.Contains(host, "amazon.it"):
		return "EUR"
	case strings.Contains(host, "amazon.es"):
		return "EUR"
	case strings.Contains(host, "amazon.ca"):
		return "CAD"
	case strings.Contains(host, "amazon.co.jp"):
		return "JPY"
	case strings.Contains(host, "amazon.com.au"):
		return "AUD"
	case strings.Contains(host, "amazon.in"):
		return "INR"
	case strings.Contains(host, "amazon.com.br"):
		return "BRL"
	case strings.Contains(host, "amazon.com.mx"):
		return "MXN"
	case strings.Contains(host, "amazon.nl"):
		return "EUR"
	case strings.Contains(host, "amazon.se"):
		return "SEK"
	case strings.Contains(host, "amazon.pl"):
		return "PLN"
	default:
		return "USD" // 默认美元
	}
}

// GetMarketplaceFromURL 从URL中获取市场信息
func (uh *URLHelper) GetMarketplaceFromURL(productURL string) string {
	parsedURL, err := url.Parse(productURL)
	if err != nil {
		return "US" // 默认美国
	}

	host := strings.ToLower(parsedURL.Host)

	// 根据域名判断市场
	switch {
	case strings.Contains(host, "amazon.com"):
		return "US"
	case strings.Contains(host, "amazon.co.uk"):
		return "UK"
	case strings.Contains(host, "amazon.de"):
		return "DE"
	case strings.Contains(host, "amazon.fr"):
		return "FR"
	case strings.Contains(host, "amazon.it"):
		return "IT"
	case strings.Contains(host, "amazon.es"):
		return "ES"
	case strings.Contains(host, "amazon.ca"):
		return "CA"
	case strings.Contains(host, "amazon.co.jp"):
		return "JP"
	case strings.Contains(host, "amazon.com.au"):
		return "AU"
	case strings.Contains(host, "amazon.in"):
		return "IN"
	case strings.Contains(host, "amazon.com.br"):
		return "BR"
	case strings.Contains(host, "amazon.com.mx"):
		return "MX"
	case strings.Contains(host, "amazon.nl"):
		return "NL"
	case strings.Contains(host, "amazon.se"):
		return "SE"
	case strings.Contains(host, "amazon.pl"):
		return "PL"
	default:
		return "US" // 默认美国
	}
}

// IsValidAmazonURL 检查是否为有效的Amazon URL
func (uh *URLHelper) IsValidAmazonURL(productURL string) bool {
	if productURL == "" {
		return false
	}

	parsedURL, err := url.Parse(productURL)
	if err != nil {
		return false
	}

	host := strings.ToLower(parsedURL.Host)
	return strings.Contains(host, "amazon.")
}

// NormalizeURL 标准化Amazon URL
func (uh *URLHelper) NormalizeURL(productURL string) string {
	if !uh.IsValidAmazonURL(productURL) {
		return productURL
	}

	asin := uh.ExtractASINFromURL(productURL)
	if asin == "" {
		return productURL
	}

	parsedURL, err := url.Parse(productURL)
	if err != nil {
		return productURL
	}

	// 构建标准化的URL
	normalizedURL := parsedURL.Scheme + "://" + parsedURL.Host + "/dp/" + asin
	return normalizedURL
}
