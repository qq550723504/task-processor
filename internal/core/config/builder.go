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
				Enabled: viper.GetBool("platforms.temu.enabled"),
				AutoPricing: AutoPricingConfig{
					Enabled:        viper.GetBool("platforms.temu.autoPricing.enabled"),
					Interval:       viper.GetInt("platforms.temu.autoPricing.interval"),
					BatchSize:      viper.GetInt("platforms.temu.autoPricing.batchSize"),
					UseAmazonPrice: viper.GetBool("platforms.temu.autoPricing.useAmazonPrice"),
				},
				Sync: SyncConfig{
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
				Enabled: viper.GetBool("platforms.shein.enabled"),
				AutoPricing: AutoPricingConfig{
					Enabled:   viper.GetBool("platforms.shein.autoPricing.enabled"),
					Interval:  viper.GetInt("platforms.shein.autoPricing.interval"),
					BatchSize: viper.GetInt("platforms.shein.autoPricing.batchSize"),
				},
				Sync: SyncConfig{
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
				BrowserConfig: BrowserConfig{
					Enabled:        viper.GetBool("platforms.alibaba1688.browserConfig.enabled"),
					Headless:       viper.GetBool("platforms.alibaba1688.browserConfig.headless"),
					BrowserPath:    viper.GetString("platforms.alibaba1688.browserConfig.browserPath"),
					PoolSize:       viper.GetInt("platforms.alibaba1688.browserConfig.poolSize"),
					ViewportWidth:  viper.GetInt("platforms.alibaba1688.browserConfig.viewportWidth"),
					ViewportHeight: viper.GetInt("platforms.alibaba1688.browserConfig.viewportHeight"),
					ProxyServer:    viper.GetString("platforms.alibaba1688.browserConfig.proxyServer"),
					RandomConfig: BrowserRandomConfig{
						Enabled:             viper.GetBool("platforms.alibaba1688.browserConfig.randomConfig.enabled"),
						Strategy:            viper.GetString("platforms.alibaba1688.browserConfig.randomConfig.strategy"),
						PresetName:          viper.GetString("platforms.alibaba1688.browserConfig.randomConfig.presetName"),
						FingerprintStrategy: viper.GetString("platforms.alibaba1688.browserConfig.randomConfig.fingerprintStrategy"),
						HealthCheckEnabled:  viper.GetBool("platforms.alibaba1688.browserConfig.randomConfig.healthCheckEnabled"),
						MaxRetries:          viper.GetInt("platforms.alibaba1688.browserConfig.randomConfig.maxRetries"),
					},
				},
				RandomConfig: BrowserRandomConfig{
					Enabled:             viper.GetBool("platforms.alibaba1688.randomConfig.enabled"),
					Strategy:            viper.GetString("platforms.alibaba1688.randomConfig.strategy"),
					PresetName:          viper.GetString("platforms.alibaba1688.randomConfig.presetName"),
					FingerprintStrategy: viper.GetString("platforms.alibaba1688.randomConfig.fingerprintStrategy"),
					HealthCheckEnabled:  viper.GetBool("platforms.alibaba1688.randomConfig.healthCheckEnabled"),
					MaxRetries:          viper.GetInt("platforms.alibaba1688.randomConfig.maxRetries"),
				},
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
			SPAPI:             loadSPAPIConfig(),

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

	return cfg
}
