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

// domainMap 地区到域名的映射
var domainMap = map[string]string{
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
	"br": "amazon.com.br",
	"in": "amazon.in",
	"ae": "amazon.ae",
	"sa": "amazon.sa",
}

// zipcodeMap 地区到默认邮编的映射
var zipcodeMap = map[string]string{
	"us": "94107",     // 旧金山（默认）
	"uk": "SW1A 1AA",  // 伦敦
	"de": "10115",     // 柏林
	"fr": "75001",     // 巴黎
	"jp": "153-0064",  // 东京
	"ca": "M5H 2N2",   // 多伦多
	"it": "00118",     // 罗马
	"es": "28001",     // 马德里
	"in": "110001",    // 新德里
	"mx": "11000",     // 墨西哥城
	"br": "01310-100", // 圣保罗
	"au": "2000",      // 悉尼
	"ae": "00000",     // 迪拜
	"sa": "11564",     // 利雅得
}

// languageMap 地区到语言代码的映射
var languageMap = map[string]string{
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
	"br": "pt_BR",
	"in": "en_IN",
	"ae": "en_AE",
	"sa": "en_AE",
}

// GetAmazonDomainByRegion 根据地区获取Amazon域名
func (r *DomainResolver) GetAmazonDomainByRegion(region string) string {
	if domain, exists := domainMap[strings.ToLower(region)]; exists {
		return domain
	}
	return "amazon.com" // 默认美国站
}

// GetLanguageByRegion 根据地区获取语言代码
func (r *DomainResolver) GetLanguageByRegion(region string) string {
	if language, exists := languageMap[strings.ToLower(region)]; exists {
		return language
	}
	return "en_US" // 默认英语(美国)
}

// GetZipcodeByRegion 根据地区获取默认邮编
func (r *DomainResolver) GetZipcodeByRegion(region string) string {
	if zipcode, exists := zipcodeMap[strings.ToLower(region)]; exists {
		return zipcode
	}
	return "94107" // 默认使用美国邮编
}

// ShouldUseDefaultZipcode 判断该地区是否应自动补默认邮编。
// 美国站默认不主动补邮编，只有显式指定时才设置。
func (r *DomainResolver) ShouldUseDefaultZipcode(region string) bool {
	return shouldUseDefaultZipcode(region)
}

// BuildAmazonProductURL 构建Amazon产品URL
func (r *DomainResolver) BuildAmazonProductURL(region, asin string) string {
	domain := r.GetAmazonDomainByRegion(region)
	language := r.GetLanguageByRegion(region)
	return "https://www." + domain + "/dp/" + asin + "?th=1&psc=1&language=" + language
}

// GetDomainMap 获取地区到域名的映射（兼容旧代码）
func GetDomainMap() map[string]string {
	result := make(map[string]string, len(domainMap))
	for k, v := range domainMap {
		result[k] = v
	}
	return result
}

// GetZipcodeMap 获取地区到默认邮编的映射（兼容旧代码）
func GetZipcodeMap() map[string]string {
	result := make(map[string]string, len(zipcodeMap))
	for k, v := range zipcodeMap {
		result[k] = v
	}
	return result
}

// GetDefaultDomain 获取默认域名（兼容旧代码）
func GetDefaultDomain(region string) string {
	resolver := &DomainResolver{}
	return resolver.GetAmazonDomainByRegion(region)
}

// GetDefaultZipcode 获取默认邮编（兼容旧代码）
func GetDefaultZipcode(region string) string {
	resolver := &DomainResolver{}
	return resolver.GetZipcodeByRegion(region)
}

// ExtractRegionFromURL 从URL中提取地区代码
func (r *DomainResolver) ExtractRegionFromURL(url string) string {
	url = strings.ToLower(url)

	// 查找匹配的域名
	for region, domain := range domainMap {
		if strings.Contains(url, domain) {
			return region
		}
	}

	return ""
}

func shouldUseDefaultZipcode(region string) bool {
	region = strings.ToLower(strings.TrimSpace(region))
	return region != "" && region != "us"
}
