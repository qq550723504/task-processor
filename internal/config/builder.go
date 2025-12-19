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
			Concurrency:      viper.GetInt("worker.concurrency"),
			BufferSize:       viper.GetInt("worker.bufferSize"),
			TaskInterval:     viper.GetInt("worker.taskInterval"),
			MaxFetchPerCycle: viper.GetInt("worker.maxFetchPerCycle"),
			QueueThreshold:   viper.GetInt("worker.queueThreshold"),
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
				AutoPricing: AutoPricingConfig{
					Enabled:   viper.GetBool("platforms.temu.autoPricing.enabled"),
					Interval:  viper.GetInt("platforms.temu.autoPricing.interval"),
					BatchSize: viper.GetInt("platforms.temu.autoPricing.batchSize"),
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
		},
		Amazon: AmazonConfig{
			Enabled:           viper.GetBool("amazon.enabled"),
			Headless:          viper.GetBool("amazon.headless"),
			BrowserPath:       viper.GetString("amazon.browserPath"),
			PoolSize:          viper.GetInt("amazon.poolSize"),
			Zipcodes:          viper.GetStringMapString("amazon.zipcodes"),
			ViewportWidth:     viper.GetInt("amazon.viewportWidth"),
			ViewportHeight:    viper.GetInt("amazon.viewportHeight"),
			ProxyServer:       viper.GetString("amazon.proxyServer"),
			DataFreshnessDays: viper.GetInt("amazon.dataFreshnessDays"),
			SPAPI:             loadSPAPIConfig(),
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
