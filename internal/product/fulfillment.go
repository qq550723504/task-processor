package product

import "strings"

// IsFBAFulfillment 判断是否为FBA配送
func IsFBAFulfillment(shipsFrom string) bool {
	if shipsFrom == "" {
		return false
	}
	return strings.Contains(shipsFrom, "Amazon") ||
		strings.Contains(shipsFrom, "amazon") ||
		strings.Contains(shipsFrom, "AMAZON")
}

// IsAMZSeller 判断是否为亚马逊自营
func IsAMZSeller(sellerName string) bool {
	if sellerName == "" {
		return false
	}
	return strings.Contains(sellerName, "Amazon") ||
		strings.Contains(sellerName, "amazon") ||
		strings.Contains(sellerName, "AMAZON")
}
