// Package product 提供Amazon域名解析功能
package product

import "strings"

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
	"us": "94107",
	"uk": "SW1A 1AA",
	"de": "10115",
	"fr": "75001",
	"jp": "153-0064",
	"ca": "M5H 2N2",
	"it": "00118",
	"es": "28001",
	"in": "110001",
	"mx": "11000",
	"br": "01310-100",
	"au": "2000",
	"ae": "00000",
	"sa": "11564",
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

// DomainResolver Amazon域名解析器
type DomainResolver struct{}

// NewDomainResolver 创建域名解析器
func NewDomainResolver() *DomainResolver {
	return &DomainResolver{}
}

// GetAmazonDomainByRegion 根据地区获取Amazon域名
func (r *DomainResolver) GetAmazonDomainByRegion(region string) string {
	if domain, exists := domainMap[strings.ToLower(region)]; exists {
		return domain
	}
	return "amazon.com"
}

// GetLanguageByRegion 根据地区获取语言代码
func (r *DomainResolver) GetLanguageByRegion(region string) string {
	if language, exists := languageMap[strings.ToLower(region)]; exists {
		return language
	}
	return "en_US"
}

// GetZipcodeByRegion 根据地区获取默认邮编
func (r *DomainResolver) GetZipcodeByRegion(region string) string {
	if zipcode, exists := zipcodeMap[strings.ToLower(region)]; exists {
		return zipcode
	}
	return "94107"
}

// BuildAmazonProductURL 构建Amazon产品URL
func (r *DomainResolver) BuildAmazonProductURL(region, asin string) string {
	domain := r.GetAmazonDomainByRegion(region)
	language := r.GetLanguageByRegion(region)
	return "https://www." + domain + "/dp/" + asin + "?th=1&psc=1&language=" + language
}
