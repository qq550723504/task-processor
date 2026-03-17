package store

import "task-processor/internal/shein/api/product"

// getSiteListByRegion 根据区域获取站点列表
func GetSiteListByRegion(region string) []product.SiteInfo {
	// 根据不同的区域返回不同的站点配置
	switch region {
	case "US":
		return []product.SiteInfo{
			{
				MainSite:    "shein",
				SubSiteList: []string{"shein-us"},
			},
		}
	case "FR":
		return []product.SiteInfo{
			{
				MainSite:    "shein",
				SubSiteList: []string{"shein-fr"},
			},
		}
	case "DE":
		return []product.SiteInfo{
			{
				MainSite:    "shein",
				SubSiteList: []string{"shein-de"},
			},
		}
	case "IT":
		return []product.SiteInfo{
			{
				MainSite:    "shein",
				SubSiteList: []string{"shein-it"},
			},
		}
	case "ES":
		return []product.SiteInfo{
			{
				MainSite:    "shein",
				SubSiteList: []string{"shein-es"},
			},
		}
	case "UK":
		return []product.SiteInfo{
			{
				MainSite:    "shein",
				SubSiteList: []string{"shein-uk"},
			},
		}
	case "AU":
		return []product.SiteInfo{
			{
				MainSite:    "shein",
				SubSiteList: []string{"shein-au"},
			},
		}
	case "JP":
		return []product.SiteInfo{
			{
				MainSite:    "shein",
				SubSiteList: []string{"shein-jp"},
			},
		}
	case "MX":
		return []product.SiteInfo{
			{
				MainSite:    "shein",
				SubSiteList: []string{"shein-mx"},
			},
		}
	case "SA":
		return []product.SiteInfo{
			{
				MainSite:    "shein",
				SubSiteList: []string{"shein-sa"},
			},
		}
	case "AE":
		return []product.SiteInfo{
			{
				MainSite:    "shein",
				SubSiteList: []string{"shein-ae"},
			},
		}
	default:
		// 默认返回US站点配置
		return []product.SiteInfo{
			{
				MainSite:    "shein",
				SubSiteList: []string{"shein-us"},
			},
		}
	}
}

// GetCurrencyByRegion 根据区域获取货币
func GetCurrencyByRegion(region string) string {
	switch region {
	case "US":
		return "USD"
	case "FR":
		return "EUR"
	case "DE":
		return "EUR"
	case "IT":
		return "EUR"
	case "ES":
		return "EUR"
	case "UK":
		return "GBP"
	case "AU":
		return "AUD"
	case "JP":
		return "JPY"
	case "CA":
		return "CAD"
	case "MX":
		return "MXN"
	case "SA":
		return "SAR"
	case "AE":
		return "AED"
	default:
		return "USD"
	}
}

// GetAmazonDomainByRegion 根据区域获取Amazon域名
func GetAmazonDomainByRegion(region string) string {
	switch region {
	case "US":
		return "amazon.com"
	case "FR":
		return "amazon.fr"
	case "DE":
		return "amazon.de"
	case "IT":
		return "amazon.it"
	case "ES":
		return "amazon.es"
	case "UK":
		return "amazon.co.uk"
	case "AU":
		return "amazon.com.au"
	case "JP":
		return "amazon.co.jp"
	case "CA":
		return "amazon.ca"
	case "MX":
		return "amazon.com.mx"
	case "SA":
		return "amazon.sa"
	case "AE":
		return "amazon.ae"
	default:
		return "amazon.com"
	}
}

// GetDefaultZipcodeByRegion 根据区域获取默认邮编（内置默认值）
func GetDefaultZipcodeByRegion(region string) string {
	switch region {
	case "US":
		return "94107" // 美国加州旧金山
	case "FR":
		return "75001" // 法国巴黎
	case "DE":
		return "10115" // 德国柏林
	case "IT":
		return "00118" // 意大利罗马
	case "ES":
		return "28001" // 西班牙马德里
	case "UK":
		return "SW1A 1AA" // 英国伦敦
	case "AU":
		return "2000" // 澳大利亚悉尼
	case "JP":
		return "153-0064" // 日本东京
	case "CA":
		return "M5V 3A8" // 加拿大多伦多
	case "MX":
		return "06600" // 墨西哥墨西哥城
	case "SA":
		return "11564" // 沙特利雅得
	case "AE":
		return "00000" // 阿联酋迪拜
	default:
		return "94107"
	}
}

// GetZipcodeForRegion 根据区域获取邮编，优先使用配置中的邮编映射
// 参数：
//   - region: 地区代码（如 "US", "IT"）
//   - configZipcodes: 配置文件中的邮编映射
//
// 返回：该地区应使用的邮编
func GetZipcodeForRegion(region string, configZipcodes map[string]string) string {
	// 1. 优先使用配置中的地区邮编映射
	if configZipcodes != nil {
		if zipcode, ok := configZipcodes[region]; ok && zipcode != "" {
			return zipcode
		}
	}

	// 2. 使用内置的地区默认邮编
	return GetDefaultZipcodeByRegion(region)
}
