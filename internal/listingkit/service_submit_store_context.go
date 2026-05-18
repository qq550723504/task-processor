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
		if profile.StoreID > 0 {
			settings.DefaultStoreID = profile.StoreID
		}
		if profile.Site != "" {
			settings.Site = profile.Site
		}
		if profile.WarehouseCode != "" {
			settings.WarehouseCode = profile.WarehouseCode
		}
		if profile.DefaultStock > 0 {
			settings.DefaultStock = profile.DefaultStock
		}
		if profile.DefaultSubmitMode != "" {
			settings.DefaultSubmitMode = profile.DefaultSubmitMode
		}
		settings.Pricing = normalizeSheinPricingRule(profile.Pricing, settings.Pricing)
	}
	if task != nil && task.Request != nil {
		if country := strings.ToUpper(strings.TrimSpace(task.Request.Country)); country != "" {
			settings.Site = country
		}
	}
	storeID, err := s.resolveSheinStoreID(ctx, task)
	if err != nil || storeID <= 0 {
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
