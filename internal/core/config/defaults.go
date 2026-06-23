package config

import (
	"time"

	"github.com/spf13/viper"
)

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
	v.SetDefault("management.httpClient.insecureSkipVerify", defaults.Management.HTTPClient.InsecureSkipVerify)
	v.SetDefault("debug.save_publish_json", defaults.Debug.SavePublishJSON)
	v.SetDefault("debug.productEnrichMockLLM", defaults.Debug.ProductEnrichMockLLM)
	v.SetDefault("listingkit.sheinSubmitDebugDumpDir", defaults.ListingKit.SheinSubmitDebugDumpDir)
	v.SetDefault("listingkit.platformAdminUsers", defaults.ListingKit.PlatformAdminUsers)
	v.SetDefault("listingkit.platformAdminRoles", defaults.ListingKit.PlatformAdminRoles)
	v.SetDefault("listingkit.zitadel.issuerURL", defaults.ListingKit.Zitadel.IssuerURL)
	v.SetDefault("listingkit.zitadel.clientID", defaults.ListingKit.Zitadel.ClientID)
	v.SetDefault("listingkit.zitadel.clientSecret", defaults.ListingKit.Zitadel.ClientSecret)
	v.SetDefault("listingkit.zitadel.allowedTenantIDs", defaults.ListingKit.Zitadel.AllowedTenantIDs)
	v.SetDefault("listingkit.zitadel.allowedUserIDs", defaults.ListingKit.Zitadel.AllowedUserIDs)
	v.SetDefault("listingkit.zitadel.allowedUsernames", defaults.ListingKit.Zitadel.AllowedUsernames)
	v.SetDefault("listingkit.zitadel.allowedRoles", defaults.ListingKit.Zitadel.AllowedRoles)

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
	v.SetDefault("productimage.scene.enabled", pi.Scene.Enabled)
	v.SetDefault("productimage.scene.endpoint", pi.Scene.Endpoint)
	v.SetDefault("productimage.scene.apiKey", pi.Scene.APIKey)
	v.SetDefault("productimage.scene.timeout", pi.Scene.Timeout)
	v.SetDefault("productimage.publisher.enabled", pi.Publisher.Enabled)
	v.SetDefault("productimage.publisher.provider", pi.Publisher.Provider)
	v.SetDefault("productimage.publisher.outputDir", pi.Publisher.OutputDir)
	v.SetDefault("productimage.publisher.publicBase", pi.Publisher.PublicBase)
	v.SetDefault("productimage.publisher.s3.bucket", pi.Publisher.S3.Bucket)
	v.SetDefault("productimage.publisher.s3.region", pi.Publisher.S3.Region)
	v.SetDefault("productimage.publisher.s3.endpoint", pi.Publisher.S3.Endpoint)
	v.SetDefault("productimage.publisher.s3.accessKeyID", pi.Publisher.S3.AccessKeyID)
	v.SetDefault("productimage.publisher.s3.secretAccessKey", pi.Publisher.S3.SecretAccessKey)
	v.SetDefault("productimage.publisher.s3.usePathStyle", pi.Publisher.S3.UsePathStyle)
	v.SetDefault("productimage.lifecycle.cleanupTemporaryFiles", pi.Lifecycle.CleanupTemporaryFiles)
	v.SetDefault("productimage.lifecycle.reuseExistingAssets", pi.Lifecycle.ReuseExistingAssets)
}

func setBrowserDefaults(v *viper.Viper, defaults *Config) {
	b := defaults.Browser

	v.SetDefault("browser.enabled", b.Enabled)
	v.SetDefault("browser.headless", b.Headless)
	v.SetDefault("browser.browserPath", b.BrowserPath)
	v.SetDefault("browser.userDataDir", b.UserDataDir)
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
	v.SetDefault("browser.randomConfig.maxUsesPerInstance", rc.MaxUsesPerInstance)
}

