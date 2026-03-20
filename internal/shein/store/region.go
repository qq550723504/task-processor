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
	case "CA":
		return []product.SiteInfo{
			{
				MainSite:    "shein",
				SubSiteList: []string{"shein-ca"},
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
