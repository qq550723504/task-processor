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
	viper.SetDefault("worker.cleanupInterval", 120)    // 2分钟清理间隔
	viper.SetDefault("worker.taskTimeout", 900)        // 15分钟任务超时
	viper.SetDefault("worker.stuckTaskThreshold", 300) // 5分钟卡住阈值
	viper.SetDefault("worker.forceCleanupAfter", 1800) // 30分钟强制清理

	// OpenAI默认配置
	viper.SetDefault("openai.apiKey", "sk-yJ3RQprPLyBcoqEkeBNimE6Dhj86CAjY2uHAc5yqLZd1KHa3")
	viper.SetDefault("openai.model", "gemini-2.0-flash")
	viper.SetDefault("openai.baseURL", "https://ai.linkcloudai.com/v1")
	viper.SetDefault("openai.timeout", 300)

	// 管理系统默认配置
	viper.SetDefault("management.baseURL", "http://getway.linkcloudai.com")
	viper.SetDefault("management.clientID", "go-listing")
	viper.SetDefault("management.clientSecret", "go-listing-secret")
	viper.SetDefault("management.tokenURL", "http://getway.linkcloudai.com/admin-api/system/oauth2/token")
	viper.SetDefault("management.scopes", []string{"user.read"})
	viper.SetDefault("management.tenantID", "1")

	// 浏览器默认配置
	setBrowserDefaults()

	// Amazon默认配置
	setAmazonDefaults()

	// 更新器默认配置
	setUpdaterDefaults()

	// 平台配置默认值
	setPlatformDefaults()
}

// setBrowserDefaults 设置浏览器默认配置
func setBrowserDefaults() {
	viper.SetDefault("browser.enabled", true)
	viper.SetDefault("browser.headless", true)
	viper.SetDefault("browser.browserPath", "./chrome/chrome.exe")
	viper.SetDefault("browser.poolSize", 3) // 浏览器池大小
	viper.SetDefault("browser.viewportWidth", 1920)
	viper.SetDefault("browser.viewportHeight", 1080)
	viper.SetDefault("browser.proxyServer", "")

	// 随机配置默认值
	viper.SetDefault("browser.randomConfig.enabled", true)
	viper.SetDefault("browser.randomConfig.strategy", "random")
	viper.SetDefault("browser.randomConfig.presetName", "windows_high_end")
	viper.SetDefault("browser.randomConfig.fingerprintStrategy", "random")
	viper.SetDefault("browser.randomConfig.healthCheckEnabled", true)
	viper.SetDefault("browser.randomConfig.maxRetries", 3)
}

// setAmazonDefaults 设置Amazon默认配置
func setAmazonDefaults() {
	viper.SetDefault("amazon.enabled", true)
	viper.SetDefault("amazon.dataFreshnessDays", 7)

	// Amazon SPAPI 默认配置
	viper.SetDefault("amazon.spapi.enabled", false)
	viper.SetDefault("amazon.spapi.region", "us-east-1")
	viper.SetDefault("amazon.spapi.defaultMarketplace", "us")
	viper.SetDefault("amazon.spapi.defaultFulfillmentType", "FBM")
	viper.SetDefault("amazon.spapi.defaultCondition", "New")
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
	// 平台启用状态默认配置
	viper.SetDefault("platforms.temu.enabled", true)           // 默认启用处理器（上架任务）
	viper.SetDefault("platforms.temu.schedulerEnabled", false) // 默认禁用调度任务

	// 自动核价默认配置
	viper.SetDefault("platforms.temu.autoPricing.enabled", false)
	viper.SetDefault("platforms.temu.autoPricing.interval", 300)
	viper.SetDefault("platforms.temu.autoPricing.batchSize", 100)
	viper.SetDefault("platforms.temu.autoPricing.useAmazonPrice", false) // 默认使用Amazon价格数据

	// 产品同步默认配置
	viper.SetDefault("platforms.temu.productSync.enabled", false)
	viper.SetDefault("platforms.temu.productSync.interval", 3600) // 1小时

	// 库存同步默认配置
	viper.SetDefault("platforms.temu.inventorySync.enabled", false)
	viper.SetDefault("platforms.temu.inventorySync.interval", 1800) // 30分钟

	// 活动报名默认配置
	viper.SetDefault("platforms.temu.activityRegistration.enabled", false)
	viper.SetDefault("platforms.temu.activityRegistration.interval", 7200) // 2小时

	// 旧版产品同步默认配置（保留兼容）
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
	// 平台启用状态默认配置
	viper.SetDefault("platforms.shein.enabled", true)           // 默认启用处理器（上架任务）
	viper.SetDefault("platforms.shein.schedulerEnabled", false) // 默认禁用调度任务

	// 自动核价默认配置
	viper.SetDefault("platforms.shein.autoPricing.enabled", false)
	viper.SetDefault("platforms.shein.autoPricing.interval", 300)
	viper.SetDefault("platforms.shein.autoPricing.batchSize", 100)

	// 产品同步默认配置
	viper.SetDefault("platforms.shein.productSync.enabled", false)
	viper.SetDefault("platforms.shein.productSync.interval", 3600) // 1小时

	// 库存同步默认配置
	viper.SetDefault("platforms.shein.inventorySync.enabled", false)
	viper.SetDefault("platforms.shein.inventorySync.interval", 1800) // 30分钟

	// 活动报名默认配置
	viper.SetDefault("platforms.shein.activityRegistration.enabled", false)
	viper.SetDefault("platforms.shein.activityRegistration.interval", 7200) // 2小时

	// 旧版产品同步默认配置（保留兼容）
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
