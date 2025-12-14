// Package config 提供配置管理功能
package config

import "github.com/spf13/viper"

// setDefaults 设置默认配置值
func setDefaults() {
	// 处理器默认配置
	viper.SetDefault("processor.maxRetries", 3)
	viper.SetDefault("processor.timeout", 300)

	// 工作池默认配置
	viper.SetDefault("worker.concurrency", 10)
	viper.SetDefault("worker.bufferSize", 100)
	viper.SetDefault("worker.taskInterval", 60)
	viper.SetDefault("worker.maxFetchPerCycle", 5)
	viper.SetDefault("worker.queueThreshold", 75)

	// OpenAI默认配置
	viper.SetDefault("openai.apiKey", "sk-qns4hBrljHkJ520vwwA2508c7Dj3Oe0zGlh7oq7FWkcWXkx4")
	viper.SetDefault("openai.model", "gemini-2.0-flash")
	viper.SetDefault("openai.baseURL", "https://yunwu.ai/v1")
	viper.SetDefault("openai.timeout", 120)

	// 管理系统默认配置
	viper.SetDefault("management.baseURL", "http://getway.linkcloudai.com")
	viper.SetDefault("management.clientID", "go-listing")
	viper.SetDefault("management.clientSecret", "go-listing-secret")
	viper.SetDefault("management.tokenURL", "http://getway.linkcloudai.com/admin-api/system/oauth2/token")
	viper.SetDefault("management.scopes", []string{"user.read"})
	viper.SetDefault("management.tenantID", "1")

	// 自动核价默认配置
	setAutoPricingDefaults()

	// Amazon默认配置
	setAmazonDefaults()

	// 更新器默认配置
	setUpdaterDefaults()

	// 同步和监控默认配置
	setSyncMonitorDefaults()
}

// setAutoPricingDefaults 设置自动核价默认配置
func setAutoPricingDefaults() {
	// TEMU 自动核价默认配置
	viper.SetDefault("autoPricing.temu.enabled", true)
	viper.SetDefault("autoPricing.temu.interval", 300)
	viper.SetDefault("autoPricing.temu.batchSize", 100)

	// SHEIN 自动核价默认配置
	viper.SetDefault("autoPricing.shein.enabled", false)
	viper.SetDefault("autoPricing.shein.interval", 300)
	viper.SetDefault("autoPricing.shein.batchSize", 100)
}

// setAmazonDefaults 设置Amazon默认配置
func setAmazonDefaults() {
	viper.SetDefault("amazon.enabled", true)
	viper.SetDefault("amazon.headless", true)
	viper.SetDefault("amazon.browserPath", "./chrome/chrome.exe")
	viper.SetDefault("amazon.poolSize", 3)
	viper.SetDefault("amazon.viewportWidth", 1920)
	viper.SetDefault("amazon.viewportHeight", 1080)
	viper.SetDefault("amazon.proxyServer", "")
	viper.SetDefault("amazon.dataFreshnessDays", 7)
}

// setUpdaterDefaults 设置更新器默认配置
func setUpdaterDefaults() {
	viper.SetDefault("updater.enabled", true)
	viper.SetDefault("updater.updateURL", "https://auto-update-1303159911.cos.ap-shanghai.myqcloud.com/task-processor/version.json")
	viper.SetDefault("updater.checkInterval", 300)
	viper.SetDefault("updater.insecureSkipVerify", false)
}

// setSyncMonitorDefaults 设置同步和监控默认配置
func setSyncMonitorDefaults() {
	// 产品同步默认配置
	viper.SetDefault("sync.enabled", false)
	viper.SetDefault("sync.storeIDs", []int64{})

	// 产品监控默认配置
	viper.SetDefault("monitor.enabled", false)
	viper.SetDefault("monitor.storeIDs", []int64{})
	viper.SetDefault("monitor.checkInterval", 1440) // 默认24小时
	viper.SetDefault("monitor.batchSize", 50)
	viper.SetDefault("monitor.enablePriceAlert", true)
	viper.SetDefault("monitor.enableStockAlert", true)
	viper.SetDefault("monitor.priceChangeThreshold", 5.0)
	viper.SetDefault("monitor.stockChangeThreshold", 5)
}
