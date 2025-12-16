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
		AutoPricing: AutoPricingConfig{
			Temu: PlatformAutoPricingConfig{
				Enabled:   viper.GetBool("autoPricing.temu.enabled"),
				Interval:  viper.GetInt("autoPricing.temu.interval"),
				BatchSize: viper.GetInt("autoPricing.temu.batchSize"),
			},
			Shein: PlatformAutoPricingConfig{
				Enabled:   viper.GetBool("autoPricing.shein.enabled"),
				Interval:  viper.GetInt("autoPricing.shein.interval"),
				BatchSize: viper.GetInt("autoPricing.shein.batchSize"),
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

	// 加载同步配置（如果存在）
	if viper.IsSet("sync") {
		cfg.Sync = &SyncConfig{
			Enabled:  viper.GetBool("sync.enabled"),
			StoreIDs: getInt64Slice("sync.storeIDs"),
		}
	}

	// 加载监控配置（如果存在）
	if viper.IsSet("monitor") {
		cfg.Monitor = &MonitorConfig{
			Enabled:              viper.GetBool("monitor.enabled"),
			StoreIDs:             getInt64Slice("monitor.storeIDs"),
			CheckInterval:        viper.GetInt("monitor.checkInterval"),
			BatchSize:            viper.GetInt("monitor.batchSize"),
			EnablePriceAlert:     viper.GetBool("monitor.enablePriceAlert"),
			EnableStockAlert:     viper.GetBool("monitor.enableStockAlert"),
			PriceChangeThreshold: viper.GetFloat64("monitor.priceChangeThreshold"),
			StockChangeThreshold: viper.GetInt("monitor.stockChangeThreshold"),
		}
	}

	return cfg
}
