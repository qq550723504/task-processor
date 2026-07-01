package listingadmin

import "strings"

func (r listingScheduledTaskConfig) toScheduledTaskConfig() ScheduledTaskConfig {
	return ScheduledTaskConfig{
		ID:              r.ID,
		TenantID:        r.TenantID,
		StoreID:         r.StoreID,
		Platform:        r.Platform,
		TaskType:        r.TaskType,
		Enabled:         r.Enabled != 0,
		IntervalSeconds: r.IntervalSeconds,
		Remark:          r.Remark,
		CreateTime:      r.CreateTime,
		UpdateTime:      r.UpdateTime,
	}
}

func listingScheduledTaskConfigFromScheduledTaskConfig(config *ScheduledTaskConfig) listingScheduledTaskConfig {
	if config == nil {
		return listingScheduledTaskConfig{}
	}
	return listingScheduledTaskConfig{
		ID:              config.ID,
		TenantID:        config.TenantID,
		StoreID:         config.StoreID,
		Platform:        normalizeScheduledTaskPlatform(config.Platform),
		TaskType:        normalizeScheduledTaskType(config.TaskType),
		Enabled:         boolToInt16(config.Enabled),
		IntervalSeconds: normalizeScheduledTaskInterval(config.IntervalSeconds),
		Remark:          strings.TrimSpace(config.Remark),
		CreateTime:      config.CreateTime,
		UpdateTime:      config.UpdateTime,
	}
}

func normalizeScheduledTaskPlatform(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeScheduledTaskType(value string) string {
	value = strings.TrimSpace(value)
	switch strings.ToLower(value) {
	case "inventory", "inventorysync", "inventory_sync":
		return "inventory"
	case "productsync", "product_sync":
		return "productSync"
	case "activity", "activityregistration", "activity_registration":
		return "activity"
	case "pricing", "autopricing", "auto_pricing":
		return "pricing"
	default:
		return value
	}
}

func normalizeScheduledTaskInterval(value int) int {
	if value <= 0 {
		return 3600
	}
	return value
}
