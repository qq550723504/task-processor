package config

import "github.com/spf13/viper"

func setDefaults(v *viper.Viper) {
	defaults := NewDefaultConfig()

	v.SetDefault("processor.maxRetries", defaults.Processor.MaxRetries)
	v.SetDefault("processor.timeout", defaults.Processor.Timeout)

	v.SetDefault("worker.concurrency", defaults.Worker.Concurrency)
	v.SetDefault("worker.bufferSize", defaults.Worker.BufferSize)
	v.SetDefault("worker.taskInterval", defaults.Worker.TaskInterval)
	v.SetDefault("worker.maxFetchPerCycle", defaults.Worker.MaxFetchPerCycle)
	v.SetDefault("worker.queueThreshold", defaults.Worker.QueueThreshold)
	v.SetDefault("worker.cleanupInterval", defaults.Worker.CleanupInterval)
	v.SetDefault("worker.taskTimeout", defaults.Worker.TaskTimeout)
	v.SetDefault("worker.stuckTaskThreshold", defaults.Worker.StuckTaskThreshold)
	v.SetDefault("worker.forceCleanupAfter", defaults.Worker.ForceCleanupAfter)

	v.SetDefault("openai.apiKey", defaults.OpenAI.APIKey)
	v.SetDefault("openai.model", defaults.OpenAI.Model)
	v.SetDefault("openai.baseURL", defaults.OpenAI.BaseURL)
	v.SetDefault("openai.timeout", defaults.OpenAI.Timeout)

	v.SetDefault("management.baseURL", defaults.Management.BaseURL)
	v.SetDefault("management.clientID", defaults.Management.ClientID)
	v.SetDefault("management.clientSecret", defaults.Management.ClientSecret)
	v.SetDefault("management.tokenURL", defaults.Management.TokenURL)
	v.SetDefault("management.scopes", defaults.Management.Scopes)
	v.SetDefault("management.tenantID", defaults.Management.TenantID)

	setBrowserDefaults(v, defaults)
	setAmazonDefaults(v, defaults)
	setProductImageDefaults(v, defaults)
	setUpdaterDefaults(v, defaults)
	setPlatformDefaults(v, defaults)
	setRabbitMQDefaults(v, defaults)
}

func setProductImageDefaults(v *viper.Viper, defaults *Config) {
	pi := defaults.ProductImage

	v.SetDefault("productimage.workDir", pi.WorkDir)
	v.SetDefault("productimage.segmenter.enabled", pi.Segmenter.Enabled)
	v.SetDefault("productimage.segmenter.endpoint", pi.Segmenter.Endpoint)
	v.SetDefault("productimage.segmenter.apiKey", pi.Segmenter.APIKey)
	v.SetDefault("productimage.segmenter.timeout", pi.Segmenter.Timeout)
	v.SetDefault("productimage.whiteBackground.enabled", pi.WhiteBackground.Enabled)
	v.SetDefault("productimage.whiteBackground.endpoint", pi.WhiteBackground.Endpoint)
	v.SetDefault("productimage.whiteBackground.apiKey", pi.WhiteBackground.APIKey)
	v.SetDefault("productimage.whiteBackground.timeout", pi.WhiteBackground.Timeout)
	v.SetDefault("productimage.publisher.enabled", pi.Publisher.Enabled)
	v.SetDefault("productimage.publisher.provider", pi.Publisher.Provider)
	v.SetDefault("productimage.publisher.outputDir", pi.Publisher.OutputDir)
	v.SetDefault("productimage.publisher.publicBase", pi.Publisher.PublicBase)
	v.SetDefault("productimage.lifecycle.cleanupTemporaryFiles", pi.Lifecycle.CleanupTemporaryFiles)
	v.SetDefault("productimage.lifecycle.reuseExistingAssets", pi.Lifecycle.ReuseExistingAssets)
}

func setBrowserDefaults(v *viper.Viper, defaults *Config) {
	b := defaults.Browser

	v.SetDefault("browser.enabled", b.Enabled)
	v.SetDefault("browser.headless", b.Headless)
	v.SetDefault("browser.browserPath", b.BrowserPath)
	v.SetDefault("browser.poolSize", b.PoolSize)
	v.SetDefault("browser.viewportWidth", b.ViewportWidth)
	v.SetDefault("browser.viewportHeight", b.ViewportHeight)
	v.SetDefault("browser.proxyServer", b.ProxyServer)

	rc := b.RandomConfig
	v.SetDefault("browser.randomConfig.enabled", rc.Enabled)
	v.SetDefault("browser.randomConfig.strategy", rc.Strategy)
	v.SetDefault("browser.randomConfig.presetName", rc.PresetName)
	v.SetDefault("browser.randomConfig.fingerprintStrategy", rc.FingerprintStrategy)
	v.SetDefault("browser.randomConfig.healthCheckEnabled", rc.HealthCheckEnabled)
	v.SetDefault("browser.randomConfig.maxRetries", rc.MaxRetries)
}

func setAmazonDefaults(v *viper.Viper, defaults *Config) {
	a := defaults.Amazon

	v.SetDefault("amazon.enabled", a.Enabled)
	v.SetDefault("amazon.dataFreshnessDays", a.DataFreshnessDays)

	sp := defaults.Amazon.SPAPI
	v.SetDefault("amazon.spapi.enabled", sp.Enabled)
	v.SetDefault("amazon.spapi.region", sp.Region)
	v.SetDefault("amazon.spapi.defaultMarketplace", sp.DefaultMarketplace)
	v.SetDefault("amazon.spapi.defaultFulfillmentType", sp.DefaultFulfillmentType)
	v.SetDefault("amazon.spapi.defaultCondition", sp.DefaultCondition)
}

