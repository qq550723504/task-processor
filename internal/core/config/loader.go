// Package config 提供配置加载功能
package config

import (
	"fmt"
	"os"
	"task-processor/internal/core/config/types"

	"github.com/sirupsen/logrus"
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

// LoadConfigWithFallback 加载配置，失败时使用默认配置（统一入口）
// 用于替代各个cmd中重复的配置加载逻辑
func LoadConfigWithFallback(configPath string, logger *logrus.Logger) *Config {
	// 检查配置文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if logger != nil {
			logger.Warnf("⚠️  配置文件不存在: %s，使用默认配置", configPath)
		}
		return NewDefaultConfig()
	}

	// 加载配置
	if logger != nil {
		logger.Infof("📄 加载应用配置: %s", configPath)
	}

	cfg := LoadConfigFromFile(configPath)
	if cfg == nil {
		if logger != nil {
			logger.Warn("⚠️  配置加载失败，使用默认配置")
		}
		return NewDefaultConfig()
	}

	if logger != nil {
		logger.Info("✅ 配置加载成功")
	}
	return cfg
}

// NewDefaultConfig 创建默认配置
// NewDefaultConfig 创建默认配置
func NewDefaultConfig() *Config {
	return &Config{
		Config: &types.Config{
			Processor: types.ProcessorConfig{
				MaxRetries: 3,
				Timeout:    300,
			},
			Worker: types.WorkerConfig{
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
			OpenAI: types.OpenAIConfig{
				APIKey:  "sk-yJ3RQprPLyBcoqEkeBNimE6Dhj86CAjY2uHAc5yqLZd1KHa3",
				Model:   "gemini-2.0-flash",
				BaseURL: "https://ai.linkcloudai.com/v1",
				Timeout: 120,
			},
			Management: types.ManagementConfig{
				BaseURL:      "http://getway.linkcloudai.com",
				ClientID:     "go-listing",
				ClientSecret: "go-listing-secret",
				TokenURL:     "http://getway.linkcloudai.com/admin-api/system/oauth2/token",
				Scopes:       []string{"user.read"},
				TenantID:     "1",
			},
			Browser: types.BrowserConfig{
				Enabled:        true,
				Headless:       true,
				BrowserPath:    "./chrome/chrome.exe",
				PoolSize:       3,
				ViewportWidth:  1920,
				ViewportHeight: 1080,
				RandomConfig: types.BrowserRandomConfig{
					Enabled:             true,
					Strategy:            "random",
					PresetName:          "windows_high_end",
					FingerprintStrategy: "random",
					HealthCheckEnabled:  true,
					MaxRetries:          3,
				},
			},
			Amazon: types.AmazonConfig{
				Enabled:           true,
				DataFreshnessDays: 7,
			},
			Updater: types.UpdaterConfig{
				Enabled:            false,
				UpdateURL:          "https://auto-update-1303159911.cos.ap-shanghai.myqcloud.com/task-processor/version.json",
				CheckInterval:      300,
				InsecureSkipVerify: false,
			},
			Platforms: types.PlatformsConfig{
				Temu: types.PlatformConfig{
					Enabled:          false,
					SchedulerEnabled: false,
					AutoPricing: types.AutoPricingConfig{
						Enabled:        false,
						Interval:       300,
						BatchSize:      100,
						UseAmazonPrice: false,
					},
					ProductSync: types.ScheduledTaskConfig{
						Enabled:  false,
						Interval: 3600,
					},
					InventorySync: types.ScheduledTaskConfig{
						Enabled:  false,
						Interval: 1800,
					},
					ActivityRegistration: types.ScheduledTaskConfig{
						Enabled:  false,
						Interval: 7200,
					},
					SyncProduct: types.SyncProductConfig{
						Enabled:   true,
						StoreIDs:  []int64{},
						Interval:  60,
						BatchSize: 50,
					},
					Monitor: types.MonitorConfig{
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
				Shein: types.PlatformConfig{
					Enabled:          false,
					SchedulerEnabled: false,
					AutoPricing: types.AutoPricingConfig{
						Enabled:   false,
						Interval:  300,
						BatchSize: 100,
					},
					ProductSync: types.ScheduledTaskConfig{
						Enabled:  false,
						Interval: 3600,
					},
					InventorySync: types.ScheduledTaskConfig{
						Enabled:  false,
						Interval: 1800,
					},
					ActivityRegistration: types.ScheduledTaskConfig{
						Enabled:  false,
						Interval: 7200,
					},
					SyncProduct: types.SyncProductConfig{
						Enabled:   true,
						StoreIDs:  []int64{},
						Interval:  60,
						BatchSize: 50,
					},
					Monitor: types.MonitorConfig{
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
		},
	}
}

// applyDefaults 应用默认配置值
func applyDefaults(cfg *Config) {
	// Config现在是包装类型,不需要应用默认值
	// 默认值已经在NewDefaultConfig中设置
}
