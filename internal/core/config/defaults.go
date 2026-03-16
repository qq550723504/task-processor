// Package config 提供配置管理功能
package config

import "github.com/spf13/viper"

// setDefaults 设置默认配置值
func setDefaults() {
	// 统一使用 NewDefaultConfig 作为默认值来源，避免散落的硬编码
	defaults := NewDefaultConfig()

	// 处理器默认配置
	viper.SetDefault("processor.maxRetries", defaults.Processor.MaxRetries)
	viper.SetDefault("processor.timeout", defaults.Processor.Timeout)

	// 工作池默认配置
	viper.SetDefault("worker.concurrency", defaults.Worker.Concurrency)
	viper.SetDefault("worker.bufferSize", defaults.Worker.BufferSize)
	viper.SetDefault("worker.taskInterval", defaults.Worker.TaskInterval)
	viper.SetDefault("worker.maxFetchPerCycle", defaults.Worker.MaxFetchPerCycle)
	viper.SetDefault("worker.queueThreshold", defaults.Worker.QueueThreshold)
	viper.SetDefault("worker.cleanupInterval", defaults.Worker.CleanupInterval)       // 2分钟清理间隔
	viper.SetDefault("worker.taskTimeout", defaults.Worker.TaskTimeout)               // 15分钟任务超时
	viper.SetDefault("worker.stuckTaskThreshold", defaults.Worker.StuckTaskThreshold) // 5分钟卡住阈值
	viper.SetDefault("worker.forceCleanupAfter", defaults.Worker.ForceCleanupAfter)   // 30分钟强制清理

	// OpenAI默认配置
	viper.SetDefault("openai.apiKey", defaults.OpenAI.APIKey)
	viper.SetDefault("openai.model", defaults.OpenAI.Model)
	viper.SetDefault("openai.baseURL", defaults.OpenAI.BaseURL)
	viper.SetDefault("openai.timeout", defaults.OpenAI.Timeout)

	// 管理系统默认配置
	viper.SetDefault("management.baseURL", defaults.Management.BaseURL)
	viper.SetDefault("management.clientID", defaults.Management.ClientID)
	viper.SetDefault("management.clientSecret", defaults.Management.ClientSecret)
	viper.SetDefault("management.tokenURL", defaults.Management.TokenURL)
	viper.SetDefault("management.scopes", defaults.Management.Scopes)
	viper.SetDefault("management.tenantID", defaults.Management.TenantID)

	// 将默认配置传递给各个模块的默认值设置函数
	setBrowserDefaults(defaults)
	setAmazonDefaults(defaults)
	setUpdaterDefaults(defaults)
	setPlatformDefaults(defaults)
}

// setBrowserDefaults 设置浏览器默认配置
func setBrowserDefaults(defaults *Config) {
	b := defaults.Browser

	viper.SetDefault("browser.enabled", b.Enabled)
	viper.SetDefault("browser.headless", b.Headless)
	viper.SetDefault("browser.browserPath", b.BrowserPath)
	viper.SetDefault("browser.poolSize", b.PoolSize) // 浏览器池大小
	viper.SetDefault("browser.viewportWidth", b.ViewportWidth)
	viper.SetDefault("browser.viewportHeight", b.ViewportHeight)
	viper.SetDefault("browser.proxyServer", b.ProxyServer)

	// 随机配置默认值
	rc := b.RandomConfig
	viper.SetDefault("browser.randomConfig.enabled", rc.Enabled)
	viper.SetDefault("browser.randomConfig.strategy", rc.Strategy)
	viper.SetDefault("browser.randomConfig.presetName", rc.PresetName)
	viper.SetDefault("browser.randomConfig.fingerprintStrategy", rc.FingerprintStrategy)
	viper.SetDefault("browser.randomConfig.healthCheckEnabled", rc.HealthCheckEnabled)
	viper.SetDefault("browser.randomConfig.maxRetries", rc.MaxRetries)
}

// setAmazonDefaults 设置Amazon默认配置
func setAmazonDefaults(defaults *Config) {
	a := defaults.Amazon

	viper.SetDefault("amazon.enabled", a.Enabled)
	viper.SetDefault("amazon.dataFreshnessDays", a.DataFreshnessDays)

	// Amazon SPAPI 默认配置
	sp := defaults.Amazon.SPAPI
	viper.SetDefault("amazon.spapi.enabled", sp.Enabled)
	viper.SetDefault("amazon.spapi.region", sp.Region)
	viper.SetDefault("amazon.spapi.defaultMarketplace", sp.DefaultMarketplace)
	viper.SetDefault("amazon.spapi.defaultFulfillmentType", sp.DefaultFulfillmentType)
	viper.SetDefault("amazon.spapi.defaultCondition", sp.DefaultCondition)
}

// setUpdaterDefaults 设置更新器默认配置
func setUpdaterDefaults(defaults *Config) {
	u := defaults.Updater

	viper.SetDefault("updater.enabled", u.Enabled)
	viper.SetDefault("updater.updateURL", u.UpdateURL)
	viper.SetDefault("updater.checkInterval", u.CheckInterval)
	viper.SetDefault("updater.insecureSkipVerify", u.InsecureSkipVerify)
}

// setPlatformDefaults 设置平台配置默认值
func setPlatformDefaults(defaults *Config) {
	// TEMU 平台默认配置
	setTemuDefaults(&defaults.Platforms.Temu)

	// SHEIN 平台默认配置
	setSheinDefaults(&defaults.Platforms.Shein)
}

