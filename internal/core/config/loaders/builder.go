// Package loaders 提供配置加载功能
package loaders

import (
	"task-processor/internal/core/config/types"

	"github.com/spf13/viper"
)

// BuildConfig 构建配置对象
func BuildConfig() *types.Config {
	cfg := &types.Config{
		Processor: types.ProcessorConfig{
			MaxRetries: viper.GetInt("processor.maxRetries"),
			Timeout:    viper.GetInt("processor.timeout"),
		},
		Worker: types.WorkerConfig{
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
		OpenAI: types.OpenAIConfig{
			APIKey:  viper.GetString("openai.apiKey"),
			Model:   viper.GetString("openai.model"),
			BaseURL: viper.GetString("openai.baseURL"),
			Timeout: viper.GetInt("openai.timeout"),
		},
		Management: types.ManagementConfig{
			BaseURL:      viper.GetString("management.baseURL"),
			ClientID:     viper.GetString("management.clientID"),
			ClientSecret: viper.GetString("management.clientSecret"),
			TokenURL:     viper.GetString("management.tokenURL"),
			Scopes:       viper.GetStringSlice("management.scopes"),
			TenantID:     viper.GetString("management.tenantID"),
			UserID:       viper.GetInt64("management.userID"),
			StoreIDs:     getInt64Slice("management.storeIDs"),
		},
		Platforms: types.PlatformsConfig{
			Temu:  BuildPlatformConfig("platforms.temu"),
			Shein: BuildPlatformConfig("platforms.shein"),
			Alibaba1688: types.Alibaba1688Config{
				Enabled:  viper.GetBool("platforms.alibaba1688.enabled"),
				Timeout:  viper.GetInt("platforms.alibaba1688.timeout"),
				PoolSize: viper.GetInt("platforms.alibaba1688.poolSize"),
			},
		},
		Browser: types.BrowserConfig{
			Enabled:        viper.GetBool("browser.enabled"),
			Headless:       viper.GetBool("browser.headless"),
			BrowserPath:    viper.GetString("browser.browserPath"),
			PoolSize:       viper.GetInt("browser.poolSize"),
			ViewportWidth:  viper.GetInt("browser.viewportWidth"),
			ViewportHeight: viper.GetInt("browser.viewportHeight"),
			ProxyServer:    viper.GetString("browser.proxyServer"),
			RandomConfig: types.BrowserRandomConfig{
				Enabled:             viper.GetBool("browser.randomConfig.enabled"),
				Strategy:            viper.GetString("browser.randomConfig.strategy"),
				PresetName:          viper.GetString("browser.randomConfig.presetName"),
				FingerprintStrategy: viper.GetString("browser.randomConfig.fingerprintStrategy"),
				HealthCheckEnabled:  viper.GetBool("browser.randomConfig.healthCheckEnabled"),
				MaxRetries:          viper.GetInt("browser.randomConfig.maxRetries"),
			},
		},
		Amazon: types.AmazonConfig{
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
			RandomConfig: types.BrowserRandomConfig{
				Enabled:             viper.GetBool("browser.randomConfig.enabled"),
				Strategy:            viper.GetString("browser.randomConfig.strategy"),
				PresetName:          viper.GetString("browser.randomConfig.presetName"),
				FingerprintStrategy: viper.GetString("browser.randomConfig.fingerprintStrategy"),
				HealthCheckEnabled:  viper.GetBool("browser.randomConfig.healthCheckEnabled"),
				MaxRetries:          viper.GetInt("browser.randomConfig.maxRetries"),
			},
		},
		Updater: types.UpdaterConfig{
			Enabled:            viper.GetBool("updater.enabled"),
			UpdateURL:          viper.GetString("updater.updateURL"),
			CheckInterval:      viper.GetInt("updater.checkInterval"),
			InsecureSkipVerify: viper.GetBool("updater.insecureSkipVerify"),
		},
	}

	// 构建RabbitMQ配置（可选）
	if viper.GetBool("rabbitmq.enabled") {
		cfg.RabbitMQ = BuildRabbitMQConfig()
	}

	return cfg
}
