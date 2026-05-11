package listingkit

import (
	"strings"

	sheinwarehouse "task-processor/internal/shein/api/warehouse"
	sheinclient "task-processor/internal/shein/client"
)

func (s *service) resolveSheinSubmitSettings(task *Task) SheinSettings {
	settings := s.currentSheinSubmitSettings()
	if task != nil && task.Request != nil {
		if country := strings.ToUpper(strings.TrimSpace(task.Request.Country)); country != "" {
			settings.Site = country
		}
	}
	storeID := s.resolveSheinStoreID(task)
	if storeID <= 0 {
		return settings
	}
	if warehouseCode := s.resolveSheinWarehouseCode(storeID, settings.Site); warehouseCode != "" {
		settings.WarehouseCode = warehouseCode
	}
	return settings
}

func (s *service) resolveSheinWarehouseCode(storeID int64, site string) string {
	if s == nil || s.sheinManagementClient == nil || storeID <= 0 {
		return ""
	}
	apiClient := sheinclient.NewAPIClient(storeID, s.sheinManagementClient)
	if !apiClient.HasCookies() {
		if err := apiClient.ForceRefreshCookies(); err != nil {
			return ""
		}
	}
	if !apiClient.HasCookies() {
		return ""
	}
	baseAPI := sheinclient.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		storeID,
		apiClient.GetHTTPClient(),
	)
	baseAPI.SetAuthRefreshFunc(apiClient.ForceRefreshCookies)
	warehouseAPI := sheinwarehouse.NewClient(baseAPI)
	warehouses, err := warehouseAPI.GetWarehouses()
	if err != nil || warehouses == nil {
		return ""
	}
	return pickSheinWarehouseCode(warehouses, site)
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
