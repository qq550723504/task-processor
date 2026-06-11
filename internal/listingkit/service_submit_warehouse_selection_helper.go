package listingkit

import (
	"strings"

	sheinwarehouse "task-processor/internal/shein/api/warehouse"
)

func pickSheinWarehouseCode(warehouses *sheinwarehouse.WarehouseResponse, site string) string {
	if warehouses == nil || len(warehouses.Data) == 0 {
		return ""
	}
	target := strings.ToUpper(strings.TrimSpace(site))
	if target != "" {
		for _, warehouse := range warehouses.Data {
			for _, country := range warehouse.SaleCountryList {
				if strings.EqualFold(strings.TrimSpace(country), target) {
					return strings.TrimSpace(warehouse.WarehouseCode)
				}
			}
		}
	}
	return strings.TrimSpace(warehouses.Data[0].WarehouseCode)
}