// setTemuDefaults 设置TEMU平台默认配置
func setTemuDefaults(p *PlatformConfig) {

	// 平台启用状态默认配置
	viper.SetDefault("platforms.temu.enabled", p.Enabled)                   // 默认启用处理器（上架任务）
	viper.SetDefault("platforms.temu.schedulerEnabled", p.SchedulerEnabled) // 默认禁用调度任务

	// 自动核价默认配置
	ap := p.AutoPricing
	viper.SetDefault("platforms.temu.autoPricing.enabled", ap.Enabled)
	viper.SetDefault("platforms.temu.autoPricing.interval", ap.Interval)
	viper.SetDefault("platforms.temu.autoPricing.batchSize", ap.BatchSize)
	viper.SetDefault("platforms.temu.autoPricing.useAmazonPrice", ap.UseAmazonPrice) // 默认使用Amazon价格数据

	// 产品同步默认配置
	ps := p.ProductSync
	viper.SetDefault("platforms.temu.productSync.enabled", ps.Enabled)
	viper.SetDefault("platforms.temu.productSync.interval", ps.Interval) // 1小时

	// 库存同步默认配置
	is := p.InventorySync
	viper.SetDefault("platforms.temu.inventorySync.enabled", is.Enabled)
	viper.SetDefault("platforms.temu.inventorySync.interval", is.Interval) // 30分钟

	// 活动报名默认配置
	ar := p.ActivityRegistration
	viper.SetDefault("platforms.temu.activityRegistration.enabled", ar.Enabled)
	viper.SetDefault("platforms.temu.activityRegistration.interval", ar.Interval) // 2小时

	// 旧版产品同步默认配置（保留兼容）
	sp := p.SyncProduct
	viper.SetDefault("platforms.temu.sync.enabled", sp.Enabled)
	viper.SetDefault("platforms.temu.sync.storeIDs", sp.StoreIDs)
	viper.SetDefault("platforms.temu.sync.interval", sp.Interval)
	viper.SetDefault("platforms.temu.sync.batchSize", sp.BatchSize)

	// 产品监控默认配置
	m := p.Monitor
	viper.SetDefault("platforms.temu.monitor.enabled", m.Enabled)
	viper.SetDefault("platforms.temu.monitor.storeIDs", m.StoreIDs)
	viper.SetDefault("platforms.temu.monitor.checkInterval", m.CheckInterval)
	viper.SetDefault("platforms.temu.monitor.batchSize", m.BatchSize)
	viper.SetDefault("platforms.temu.monitor.enablePriceAlert", m.EnablePriceAlert)
	viper.SetDefault("platforms.temu.monitor.enableStockAlert", m.EnableStockAlert)
	viper.SetDefault("platforms.temu.monitor.priceChangeThreshold", m.PriceChangeThreshold)
	viper.SetDefault("platforms.temu.monitor.stockChangeThreshold", m.StockChangeThreshold)
}

// setSheinDefaults 设置SHEIN平台默认配置
func setSheinDefaults(p *PlatformConfig) {
	// 平台启用状态默认配置
	viper.SetDefault("platforms.shein.enabled", p.Enabled)                   // 默认启用处理器（上架任务）
	viper.SetDefault("platforms.shein.schedulerEnabled", p.SchedulerEnabled) // 默认禁用调度任务

	// 自动核价默认配置
	ap := p.AutoPricing
	viper.SetDefault("platforms.shein.autoPricing.enabled", ap.Enabled)
	viper.SetDefault("platforms.shein.autoPricing.interval", ap.Interval)
	viper.SetDefault("platforms.shein.autoPricing.batchSize", ap.BatchSize)

	// 产品同步默认配置
	ps := p.ProductSync
	viper.SetDefault("platforms.shein.productSync.enabled", ps.Enabled)
	viper.SetDefault("platforms.shein.productSync.interval", ps.Interval) // 1小时

	// 库存同步默认配置
	is := p.InventorySync
	viper.SetDefault("platforms.shein.inventorySync.enabled", is.Enabled)
	viper.SetDefault("platforms.shein.inventorySync.interval", is.Interval) // 30分钟

	// 活动报名默认配置
	ar := p.ActivityRegistration
	viper.SetDefault("platforms.shein.activityRegistration.enabled", ar.Enabled)
	viper.SetDefault("platforms.shein.activityRegistration.interval", ar.Interval) // 2小时

	// 旧版产品同步默认配置（保留兼容）
	sp := p.SyncProduct
	viper.SetDefault("platforms.shein.sync.enabled", sp.Enabled)
	viper.SetDefault("platforms.shein.sync.storeIDs", sp.StoreIDs)
	viper.SetDefault("platforms.shein.sync.interval", sp.Interval)
	viper.SetDefault("platforms.shein.sync.batchSize", sp.BatchSize)

	// 产品监控默认配置
	m := p.Monitor
	viper.SetDefault("platforms.shein.monitor.enabled", m.Enabled)
	viper.SetDefault("platforms.shein.monitor.storeIDs", m.StoreIDs)
	viper.SetDefault("platforms.shein.monitor.checkInterval", m.CheckInterval)
	viper.SetDefault("platforms.shein.monitor.batchSize", m.BatchSize)
	viper.SetDefault("platforms.shein.monitor.enablePriceAlert", m.EnablePriceAlert)
	viper.SetDefault("platforms.shein.monitor.enableStockAlert", m.EnableStockAlert)
	viper.SetDefault("platforms.shein.monitor.priceChangeThreshold", m.PriceChangeThreshold)
	viper.SetDefault("platforms.shein.monitor.stockChangeThreshold", m.StockChangeThreshold)
}
