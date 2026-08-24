package common

import "strings"

// WarehouseOption contains the platform-neutral fields needed to select a
// warehouse for a sale country.
type WarehouseOption struct {
	Code          string
	SaleCountries []string
}

// SelectWarehouseCodeForSite returns the first warehouse that sells to site.
// When no warehouse matches, it preserves the legacy first-warehouse fallback.
func SelectWarehouseCodeForSite(warehouses []WarehouseOption, site string) string {
	if len(warehouses) == 0 {
		return ""
	}

	target := strings.ToUpper(strings.TrimSpace(site))
	if target != "" {
		for _, warehouse := range warehouses {
			for _, country := range warehouse.SaleCountries {
				if strings.EqualFold(strings.TrimSpace(country), target) {
					return strings.TrimSpace(warehouse.Code)
				}
			}
		}
	}

	return strings.TrimSpace(warehouses[0].Code)
}
