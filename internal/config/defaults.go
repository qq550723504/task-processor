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

	// Amazon默认配置
	setAmazonDefaults()

	// 更新器默认配置
	setUpdaterDefaults()

	// 平台配置默认值
	setPlatformDefaults()
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

// setPlatformDefaults 设置平台配置默认值
func setPlatformDefaults() {
	// TEMU 平台默认配置
	setTemuDefaults()

	// SHEIN 平台默认配置
	setSheinDefaults()
}

// setTemuDefaults 设置TEMU平台默认配置
func setTemuDefaults() {
	// 自动定价默认配置
	viper.SetDefault("platforms.temu.autoPricing.enabled", false)
	viper.SetDefault("platforms.temu.autoPricing.interval", 300)
	viper.SetDefault("platforms.temu.autoPricing.batchSize", 100)

	// 产品同步默认配置
	viper.SetDefault("platforms.temu.sync.enabled", true)
	viper.SetDefault("platforms.temu.sync.storeIDs", []int64{})
	viper.SetDefault("platforms.temu.sync.interval", 60)
	viper.SetDefault("platforms.temu.sync.batchSize", 50)

	// 产品监控默认配置
	viper.SetDefault("platforms.temu.monitor.enabled", true)
	viper.SetDefault("platforms.temu.monitor.storeIDs", []int64{})
	viper.SetDefault("platforms.temu.monitor.checkInterval", 1440)
	viper.SetDefault("platforms.temu.monitor.batchSize", 100)
	viper.SetDefault("platforms.temu.monitor.enablePriceAlert", true)
	viper.SetDefault("platforms.temu.monitor.enableStockAlert", true)
	viper.SetDefault("platforms.temu.monitor.priceChangeThreshold", 10.0)
	viper.SetDefault("platforms.temu.monitor.stockChangeThreshold", 5)
}

// setSheinDefaults 设置SHEIN平台默认配置
func setSheinDefaults() {
	// 自动定价默认配置
	viper.SetDefault("platforms.shein.autoPricing.enabled", false)
	viper.SetDefault("platforms.shein.autoPricing.interval", 300)
	viper.SetDefault("platforms.shein.autoPricing.batchSize", 100)

	// 产品同步默认配置
	viper.SetDefault("platforms.shein.sync.enabled", true)
	viper.SetDefault("platforms.shein.sync.storeIDs", []int64{})
	viper.SetDefault("platforms.shein.sync.interval", 60)
	viper.SetDefault("platforms.shein.sync.batchSize", 50)

	// 产品监控默认配置
	viper.SetDefault("platforms.shein.monitor.enabled", true)
	viper.SetDefault("platforms.shein.monitor.storeIDs", []int64{})
	viper.SetDefault("platforms.shein.monitor.checkInterval", 1440)
	viper.SetDefault("platforms.shein.monitor.batchSize", 100)
	viper.SetDefault("platforms.shein.monitor.enablePriceAlert", true)
	viper.SetDefault("platforms.shein.monitor.enableStockAlert", true)
	viper.SetDefault("platforms.shein.monitor.priceChangeThreshold", 10.0)
	viper.SetDefault("platforms.shein.monitor.stockChangeThreshold", 5)
}
