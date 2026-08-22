package listingkit

import (
	"task-processor/internal/publishing/common"
	sheinwarehouse "task-processor/internal/shein/api/warehouse"
)

func pickSheinWarehouseCode(warehouses *sheinwarehouse.WarehouseResponse, site string) string {
	if warehouses == nil || len(warehouses.Data) == 0 {
		return ""
	}

	options := make([]common.WarehouseOption, 0, len(warehouses.Data))
	for _, warehouse := range warehouses.Data {
		options = append(options, common.WarehouseOption{
			Code:          warehouse.WarehouseCode,
			SaleCountries: warehouse.SaleCountryList,
		})
	}
	return common.SelectWarehouseCodeForSite(options, site)
}