func setUpdaterDefaults(v *viper.Viper, defaults *Config) {
	u := defaults.Updater

	v.SetDefault("updater.enabled", u.Enabled)
	v.SetDefault("updater.updateURL", u.UpdateURL)
	v.SetDefault("updater.checkInterval", u.CheckInterval)
	v.SetDefault("updater.insecureSkipVerify", u.InsecureSkipVerify)
}

func setRabbitMQDefaults(v *viper.Viper, defaults *Config) {
	if defaults.RabbitMQ == nil {
		return
	}

	r := defaults.RabbitMQ
	v.SetDefault("rabbitmq.enabled", r.Enabled)
	v.SetDefault("rabbitmq.url", r.URL)
	v.SetDefault("rabbitmq.maxReconnectTries", r.MaxReconnectTries)
	v.SetDefault("rabbitmq.consumer.prefetchCount", r.Consumer.PrefetchCount)
	v.SetDefault("rabbitmq.consumer.prefetchSize", r.Consumer.PrefetchSize)
	v.SetDefault("rabbitmq.consumer.maxRetries", r.Consumer.MaxRetries)
	v.SetDefault("rabbitmq.node.maxConcurrency", r.Node.MaxConcurrency)
	v.SetDefault("rabbitmq.node.healthCheckPort", r.Node.HealthCheckPort)
	v.SetDefault("rabbitmq.node.metricsPort", r.Node.MetricsPort)
	v.SetDefault("rabbitmq.node.logLevel", r.Node.LogLevel)
}

func setPlatformDefaults(v *viper.Viper, defaults *Config) {
	setTemuDefaults(v, &defaults.Platforms.Temu)
	setSheinDefaults(v, &defaults.Platforms.Shein)
}

func setTemuDefaults(v *viper.Viper, p *PlatformConfig) {
	v.SetDefault("platforms.temu.enabled", p.Enabled)
	v.SetDefault("platforms.temu.schedulerEnabled", p.SchedulerEnabled)

	ap := p.AutoPricing
	v.SetDefault("platforms.temu.autoPricing.enabled", ap.Enabled)
	v.SetDefault("platforms.temu.autoPricing.interval", ap.Interval)
	v.SetDefault("platforms.temu.autoPricing.batchSize", ap.BatchSize)
	v.SetDefault("platforms.temu.autoPricing.useAmazonPrice", ap.UseAmazonPrice)

	ps := p.ProductSync
	v.SetDefault("platforms.temu.productSync.enabled", ps.Enabled)
	v.SetDefault("platforms.temu.productSync.interval", ps.Interval)

	is := p.InventorySync
	v.SetDefault("platforms.temu.inventorySync.enabled", is.Enabled)
	v.SetDefault("platforms.temu.inventorySync.interval", is.Interval)

	ar := p.ActivityRegistration
	v.SetDefault("platforms.temu.activityRegistration.enabled", ar.Enabled)
	v.SetDefault("platforms.temu.activityRegistration.interval", ar.Interval)

	m := p.Monitor
	v.SetDefault("platforms.temu.monitor.enabled", m.Enabled)
	v.SetDefault("platforms.temu.monitor.storeIDs", m.StoreIDs)
	v.SetDefault("platforms.temu.monitor.checkInterval", m.CheckInterval)
	v.SetDefault("platforms.temu.monitor.batchSize", m.BatchSize)
	v.SetDefault("platforms.temu.monitor.enablePriceAlert", m.EnablePriceAlert)
	v.SetDefault("platforms.temu.monitor.enableStockAlert", m.EnableStockAlert)
	v.SetDefault("platforms.temu.monitor.priceChangeThreshold", m.PriceChangeThreshold)
	v.SetDefault("platforms.temu.monitor.stockChangeThreshold", m.StockChangeThreshold)
}

func setSheinDefaults(v *viper.Viper, p *PlatformConfig) {
	v.SetDefault("platforms.shein.enabled", p.Enabled)
	v.SetDefault("platforms.shein.schedulerEnabled", p.SchedulerEnabled)

	ap := p.AutoPricing
	v.SetDefault("platforms.shein.autoPricing.enabled", ap.Enabled)
	v.SetDefault("platforms.shein.autoPricing.interval", ap.Interval)
	v.SetDefault("platforms.shein.autoPricing.batchSize", ap.BatchSize)

	ps := p.ProductSync
	v.SetDefault("platforms.shein.productSync.enabled", ps.Enabled)
	v.SetDefault("platforms.shein.productSync.interval", ps.Interval)

	is := p.InventorySync
	v.SetDefault("platforms.shein.inventorySync.enabled", is.Enabled)
	v.SetDefault("platforms.shein.inventorySync.interval", is.Interval)

	ar := p.ActivityRegistration
	v.SetDefault("platforms.shein.activityRegistration.enabled", ar.Enabled)
	v.SetDefault("platforms.shein.activityRegistration.interval", ar.Interval)

	m := p.Monitor
	v.SetDefault("platforms.shein.monitor.enabled", m.Enabled)
	v.SetDefault("platforms.shein.monitor.storeIDs", m.StoreIDs)
	v.SetDefault("platforms.shein.monitor.checkInterval", m.CheckInterval)
	v.SetDefault("platforms.shein.monitor.batchSize", m.BatchSize)
	v.SetDefault("platforms.shein.monitor.enablePriceAlert", m.EnablePriceAlert)
	v.SetDefault("platforms.shein.monitor.enableStockAlert", m.EnableStockAlert)
	v.SetDefault("platforms.shein.monitor.priceChangeThreshold", m.PriceChangeThreshold)
	v.SetDefault("platforms.shein.monitor.stockChangeThreshold", m.StockChangeThreshold)
}
