// Package config 提供配置管理功能
package config

import "github.com/spf13/viper"

// buildConfig 构建配置对象
func buildConfig() *Config {
	cfg := &Config{
		Processor: ProcessorConfig{
			MaxRetries: viper.GetInt("processor.maxRetries"),
			Timeout:    viper.GetInt("processor.timeout"),
		},
		Worker: WorkerConfig{
			Concurrency:        viper.GetInt("worker.concurrency"),
			BufferSize:         viper.GetInt("worker.bufferSize"),
			TaskInterval:       viper.GetInt("worker.taskInterval"),
			MaxFetchPerCycle:   viper.GetInt("worker.maxFetchPerCycle"),
			QueueThreshold:     viper.GetInt("worker.queueThreshold"),
			CleanupInterval:    viper.GetInt("worker.cleanupInterval"),
			TaskTimeout:        viper.GetInt("worker.taskTimeout"),
			StuckTaskThreshold: viper.GetInt("worker.stuckTaskThreshold"),
			ForceCleanupAfter:  viper.GetInt("worker.forceCleanupAfter"),
		},
		OpenAI: OpenAIConfig{
			APIKey:  viper.GetString("openai.apiKey"),
			Model:   viper.GetString("openai.model"),
			BaseURL: viper.GetString("openai.baseURL"),
			Timeout: viper.GetInt("openai.timeout"),
		},
		Management: ManagementConfig{
			BaseURL:      viper.GetString("management.baseURL"),
			ClientID:     viper.GetString("management.clientID"),
			ClientSecret: viper.GetString("management.clientSecret"),
			TokenURL:     viper.GetString("management.tokenURL"),
			Scopes:       viper.GetStringSlice("management.scopes"),
			TenantID:     viper.GetString("management.tenantID"),
			UserID:       viper.GetInt64("management.userID"),
			StoreIDs:     getInt64Slice("management.storeIDs"),
		},
		Platforms: PlatformsConfig{
			Temu: PlatformConfig{
				Enabled:          viper.GetBool("platforms.temu.enabled"),
				SchedulerEnabled: viper.GetBool("platforms.temu.schedulerEnabled"),
				AutoPricing: AutoPricingConfig{
					Enabled:        viper.GetBool("platforms.temu.autoPricing.enabled"),
					Interval:       viper.GetInt("platforms.temu.autoPricing.interval"),
					BatchSize:      viper.GetInt("platforms.temu.autoPricing.batchSize"),
					UseAmazonPrice: viper.GetBool("platforms.temu.autoPricing.useAmazonPrice"),
				},
				ProductSync: ScheduledTaskConfig{
					Enabled:  viper.GetBool("platforms.temu.productSync.enabled"),
					Interval: viper.GetInt("platforms.temu.productSync.interval"),
				},
				InventorySync: ScheduledTaskConfig{
					Enabled:  viper.GetBool("platforms.temu.inventorySync.enabled"),
					Interval: viper.GetInt("platforms.temu.inventorySync.interval"),
				},
				ActivityRegistration: ScheduledTaskConfig{
					Enabled:  viper.GetBool("platforms.temu.activityRegistration.enabled"),
					Interval: viper.GetInt("platforms.temu.activityRegistration.interval"),
				},
				SyncProduct: SyncProductConfig{
					Enabled:   viper.GetBool("platforms.temu.sync.enabled"),
					StoreIDs:  getInt64Slice("platforms.temu.sync.storeIDs"),
					Interval:  viper.GetInt("platforms.temu.sync.interval"),
					BatchSize: viper.GetInt("platforms.temu.sync.batchSize"),
				},
				Monitor: MonitorConfig{
					Enabled:              viper.GetBool("platforms.temu.monitor.enabled"),
					StoreIDs:             getInt64Slice("platforms.temu.monitor.storeIDs"),
					CheckInterval:        viper.GetInt("platforms.temu.monitor.checkInterval"),
					BatchSize:            viper.GetInt("platforms.temu.monitor.batchSize"),
					EnablePriceAlert:     viper.GetBool("platforms.temu.monitor.enablePriceAlert"),
					EnableStockAlert:     viper.GetBool("platforms.temu.monitor.enableStockAlert"),
					PriceChangeThreshold: viper.GetFloat64("platforms.temu.monitor.priceChangeThreshold"),
					StockChangeThreshold: viper.GetInt("platforms.temu.monitor.stockChangeThreshold"),
				},
			},
			Shein: PlatformConfig{
				Enabled:          viper.GetBool("platforms.shein.enabled"),
				SchedulerEnabled: viper.GetBool("platforms.shein.schedulerEnabled"),
				AutoPricing: AutoPricingConfig{
					Enabled:   viper.GetBool("platforms.shein.autoPricing.enabled"),
					Interval:  viper.GetInt("platforms.shein.autoPricing.interval"),
					BatchSize: viper.GetInt("platforms.shein.autoPricing.batchSize"),
				},
				ProductSync: ScheduledTaskConfig{
					Enabled:  viper.GetBool("platforms.shein.productSync.enabled"),
					Interval: viper.GetInt("platforms.shein.productSync.interval"),
				},
				InventorySync: ScheduledTaskConfig{
					Enabled:  viper.GetBool("platforms.shein.inventorySync.enabled"),
					Interval: viper.GetInt("platforms.shein.inventorySync.interval"),
				},
				ActivityRegistration: ScheduledTaskConfig{
					Enabled:  viper.GetBool("platforms.shein.activityRegistration.enabled"),
					Interval: viper.GetInt("platforms.shein.activityRegistration.interval"),
				},
				SyncProduct: SyncProductConfig{
					Enabled:   viper.GetBool("platforms.shein.sync.enabled"),
					StoreIDs:  getInt64Slice("platforms.shein.sync.storeIDs"),
					Interval:  viper.GetInt("platforms.shein.sync.interval"),
					BatchSize: viper.GetInt("platforms.shein.sync.batchSize"),
				},
				Monitor: MonitorConfig{
					Enabled:              viper.GetBool("platforms.shein.monitor.enabled"),
					StoreIDs:             getInt64Slice("platforms.shein.monitor.storeIDs"),
					CheckInterval:        viper.GetInt("platforms.shein.monitor.checkInterval"),
					BatchSize:            viper.GetInt("platforms.shein.monitor.batchSize"),
					EnablePriceAlert:     viper.GetBool("platforms.shein.monitor.enablePriceAlert"),
					EnableStockAlert:     viper.GetBool("platforms.shein.monitor.enableStockAlert"),
					PriceChangeThreshold: viper.GetFloat64("platforms.shein.monitor.priceChangeThreshold"),
					StockChangeThreshold: viper.GetInt("platforms.shein.monitor.stockChangeThreshold"),
				},
			},
			Alibaba1688: Alibaba1688Config{
				Enabled:  viper.GetBool("platforms.alibaba1688.enabled"),
				Timeout:  viper.GetInt("platforms.alibaba1688.timeout"),
				PoolSize: viper.GetInt("platforms.alibaba1688.poolSize"),
			},
		},
		Browser: BrowserConfig{
			Enabled:        viper.GetBool("browser.enabled"),
			Headless:       viper.GetBool("browser.headless"),
			BrowserPath:    viper.GetString("browser.browserPath"),
			PoolSize:       viper.GetInt("browser.poolSize"),
			ViewportWidth:  viper.GetInt("browser.viewportWidth"),
			ViewportHeight: viper.GetInt("browser.viewportHeight"),
			ProxyServer:    viper.GetString("browser.proxyServer"),
			RandomConfig: BrowserRandomConfig{
				Enabled:             viper.GetBool("browser.randomConfig.enabled"),
				Strategy:            viper.GetString("browser.randomConfig.strategy"),
				PresetName:          viper.GetString("browser.randomConfig.presetName"),
				FingerprintStrategy: viper.GetString("browser.randomConfig.fingerprintStrategy"),
				HealthCheckEnabled:  viper.GetBool("browser.randomConfig.healthCheckEnabled"),
				MaxRetries:          viper.GetInt("browser.randomConfig.maxRetries"),
			},
		},
		Amazon: AmazonConfig{
			Enabled:           viper.GetBool("amazon.enabled"),
			Zipcodes:          viper.GetStringMapString("amazon.zipcodes"),
			DataFreshnessDays: viper.GetInt("amazon.dataFreshnessDays"),
			CrawlTimeout:      viper.GetInt("amazon.crawlTimeout"),

			// 从浏览器配置复制兼容性字段
			Headless:       viper.GetBool("browser.headless"),
			BrowserPath:    viper.GetString("browser.browserPath"),
			PoolSize:       viper.GetInt("browser.poolSize"),
			ViewportWidth:  viper.GetInt("browser.viewportWidth"),
			ViewportHeight: viper.GetInt("browser.viewportHeight"),
			ProxyServer:    viper.GetString("browser.proxyServer"),
			RandomConfig: BrowserRandomConfig{
				Enabled:             viper.GetBool("browser.randomConfig.enabled"),
				Strategy:            viper.GetString("browser.randomConfig.strategy"),
				PresetName:          viper.GetString("browser.randomConfig.presetName"),
				FingerprintStrategy: viper.GetString("browser.randomConfig.fingerprintStrategy"),
				HealthCheckEnabled:  viper.GetBool("browser.randomConfig.healthCheckEnabled"),
				MaxRetries:          viper.GetInt("browser.randomConfig.maxRetries"),
			},
		},
		Updater: UpdaterConfig{
			Enabled:            viper.GetBool("updater.enabled"),
			UpdateURL:          viper.GetString("updater.updateURL"),
			CheckInterval:      viper.GetInt("updater.checkInterval"),
			InsecureSkipVerify: viper.GetBool("updater.insecureSkipVerify"),
		},
	}

	// 构建RabbitMQ配置（可选）
	if viper.GetBool("rabbitmq.enabled") {
		cfg.RabbitMQ = &RabbitMQConfig{
			Enabled:           viper.GetBool("rabbitmq.enabled"),
			URL:               viper.GetString("rabbitmq.url"),
			ReconnectInterval: viper.GetInt("rabbitmq.reconnectInterval"),
			MaxReconnectTries: viper.GetInt("rabbitmq.maxReconnectTries"),
		}
	}

	return cfg
}
