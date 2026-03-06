// Package amazon 提供Amazon域名解析功能
package amazon

import (
	"strings"
)

// DomainResolver Amazon域名解析器
type DomainResolver struct{}

// NewDomainResolver 创建域名解析器
func NewDomainResolver() *DomainResolver {
	return &DomainResolver{}
}

// GetAmazonDomainByRegion 根据地区获取Amazon域名
func (r *DomainResolver) GetAmazonDomainByRegion(region string) string {
	domainMap := map[string]string{
		"us": "amazon.com",
		"uk": "amazon.co.uk",
		"de": "amazon.de",
		"fr": "amazon.fr",
		"it": "amazon.it",
		"es": "amazon.es",
		"ca": "amazon.ca",
		"jp": "amazon.co.jp",
		"au": "amazon.com.au",
		"mx": "amazon.com.mx",
		"ae": "amazon.ae",
		"sa": "amazon.sa",
	}

	if domain, exists := domainMap[strings.ToLower(region)]; exists {
		return domain
	}

	return "amazon.com" // 默认美国站
}

// GetLanguageByRegion 根据地区获取语言代码
func (r *DomainResolver) GetLanguageByRegion(region string) string {
	languageMap := map[string]string{
		"us": "en_US",
		"uk": "en_GB",
		"de": "de_DE",
		"fr": "fr_FR",
		"it": "it_IT",
		"es": "es_ES",
		"ca": "en_CA",
		"jp": "ja_JP",
		"au": "en_AU",
		"mx": "es_MX",
		"ae": "en_AE",
		"sa": "en_AE",
	}

	if language, exists := languageMap[strings.ToLower(region)]; exists {
		return language
	}

	return "en_US" // 默认英语(美国)
}

// BuildAmazonProductURL 构建Amazon产品URL
func (r *DomainResolver) BuildAmazonProductURL(region, asin string) string {
	domain := r.GetAmazonDomainByRegion(region)
	language := r.GetLanguageByRegion(region)
	return "https://www." + domain + "/dp/" + asin + "?th=1&psc=1&language=" + language
}
