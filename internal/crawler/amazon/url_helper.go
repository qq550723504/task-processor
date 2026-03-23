// Package amazon 提供Amazon URL处理辅助功能
package amazon

import (
	"net/url"
	"regexp"
	"strings"
)

// MarketplaceInfo 市场信息
type MarketplaceInfo struct {
	Marketplace string
	Currency    string
}

// marketplaceInfoMap 域名到市场信息的映射表
var marketplaceInfoMap = map[string]MarketplaceInfo{
	"amazon.com":    {Marketplace: "US", Currency: "USD"},
	"amazon.co.uk":  {Marketplace: "UK", Currency: "GBP"},
	"amazon.de":     {Marketplace: "DE", Currency: "EUR"},
	"amazon.fr":     {Marketplace: "FR", Currency: "EUR"},
	"amazon.it":     {Marketplace: "IT", Currency: "EUR"},
	"amazon.es":     {Marketplace: "ES", Currency: "EUR"},
	"amazon.ca":     {Marketplace: "CA", Currency: "CAD"},
	"amazon.co.jp":  {Marketplace: "JP", Currency: "JPY"},
	"amazon.com.au": {Marketplace: "AU", Currency: "AUD"},
	"amazon.in":     {Marketplace: "IN", Currency: "INR"},
	"amazon.com.br": {Marketplace: "BR", Currency: "BRL"},
	"amazon.com.mx": {Marketplace: "MX", Currency: "MXN"},
	"amazon.nl":     {Marketplace: "NL", Currency: "EUR"},
	"amazon.se":     {Marketplace: "SE", Currency: "SEK"},
	"amazon.pl":     {Marketplace: "PL", Currency: "PLN"},
}

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

// getMarketplaceInfo 从URL中获取市场信息（内部方法）
func (uh *URLHelper) getMarketplaceInfo(productURL string) MarketplaceInfo {
	parsedURL, err := url.Parse(productURL)
	if err != nil {
		return MarketplaceInfo{Marketplace: "US", Currency: "USD"} // 默认美国/美元
	}

	host := strings.ToLower(parsedURL.Host)
	// 移除 www. 前缀
	host = strings.TrimPrefix(host, "www.")

	// 直接查找映射表
	if info, exists := marketplaceInfoMap[host]; exists {
		return info
	}

	// 默认返回美国/美元
	return MarketplaceInfo{Marketplace: "US", Currency: "USD"}
}

// GetCurrencyFromURL 从URL中获取货币信息
func (uh *URLHelper) GetCurrencyFromURL(productURL string) string {
	return uh.getMarketplaceInfo(productURL).Currency
}

// GetMarketplaceFromURL 从URL中获取市场信息
func (uh *URLHelper) GetMarketplaceFromURL(productURL string) string {
	return uh.getMarketplaceInfo(productURL).Marketplace
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
