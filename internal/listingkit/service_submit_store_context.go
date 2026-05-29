package listingkit

import (
	"context"
	"strings"

	sheinwarehouse "task-processor/internal/shein/api/warehouse"
)

func (s *service) resolveSheinSubmitSettings(ctx context.Context, task *Task) SheinSettings {
	return buildSubmitRuntimeContextResolver(s).resolveSubmitSettings(ctx, task)
}

func (s *service) resolveSheinWarehouseCode(ctx context.Context, task *Task, site string) string {
	return buildSubmitRuntimeContextResolver(s).resolveWarehouseCode(ctx, task, site)
}

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
