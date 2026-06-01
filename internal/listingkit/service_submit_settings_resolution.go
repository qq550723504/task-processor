package listingkit

import "strings"

func applySubmitSettingsProfile(settings SheinSettings, profile *ListingKitStoreProfile) SheinSettings {
	if profile == nil {
		return settings
	}
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
	return settings
}

func applySubmitSettingsTaskRequest(settings SheinSettings, task *Task) SheinSettings {
	if task == nil || task.Request == nil {
		return settings
	}
	if country := strings.ToUpper(strings.TrimSpace(task.Request.Country)); country != "" {
		settings.Site = country
	}
	return settings
}

func applySubmitWarehouseOverride(settings SheinSettings, warehouseCode string) SheinSettings {
	if strings.TrimSpace(warehouseCode) != "" {
		settings.WarehouseCode = strings.TrimSpace(warehouseCode)
	}
	return settings
}
