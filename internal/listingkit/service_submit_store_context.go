package listingkit

import (
	"context"
	"strings"

	sheinwarehouse "task-processor/internal/shein/api/warehouse"
	sheinclient "task-processor/internal/shein/client"
)

func (s *service) resolveSheinSubmitSettings(ctx context.Context, task *Task) SheinSettings {
	settings := s.currentSheinSubmitSettings()
	if profile, err := s.resolveSheinStoreProfile(ctx, task); err == nil && profile != nil {
		settings = applySubmitSettingsProfile(settings, profile)
	}
	settings = applySubmitSettingsTaskRequest(settings, task)
	if task == nil {
		return settings
	}
	return applySubmitWarehouseOverride(settings, s.resolveSheinWarehouseCode(ctx, task, settings.Site))
}

func (s *service) resolveSheinWarehouseCode(ctx context.Context, task *Task, site string) string {
	if s == nil || s.sheinStoreCatalog == nil || s.sheinAPIClientFactory == nil || task == nil {
		return ""
	}
	apiClient, storeID, err := s.newSheinAPIClient(ctx, task)
	if err != nil {
		return ""
	}
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
