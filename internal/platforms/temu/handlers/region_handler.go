package handlers

// RegionHandler 地区处理器
type RegionHandler struct{}

// NewRegionHandler 创建新的地区处理器
func NewRegionHandler() *RegionHandler {
	return &RegionHandler{}
}

// GetLanguageByRegion 根据地区获取语言
func (rh *RegionHandler) GetLanguageByRegion(region string) string {
	regionLangMap := map[string]string{
		"US": "en",
		"UK": "en",
		"CA": "en",
		"AU": "en",
		"DE": "de",
		"FR": "fr",
		"ES": "es",
		"IT": "it",
		"JP": "ja",
		"BR": "pt",
		"MX": "es",
		"IN": "en",
		"SA": "ar",
		"AE": "ar",
	}

	if lang, exists := regionLangMap[region]; exists {
		return lang
	}

	return "en" // 默认英语
}

// GetAllowSitesByRegion 根据地区获取允许的站点
func (rh *RegionHandler) GetAllowSitesByRegion(region string) []int {
	regionSiteMap := map[string][]int{
		"US": {100},
	}

	if sites, exists := regionSiteMap[region]; exists {
		return sites
	}

	return []int{1} // 默认美国站点
}

// GetOriginByRegion 根据地区获取原产地
func (rh *RegionHandler) GetOriginByRegion(region string) string {
	// 根据目标地区设置合适的原产地
	originMap := map[string]string{
		"US": "United States",
		"UK": "United Kingdom",
		"CA": "Canada",
		"AU": "Australia",
		"DE": "Germany",
		"FR": "France",
		"ES": "Spain",
		"IT": "Italy",
		"JP": "Japan",
		"BR": "Brazil",
		"MX": "Mexico",
		"IN": "India",
		"SA": "Saudi Arabia",
		"AE": "United Arab Emirates",
	}

	if origin, exists := originMap[region]; exists {
		return origin
	}

	return "China" // 默认中国制造
}

// GetCurrencyByRegion 根据地区获取货币
func (rh *RegionHandler) GetCurrencyByRegion(region string) string {
	currencyMap := map[string]string{
		"US": "USD",
		"UK": "GBP",
		"CA": "CAD",
		"AU": "AUD",
		"DE": "EUR",
		"FR": "EUR",
		"ES": "EUR",
		"IT": "EUR",
		"JP": "JPY",
		"BR": "BRL",
		"MX": "MXN",
		"IN": "INR",
		"SA": "SAR",
		"AE": "AED",
	}

	if currency, exists := currencyMap[region]; exists {
		return currency
	}

	return "USD" // 默认美元
}
