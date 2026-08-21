package listingkit

import (
	sheinpub "task-processor/internal/marketplace/shein/publishing"
	sheinwarehouse "task-processor/internal/shein/api/warehouse"
)

func pickSheinWarehouseCode(warehouses *sheinwarehouse.WarehouseResponse, site string) string {
	if warehouses == nil || len(warehouses.Data) == 0 {
		return ""
	}

	options := make([]sheinpub.WarehouseOption, 0, len(warehouses.Data))
	for _, warehouse := range warehouses.Data {
		options = append(options, sheinpub.WarehouseOption{
			Code:          warehouse.WarehouseCode,
			SaleCountries: warehouse.SaleCountryList,
		})
	}
	return sheinpub.SelectWarehouseCodeForSite(options, site)
}
