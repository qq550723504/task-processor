// Package utils 提供工具方法
package utils

// GetDomainMap 获取地区到域名的映射
func GetDomainMap() map[string]string {
	return map[string]string{
		"us": "amazon.com",
		"uk": "amazon.co.uk",
		"de": "amazon.de",
		"fr": "amazon.fr",
		"jp": "amazon.co.jp",
		"ca": "amazon.ca",
		"it": "amazon.it",
		"es": "amazon.es",
		"in": "amazon.in",
		"mx": "amazon.com.mx",
		"br": "amazon.com.br",
		"au": "amazon.com.au",
	}
}

// GetZipcodeMap 获取地区到默认邮编的映射
func GetZipcodeMap() map[string]string {
	return map[string]string{
		"us": "94107",     // 旧金山
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
	}
}

// GetDefaultDomain 获取默认域名
func GetDefaultDomain(region string) string {
	domainMap := GetDomainMap()
	if domain, exists := domainMap[region]; exists {
		return domain
	}
	return "amazon.com" // 默认使用美国站
}

// GetDefaultZipcode 获取默认邮编
func GetDefaultZipcode(region string) string {
	zipcodeMap := GetZipcodeMap()
	if zipcode, exists := zipcodeMap[region]; exists {
		return zipcode
	}
	return "94107" // 默认使用美国邮编
}
