// Package product 提供Amazon域名解析功能
package product

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
		"us":                   "amazon.com",
		"usa":                  "amazon.com",
		"united states":        "amazon.com",
		"uk":                   "amazon.co.uk",
		"gb":                   "amazon.co.uk",
		"united kingdom":       "amazon.co.uk",
		"de":                   "amazon.de",
		"germany":              "amazon.de",
		"fr":                   "amazon.fr",
		"france":               "amazon.fr",
		"it":                   "amazon.it",
		"italy":                "amazon.it",
		"es":                   "amazon.es",
		"spain":                "amazon.es",
		"ca":                   "amazon.ca",
		"canada":               "amazon.ca",
		"jp":                   "amazon.co.jp",
		"japan":                "amazon.co.jp",
		"au":                   "amazon.com.au",
		"australia":            "amazon.com.au",
		"mx":                   "amazon.com.mx",
		"mexico":               "amazon.com.mx",
		"ae":                   "amazon.ae",
		"uae":                  "amazon.ae",
		"united arab emirates": "amazon.ae",
		"sa":                   "amazon.sa",
		"saudi":                "amazon.sa",
		"saudi arabia":         "amazon.sa",
	}

	if domain, exists := domainMap[strings.ToLower(region)]; exists {
		return domain
	}

	return "amazon.com" // 默认美国站
}

// GetRegionByDomain 根据域名获取地区代码
func (r *DomainResolver) GetRegionByDomain(domain string) string {
	regionMap := map[string]string{
		"amazon.com":    "us",
		"amazon.co.uk":  "uk",
		"amazon.de":     "de",
		"amazon.fr":     "fr",
		"amazon.it":     "it",
		"amazon.es":     "es",
		"amazon.ca":     "ca",
		"amazon.co.jp":  "jp",
		"amazon.com.au": "au",
		"amazon.com.mx": "mx",
		"amazon.ae":     "ae",
		"amazon.sa":     "sa",
	}

	if region, exists := regionMap[strings.ToLower(domain)]; exists {
		return region
	}

	return "us" // 默认美国
}

// IsValidAmazonDomain 检查是否为有效的Amazon域名
func (r *DomainResolver) IsValidAmazonDomain(domain string) bool {
	validDomains := []string{
		"amazon.com", "amazon.co.uk", "amazon.de", "amazon.fr",
		"amazon.it", "amazon.es", "amazon.ca", "amazon.co.jp",
		"amazon.com.au", "amazon.com.mx", "amazon.ae", "amazon.sa",
	}

	lowerDomain := strings.ToLower(domain)
	for _, validDomain := range validDomains {
		if lowerDomain == validDomain {
			return true
		}
	}

	return false
}

// GetSupportedRegions 获取支持的地区列表
func (r *DomainResolver) GetSupportedRegions() []string {
	return []string{
		"us", "uk", "de", "fr", "it", "es",
		"ca", "jp", "au", "mx", "ae", "sa",
	}
}

// GetRegionDisplayName 获取地区显示名称
func (r *DomainResolver) GetRegionDisplayName(region string) string {
	displayNames := map[string]string{
		"us": "United States",
		"uk": "United Kingdom",
		"de": "Germany",
		"fr": "France",
		"it": "Italy",
		"es": "Spain",
		"ca": "Canada",
		"jp": "Japan",
		"au": "Australia",
		"mx": "Mexico",
		"ae": "United Arab Emirates",
		"sa": "Saudi Arabia",
	}

	if displayName, exists := displayNames[strings.ToLower(region)]; exists {
		return displayName
	}

	return region
}
