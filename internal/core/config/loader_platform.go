package config

import "github.com/spf13/viper"

func BuildPlatformConfig(v *viper.Viper, prefix string) PlatformConfig {
	cfg := PlatformConfig{
		Enabled:          v.GetBool(prefix + ".enabled"),
		SchedulerEnabled: v.GetBool(prefix + ".schedulerEnabled"),
		FetchMode:        v.GetString(prefix + ".fetchMode"),
		AutoPricing: AutoPricingConfig{
			Enabled:        v.GetBool(prefix + ".autoPricing.enabled"),
			Interval:       v.GetInt(prefix + ".autoPricing.interval"),
			BatchSize:      v.GetInt(prefix + ".autoPricing.batchSize"),
			UseAmazonPrice: v.GetBool(prefix + ".autoPricing.useAmazonPrice"),
		},
		ProductSync: ScheduledTaskConfig{
			Enabled:  v.GetBool(prefix + ".productSync.enabled"),
			Interval: v.GetInt(prefix + ".productSync.interval"),
		},
		InventorySync: ScheduledTaskConfig{
			Enabled:  v.GetBool(prefix + ".inventorySync.enabled"),
			Interval: v.GetInt(prefix + ".inventorySync.interval"),
		},
		ActivityRegistration: ScheduledTaskConfig{
			Enabled:  v.GetBool(prefix + ".activityRegistration.enabled"),
			Interval: v.GetInt(prefix + ".activityRegistration.interval"),
		},
		SyncProduct: SyncProductConfig{
			Enabled:   v.GetBool(prefix + ".sync.enabled"),
			StoreIDs:  getInt64Slice(v, prefix+".sync.storeIDs"),
			Interval:  v.GetInt(prefix + ".sync.interval"),
			BatchSize: v.GetInt(prefix + ".sync.batchSize"),
		},
		Monitor: MonitorConfig{
			Enabled:              v.GetBool(prefix + ".monitor.enabled"),
			StoreIDs:             getInt64Slice(v, prefix+".monitor.storeIDs"),
			CheckInterval:        v.GetInt(prefix + ".monitor.checkInterval"),
			BatchSize:            v.GetInt(prefix + ".monitor.batchSize"),
			EnablePriceAlert:     v.GetBool(prefix + ".monitor.enablePriceAlert"),
			EnableStockAlert:     v.GetBool(prefix + ".monitor.enableStockAlert"),
			PriceChangeThreshold: v.GetFloat64(prefix + ".monitor.priceChangeThreshold"),
			StockChangeThreshold: v.GetInt(prefix + ".monitor.stockChangeThreshold"),
		},
	}

	return normalizePlatformConfig(cfg)
}

func normalizePlatformConfig(cfg PlatformConfig) PlatformConfig {
	if !cfg.ProductSync.Enabled && cfg.SyncProduct.Enabled {
		cfg.ProductSync.Enabled = true
	}
	if cfg.ProductSync.Interval == 0 && cfg.SyncProduct.Interval > 0 {
		cfg.ProductSync.Interval = cfg.SyncProduct.Interval
	}

	return cfg
}
