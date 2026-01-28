// Package config 提供配置加载功能
package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// LoadFromBytes 从字节数据加载配置
func LoadFromBytes(data []byte) (*Config, error) {
	if len(data) == 0 {
		// 如果没有配置数据，返回默认配置
		return NewDefaultConfig(), nil
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析YAML配置失败: %w", err)
	}

	// 应用默认值
	applyDefaults(&cfg)

	return &cfg, nil
}

// NewDefaultConfig 创建默认配置
func NewDefaultConfig() *Config {
	return &Config{
		Processor: ProcessorConfig{
			MaxRetries: 3,
			Timeout:    300,
		},
		Worker: WorkerConfig{
			Concurrency:        10,
			BufferSize:         100,
			TaskInterval:       60,
			MaxFetchPerCycle:   5,
			QueueThreshold:     75,
			CleanupInterval:    120,
			TaskTimeout:        900,
			StuckTaskThreshold: 300,
			ForceCleanupAfter:  1800,
		},
		OpenAI: OpenAIConfig{
			APIKey:  "sk-yJ3RQprPLyBcoqEkeBNimE6Dhj86CAjY2uHAc5yqLZd1KHa3",
			Model:   "gemini-2.0-flash",
			BaseURL: "https://ai.linkcloudai.com/v1",
			Timeout: 120,
		},
		Management: ManagementConfig{
			BaseURL:      "http://getway.linkcloudai.com",
			ClientID:     "go-listing",
			ClientSecret: "go-listing-secret",
			TokenURL:     "http://getway.linkcloudai.com/admin-api/system/oauth2/token",
			Scopes:       []string{"user.read"},
			TenantID:     "1",
		},
		Browser: BrowserConfig{
			Enabled:        true,
			Headless:       true,
			BrowserPath:    "./chrome/chrome.exe",
			PoolSize:       3,
			ViewportWidth:  1920,
			ViewportHeight: 1080,
			RandomConfig: BrowserRandomConfig{
				Enabled:             true,
				Strategy:            "random",
				PresetName:          "windows_high_end",
				FingerprintStrategy: "random",
				HealthCheckEnabled:  true,
				MaxRetries:          3,
			},
		},
		Amazon: AmazonConfig{
			Enabled:           true,
			DataFreshnessDays: 7,
		},
		Updater: UpdaterConfig{
			Enabled:            false,
			UpdateURL:          "https://auto-update-1303159911.cos.ap-shanghai.myqcloud.com/task-processor/version.json",
			CheckInterval:      300,
			InsecureSkipVerify: false,
		},
		Platforms: PlatformsConfig{
			Temu: PlatformConfig{
				Enabled:          false,
				SchedulerEnabled: false,
				AutoPricing: AutoPricingConfig{
					Enabled:        false,
					Interval:       300,
					BatchSize:      100,
					UseAmazonPrice: false,
				},
				ProductSync: ScheduledTaskConfig{
					Enabled:  false,
					Interval: 3600,
				},
				InventorySync: ScheduledTaskConfig{
					Enabled:  false,
					Interval: 1800,
				},
				ActivityRegistration: ScheduledTaskConfig{
					Enabled:  false,
					Interval: 7200,
				},
				SyncProduct: SyncProductConfig{
					Enabled:   true,
					StoreIDs:  []int64{},
					Interval:  60,
					BatchSize: 50,
				},
				Monitor: MonitorConfig{
					Enabled:              true,
					StoreIDs:             []int64{},
					CheckInterval:        1440,
					BatchSize:            100,
					EnablePriceAlert:     true,
					EnableStockAlert:     true,
					PriceChangeThreshold: 10.0,
					StockChangeThreshold: 5,
				},
			},
			Shein: PlatformConfig{
				Enabled:          false,
				SchedulerEnabled: false,
				AutoPricing: AutoPricingConfig{
					Enabled:   false,
					Interval:  300,
					BatchSize: 100,
				},
				ProductSync: ScheduledTaskConfig{
					Enabled:  false,
					Interval: 3600,
				},
				InventorySync: ScheduledTaskConfig{
					Enabled:  false,
					Interval: 1800,
				},
				ActivityRegistration: ScheduledTaskConfig{
					Enabled:  false,
					Interval: 7200,
				},
				SyncProduct: SyncProductConfig{
					Enabled:   true,
					StoreIDs:  []int64{},
					Interval:  60,
					BatchSize: 50,
				},
				Monitor: MonitorConfig{
					Enabled:              true,
					StoreIDs:             []int64{},
					CheckInterval:        1440,
					BatchSize:            100,
					EnablePriceAlert:     true,
					EnableStockAlert:     true,
					PriceChangeThreshold: 10.0,
					StockChangeThreshold: 5,
				},
			},
		},
	}
}

