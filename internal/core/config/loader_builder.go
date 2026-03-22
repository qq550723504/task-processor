// Package loaders 提供配置加载功能
package config

import (
	"task-processor/internal/core/logger"
	"time"

	"github.com/spf13/viper"
)

// BuildConfig 构建配置对象
func BuildConfig() *Config {
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
			Clients: buildOpenAIClients(),
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
			Temu:  BuildPlatformConfig("platforms.temu"),
			Shein: BuildPlatformConfig("platforms.shein"),
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
		cfg.RabbitMQ = BuildRabbitMQConfig()
	}

	// 构建数据库配置（可选，host 不为空时才构建）
	if viper.GetString("database.host") != "" {
		cfg.Database = &DatabaseConfig{
			Host:                  viper.GetString("database.host"),
			Port:                  viper.GetInt("database.port"),
			User:                  viper.GetString("database.user"),
			Password:              viper.GetString("database.password"),
			Database:              viper.GetString("database.database"),
			MaxConnections:        viper.GetInt("database.max_connections"),
			MaxIdleConnections:    viper.GetInt("database.max_idle_connections"),
			ConnectionMaxLifetime: time.Duration(viper.GetInt64("database.connection_max_lifetime")),
		}
	}

	// 构建日志配置
	cfg.Logging = LoggingConfig{
		Level:        viper.GetString("logging.level"),
		Format:       viper.GetString("logging.format"),
		File:         viper.GetString("logging.file"),
		SplitByLevel: buildSplitByLevelConfig(),
	}

	return cfg
}

// buildOpenAIClients 从 viper 读取 openai.clients.* 配置，构建命名客户端 map。
// 未配置的字段（apiKey/baseURL/timeout）继承顶层默认值。
func buildOpenAIClients() map[string]OpenAIClientConfig {
	raw := viper.GetStringMap("openai.clients")
	if len(raw) == 0 {
		return nil
	}

	defaultKey := viper.GetString("openai.apiKey")
	defaultBase := viper.GetString("openai.baseURL")
	defaultTimeout := viper.GetInt("openai.timeout")

	clients := make(map[string]OpenAIClientConfig, len(raw))
	for name := range raw {
		prefix := "openai.clients." + name

		apiKey := viper.GetString(prefix + ".apiKey")
		if apiKey == "" {
			apiKey = defaultKey
		}
		baseURL := viper.GetString(prefix + ".baseURL")
		if baseURL == "" {
			baseURL = defaultBase
		}
		timeout := viper.GetInt(prefix + ".timeout")
		if timeout == 0 {
			timeout = defaultTimeout
		}

		clients[name] = OpenAIClientConfig{
			APIKey:  apiKey,
			Model:   viper.GetString(prefix + ".model"),
			BaseURL: baseURL,
			Timeout: timeout,
		}
	}
	return clients
}

// buildSplitByLevelConfig 从 viper 读取 logging.split_by_level 配置
func buildSplitByLevelConfig() []logger.LevelFileConfig {
	raw := viper.Get("logging.split_by_level")
	if raw == nil {
		return nil
	}

	list, ok := raw.([]any)
	if !ok {
		return nil
	}

	configs := make([]logger.LevelFileConfig, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		cfg := logger.LevelFileConfig{
			File: getStringFromMap(m, "file"),
		}

		if levelsRaw, ok := m["levels"]; ok {
			switch v := levelsRaw.(type) {
			case []any:
				for _, l := range v {
					if s, ok := l.(string); ok {
						cfg.Levels = append(cfg.Levels, s)
					}
				}
			case []string:
				cfg.Levels = v
			}
		}

		if cfg.File != "" && len(cfg.Levels) > 0 {
			configs = append(configs, cfg)
		}
	}

	return configs
}
