package browser

// GetCurrencyByRegion 根据地区获取货币代码
func GetCurrencyByRegion(region string) string {
	switch region {
	case "US":
		return "USD"
	case "FR", "DE", "IT", "ES":
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
	case "CN":
		return "CNY"
	default:
		return "USD"
	}
}