// applyDefaults 应用默认配置值
func applyDefaults(cfg *Config) {
	// 获取默认配置
	defaultCfg := NewDefaultConfig()

	// 应用Worker默认值
	if cfg.Worker.Concurrency == 0 {
		cfg.Worker.Concurrency = defaultCfg.Worker.Concurrency
	}
	if cfg.Worker.BufferSize == 0 {
		cfg.Worker.BufferSize = defaultCfg.Worker.BufferSize
	}
	if cfg.Worker.TaskInterval == 0 {
		cfg.Worker.TaskInterval = defaultCfg.Worker.TaskInterval
	}
	if cfg.Worker.MaxFetchPerCycle == 0 {
		cfg.Worker.MaxFetchPerCycle = defaultCfg.Worker.MaxFetchPerCycle
	}
	if cfg.Worker.QueueThreshold == 0 {
		cfg.Worker.QueueThreshold = defaultCfg.Worker.QueueThreshold
	}
	if cfg.Worker.CleanupInterval == 0 {
		cfg.Worker.CleanupInterval = defaultCfg.Worker.CleanupInterval
	}
	if cfg.Worker.TaskTimeout == 0 {
		cfg.Worker.TaskTimeout = defaultCfg.Worker.TaskTimeout
	}
	if cfg.Worker.StuckTaskThreshold == 0 {
		cfg.Worker.StuckTaskThreshold = defaultCfg.Worker.StuckTaskThreshold
	}
	if cfg.Worker.ForceCleanupAfter == 0 {
		cfg.Worker.ForceCleanupAfter = defaultCfg.Worker.ForceCleanupAfter
	}

	// 应用Processor默认值
	if cfg.Processor.MaxRetries == 0 {
		cfg.Processor.MaxRetries = defaultCfg.Processor.MaxRetries
	}
	if cfg.Processor.Timeout == 0 {
		cfg.Processor.Timeout = defaultCfg.Processor.Timeout
	}

	// 应用Management默认值
	if cfg.Management.BaseURL == "" {
		cfg.Management.BaseURL = defaultCfg.Management.BaseURL
	}
	if cfg.Management.ClientID == "" {
		cfg.Management.ClientID = defaultCfg.Management.ClientID
	}
	if cfg.Management.ClientSecret == "" {
		cfg.Management.ClientSecret = defaultCfg.Management.ClientSecret
	}
	if cfg.Management.TokenURL == "" {
		cfg.Management.TokenURL = defaultCfg.Management.TokenURL
	}
	if len(cfg.Management.Scopes) == 0 {
		cfg.Management.Scopes = defaultCfg.Management.Scopes
	}
	if cfg.Management.TenantID == "" {
		cfg.Management.TenantID = defaultCfg.Management.TenantID
	}

	// 应用浏览器默认配置
	if cfg.Browser.PoolSize == 0 {
		cfg.Browser.PoolSize = defaultCfg.Browser.PoolSize
	}
	if cfg.Browser.ViewportWidth == 0 {
		cfg.Browser.ViewportWidth = defaultCfg.Browser.ViewportWidth
	}
	if cfg.Browser.ViewportHeight == 0 {
		cfg.Browser.ViewportHeight = defaultCfg.Browser.ViewportHeight
	}

	// 应用Amazon配置默认值
	if cfg.Amazon.DataFreshnessDays == 0 {
		cfg.Amazon.DataFreshnessDays = defaultCfg.Amazon.DataFreshnessDays
	}

	// 应用更新器默认配置
	if cfg.Updater.CheckInterval == 0 {
		cfg.Updater.CheckInterval = defaultCfg.Updater.CheckInterval
	}

	// 应用OpenAI默认配置
	if cfg.OpenAI.APIKey == "" {
		cfg.OpenAI.APIKey = defaultCfg.OpenAI.APIKey
	}
	if cfg.OpenAI.Model == "" {
		cfg.OpenAI.Model = defaultCfg.OpenAI.Model
	}
	if cfg.OpenAI.BaseURL == "" {
		cfg.OpenAI.BaseURL = defaultCfg.OpenAI.BaseURL
	}
	if cfg.OpenAI.Timeout == 0 {
		cfg.OpenAI.Timeout = defaultCfg.OpenAI.Timeout
	}
}
