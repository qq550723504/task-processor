// Package loaders 提供配置加载功能
package config

import (
	"github.com/spf13/viper"
)

// BuildPlatformConfig 构建单个平台配置
func BuildPlatformConfig(prefix string) PlatformConfig {
	return PlatformConfig{
		Enabled:          viper.GetBool(prefix + ".enabled"),
		SchedulerEnabled: viper.GetBool(prefix + ".schedulerEnabled"),
		AutoPricing: AutoPricingConfig{
			Enabled:        viper.GetBool(prefix + ".autoPricing.enabled"),
			Interval:       viper.GetInt(prefix + ".autoPricing.interval"),
			BatchSize:      viper.GetInt(prefix + ".autoPricing.batchSize"),
			UseAmazonPrice: viper.GetBool(prefix + ".autoPricing.useAmazonPrice"),
		},
		ProductSync: ScheduledTaskConfig{
			Enabled:  viper.GetBool(prefix + ".productSync.enabled"),
			Interval: viper.GetInt(prefix + ".productSync.interval"),
		},
		InventorySync: ScheduledTaskConfig{
			Enabled:  viper.GetBool(prefix + ".inventorySync.enabled"),
			Interval: viper.GetInt(prefix + ".inventorySync.interval"),
		},
		ActivityRegistration: ScheduledTaskConfig{
			Enabled:  viper.GetBool(prefix + ".activityRegistration.enabled"),
			Interval: viper.GetInt(prefix + ".activityRegistration.interval"),
		},
		SyncProduct: SyncProductConfig{
			Enabled:   viper.GetBool(prefix + ".sync.enabled"),
			StoreIDs:  getInt64Slice(prefix + ".sync.storeIDs"),
			Interval:  viper.GetInt(prefix + ".sync.interval"),
			BatchSize: viper.GetInt(prefix + ".sync.batchSize"),
		},
		Monitor: MonitorConfig{
			Enabled:              viper.GetBool(prefix + ".monitor.enabled"),
			StoreIDs:             getInt64Slice(prefix + ".monitor.storeIDs"),
			CheckInterval:        viper.GetInt(prefix + ".monitor.checkInterval"),
			BatchSize:            viper.GetInt(prefix + ".monitor.batchSize"),
			EnablePriceAlert:     viper.GetBool(prefix + ".monitor.enablePriceAlert"),
			EnableStockAlert:     viper.GetBool(prefix + ".monitor.enableStockAlert"),
			PriceChangeThreshold: viper.GetFloat64(prefix + ".monitor.priceChangeThreshold"),
			StockChangeThreshold: viper.GetInt(prefix + ".monitor.stockChangeThreshold"),
		},
	}
}
