// Package validation 提供通用的规则验证功能
package validation

import "regexp"

// IsFBAFulfillment 判断是否为FBA配送
// 通过检查 ships_from 字段是否包含 "Amazon" 关键词来判断
func IsFBAFulfillment(shipsFrom string) bool {
	if shipsFrom == "" {
		return false
	}

	// 支持多语言站点的 Amazon 关键词匹配
	amazonKeywords := []string{
		"Amazon",
		"amazon",
		"AMAZON",
	}

	for _, keyword := range amazonKeywords {
		if regexp.MustCompile(keyword).MatchString(shipsFrom) {
			return true
		}
	}

	return false
}

// IsAMZSeller 判断是否为亚马逊自营
// 通过检查 seller_name 字段是否包含 "Amazon" 关键词来判断
func IsAMZSeller(sellerName string) bool {
	if sellerName == "" {
		return false
	}

	// 支持多语言站点的 Amazon 卖家名称匹配
	amazonSellerKeywords := []string{
		"Amazon",
		"amazon",
		"AMAZON",
		"Amazon.com",
		"Amazon.co.jp",
		"Amazon.de",
		"Amazon.fr",
		"Amazon.co.uk",
		"Amazon.es",
		"Amazon.it",
		"Amazon.com.mx",
	}

	for _, keyword := range amazonSellerKeywords {
		if regexp.MustCompile(keyword).MatchString(sellerName) {
			return true
		}
	}

	return false
}