func setAmazonDefaults(v *viper.Viper, defaults *Config) {
	a := defaults.Amazon

	v.SetDefault("amazon.enabled", a.Enabled)
	v.SetDefault("amazon.dataFreshnessDays", a.DataFreshnessDays)
	v.SetDefault("amazon.productDedupe.lockTTLSeconds", a.ProductDedupe.LockTTLSeconds)
	v.SetDefault("amazon.productDedupe.resultTTLSeconds", a.ProductDedupe.ResultTTLSeconds)
	v.SetDefault("amazon.productDedupe.waitTimeoutSeconds", a.ProductDedupe.WaitTimeoutSeconds)
	v.SetDefault("amazon.productDedupe.pollIntervalMillis", a.ProductDedupe.PollIntervalMillis)
	v.SetDefault("amazon.failureArtifacts.enabled", a.FailureArtifacts.Enabled)
	v.SetDefault("amazon.failureArtifacts.directory", a.FailureArtifacts.Directory)
	v.SetDefault("amazon.failureArtifacts.captureHTML", a.FailureArtifacts.CaptureHTML)
	v.SetDefault("amazon.failureArtifacts.maxHTMLBytes", a.FailureArtifacts.MaxHTMLBytes)
	v.SetDefault("amazon.riskControl.captchaRecreateThreshold", a.RiskControl.CaptchaRecreateThreshold)
	v.SetDefault("amazon.riskControl.authenticationRecreateThreshold", a.RiskControl.AuthenticationRecreateThreshold)
	v.SetDefault("amazon.riskControl.browserCrashRecreateThreshold", a.RiskControl.BrowserCrashRecreateThreshold)
	v.SetDefault("amazon.riskControl.timeoutRecreateThreshold", a.RiskControl.TimeoutRecreateThreshold)
	v.SetDefault("amazon.riskControl.networkRecreateThreshold", a.RiskControl.NetworkRecreateThreshold)
	v.SetDefault("amazon.riskControl.serverErrorRecreateThreshold", a.RiskControl.ServerErrorRecreateThreshold)
	v.SetDefault("amazon.regionGuard.enabled", a.RegionGuard.Enabled)
	v.SetDefault("amazon.regionGuard.failureThreshold", a.RegionGuard.FailureThreshold)
	v.SetDefault("amazon.regionGuard.evaluationWindowSeconds", a.RegionGuard.EvaluationWindowSeconds)
	v.SetDefault("amazon.regionGuard.cooldownSeconds", a.RegionGuard.CooldownSeconds)
	v.SetDefault("amazon.qualityControl.retryOnValidationFailure", a.QualityControl.RetryOnValidationFailure)
	v.SetDefault("amazon.qualityControl.validationRetryMaxAttempts", a.QualityControl.ValidationRetryMaxAttempts)
	v.SetDefault("amazon.proxyPool.enabled", a.ProxyPool.Enabled)
	v.SetDefault("amazon.proxyPool.strategy", a.ProxyPool.Strategy)
	v.SetDefault("amazon.proxyPool.failureCooldownSeconds", a.ProxyPool.FailureCooldownSeconds)
	v.SetDefault("amazon.proxyPool.proxies", a.ProxyPool.Proxies)
	v.SetDefault("amazon.concurrencyControl.enabled", a.ConcurrencyControl.Enabled)
	v.SetDefault("amazon.concurrencyControl.maxInFlight", a.ConcurrencyControl.MaxInFlight)
	v.SetDefault("amazon.concurrencyControl.maxWaiting", a.ConcurrencyControl.MaxWaiting)
	v.SetDefault("amazon.concurrencyControl.acquireTimeoutSeconds", a.ConcurrencyControl.AcquireTimeoutSeconds)
	v.SetDefault("amazon.concurrencyControl.perRegion", a.ConcurrencyControl.PerRegion)
	v.SetDefault("amazon.remoteAPI.enabled", a.RemoteAPI.Enabled)
	v.SetDefault("amazon.remoteAPI.baseURL", a.RemoteAPI.BaseURL)
	v.SetDefault("amazon.remoteAPI.timeout", a.RemoteAPI.Timeout)

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
	v.SetDefault("rabbitmq.node.role", r.Node.Role)
	v.SetDefault("rabbitmq.node.healthCheckPort", r.Node.HealthCheckPort)
	v.SetDefault("rabbitmq.node.metricsPort", r.Node.MetricsPort)
	v.SetDefault("rabbitmq.node.logLevel", r.Node.LogLevel)
	v.SetDefault("rabbitmq.deadLetter.enabled", r.DeadLetter.Enabled)
	v.SetDefault("rabbitmq.deadLetter.queueName", firstNonEmptyString(r.DeadLetter.QueueName, "tasks.dlq"))
	v.SetDefault("rabbitmq.processingTimeoutWatchdog.enabled", r.ProcessingTimeout.Enabled)
	v.SetDefault("rabbitmq.processingTimeoutWatchdog.intervalSeconds", int(firstPositiveDuration(r.ProcessingTimeout.Interval, 5*time.Minute).Seconds()))
	v.SetDefault("rabbitmq.processingTimeoutWatchdog.timeoutMinutes", firstPositiveInt(r.ProcessingTimeout.TimeoutMinutes, 30))
	v.SetDefault("rabbitmq.processingTimeoutWatchdog.recoveryLimit", firstPositiveInt(r.ProcessingTimeout.RecoveryLimit, 100))
	v.SetDefault("rabbitmq.staleQueuedWatchdog.enabled", r.StaleQueued.Enabled)
	v.SetDefault("rabbitmq.staleQueuedWatchdog.intervalSeconds", int(firstPositiveDuration(r.StaleQueued.Interval, 5*time.Minute).Seconds()))
	v.SetDefault("rabbitmq.staleQueuedWatchdog.timeoutMinutes", firstPositiveInt(r.StaleQueued.TimeoutMinutes, 120))
	v.SetDefault("rabbitmq.staleQueuedWatchdog.recoveryLimit", firstPositiveInt(r.StaleQueued.RecoveryLimit, 500))
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstPositiveDuration(values ...time.Duration) time.Duration {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func setPlatformDefaults(v *viper.Viper, defaults *Config) {
	setTemuDefaults(v, &defaults.Platforms.Temu)
	setSheinDefaults(v, &defaults.Platforms.Shein)
	setSDSDefaults(v, &defaults.Platforms.SDS)
	setAlibaba1688Defaults(v, &defaults.Platforms.Alibaba1688)
}

func setSDSDefaults(v *viper.Viper, p *SDSPlatformConfig) {
	v.SetDefault("platforms.sds.loginService.baseURL", p.LoginService.BaseURL)
	v.SetDefault("platforms.sds.loginService.sharedKey", p.LoginService.SharedKey)
	v.SetDefault("platforms.sds.loginService.tenantID", p.LoginService.TenantID)
	v.SetDefault("platforms.sds.loginService.identifier", p.LoginService.Identifier)
	v.SetDefault("platforms.sds.loginService.merchantName", p.LoginService.MerchantName)
	v.SetDefault("platforms.sds.loginService.username", p.LoginService.Username)
	v.SetDefault("platforms.sds.loginService.password", p.LoginService.Password)
	v.SetDefault("platforms.sds.loginService.maxConcurrentLogins", p.LoginService.MaxConcurrentLogins)
	v.SetDefault("platforms.sds.loginService.profileRootDir", p.LoginService.ProfileRootDir)
	v.SetDefault("platforms.sds.loginService.artifactDir", p.LoginService.ArtifactDir)
	v.SetDefault("platforms.sds.loginService.defaultHeadless", p.LoginService.DefaultHeadless)
	v.SetDefault("platforms.sds.authRedis.host", p.AuthRedis.Host)
	v.SetDefault("platforms.sds.authRedis.port", p.AuthRedis.Port)
	v.SetDefault("platforms.sds.authRedis.password", p.AuthRedis.Password)
	v.SetDefault("platforms.sds.authRedis.db", p.AuthRedis.DB)
	v.SetDefault("platforms.sds.authRedis.pool_size", p.AuthRedis.PoolSize)
	v.SetDefault("platforms.sds.authBootstrap.staticAccessToken", p.AuthBootstrap.StaticAccessToken)
	v.SetDefault("platforms.sds.authBootstrap.staticOutToken", p.AuthBootstrap.StaticOutToken)
	v.SetDefault("platforms.sds.authBootstrap.staticMerchantID", p.AuthBootstrap.StaticMerchantID)
	v.SetDefault("platforms.sds.authBootstrap.staticCookie", p.AuthBootstrap.StaticCookie)
	v.SetDefault("platforms.sds.authBootstrap.loginDomainName", p.AuthBootstrap.LoginDomainName)
	v.SetDefault("platforms.sds.authBootstrap.loginVerifyCaptchaParam", p.AuthBootstrap.LoginVerifyCaptchaParam)
	v.SetDefault("platforms.sds.authBootstrap.loginExtraInfo", p.AuthBootstrap.LoginExtraInfo)
	v.SetDefault("platforms.sds.authBootstrap.managementStoreID", p.AuthBootstrap.ManagementStoreID)
}

func setTemuDefaults(v *viper.Viper, p *PlatformConfig) {
	v.SetDefault("platforms.temu.enabled", p.Enabled)
	v.SetDefault("platforms.temu.schedulerEnabled", p.SchedulerEnabled)
	v.SetDefault("platforms.temu.fetchMode", p.FetchMode)
	v.SetDefault("platforms.temu.httpClient.insecureSkipVerify", p.HTTPClient.InsecureSkipVerify)

	ap := p.AutoPricing
	v.SetDefault("platforms.temu.autoPricing.enabled", ap.Enabled)
	v.SetDefault("platforms.temu.autoPricing.interval", ap.Interval)
	v.SetDefault("platforms.temu.autoPricing.batchSize", ap.BatchSize)
	v.SetDefault("platforms.temu.autoPricing.useAmazonPrice", ap.UseAmazonPrice)

	lp := p.ListingPricing
	v.SetDefault("platforms.temu.listingPricing.enabled", lp.Enabled)
	v.SetDefault("platforms.temu.listingPricing.currency", lp.Currency)
	v.SetDefault("platforms.temu.listingPricing.markupRate", lp.MarkupRate)
	v.SetDefault("platforms.temu.listingPricing.fixedMarkup", lp.FixedMarkup)
	v.SetDefault("platforms.temu.listingPricing.shippingCost", lp.ShippingCost)
	v.SetDefault("platforms.temu.listingPricing.commissionRate", lp.CommissionRate)
	v.SetDefault("platforms.temu.listingPricing.minimumPrice", lp.MinimumPrice)
	v.SetDefault("platforms.temu.listingPricing.roundTo", lp.RoundTo)

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
	v.SetDefault("platforms.shein.fetchMode", p.FetchMode)
	v.SetDefault("platforms.shein.httpClient.insecureSkipVerify", p.HTTPClient.InsecureSkipVerify)
	v.SetDefault("platforms.shein.cookieRedis.host", p.CookieRedis.Host)
	v.SetDefault("platforms.shein.cookieRedis.port", p.CookieRedis.Port)
	v.SetDefault("platforms.shein.cookieRedis.password", p.CookieRedis.Password)
	v.SetDefault("platforms.shein.cookieRedis.db", p.CookieRedis.DB)
	v.SetDefault("platforms.shein.cookieRedis.pool_size", p.CookieRedis.PoolSize)
	v.SetDefault("platforms.shein.loginService.tenantID", p.LoginService.TenantID)
	v.SetDefault("platforms.shein.loginService.identifier", p.LoginService.Identifier)
	v.SetDefault("platforms.shein.loginService.merchantName", p.LoginService.MerchantName)
	v.SetDefault("platforms.shein.loginService.username", p.LoginService.Username)
	v.SetDefault("platforms.shein.loginService.password", p.LoginService.Password)
	v.SetDefault("platforms.shein.loginService.maxConcurrentLogins", p.LoginService.MaxConcurrentLogins)
	v.SetDefault("platforms.shein.loginService.profileRootDir", p.LoginService.ProfileRootDir)
	v.SetDefault("platforms.shein.loginService.artifactDir", p.LoginService.ArtifactDir)
	v.SetDefault("platforms.shein.loginService.defaultHeadless", p.LoginService.DefaultHeadless)

	ap := p.AutoPricing
	v.SetDefault("platforms.shein.autoPricing.enabled", ap.Enabled)
	v.SetDefault("platforms.shein.autoPricing.interval", ap.Interval)
	v.SetDefault("platforms.shein.autoPricing.batchSize", ap.BatchSize)

	lp := p.ListingPricing
	v.SetDefault("platforms.shein.listingPricing.enabled", lp.Enabled)
	v.SetDefault("platforms.shein.listingPricing.currency", lp.Currency)
	v.SetDefault("platforms.shein.listingPricing.markupRate", lp.MarkupRate)
	v.SetDefault("platforms.shein.listingPricing.fixedMarkup", lp.FixedMarkup)
	v.SetDefault("platforms.shein.listingPricing.shippingCost", lp.ShippingCost)
	v.SetDefault("platforms.shein.listingPricing.commissionRate", lp.CommissionRate)
	v.SetDefault("platforms.shein.listingPricing.minimumPrice", lp.MinimumPrice)
	v.SetDefault("platforms.shein.listingPricing.roundTo", lp.RoundTo)

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

func setAlibaba1688Defaults(v *viper.Viper, p *Alibaba1688Config) {
	v.SetDefault("platforms.alibaba1688.enabled", p.Enabled)
	v.SetDefault("platforms.alibaba1688.timeout", p.Timeout)
	v.SetDefault("platforms.alibaba1688.poolSize", p.PoolSize)
}
