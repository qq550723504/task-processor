package config

import (
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func LoadFromBytes(data []byte) (*Config, error) {
	if len(data) == 0 {
		cfg := NewDefaultConfig()
		applyEnvOverrides(cfg)
		if err := cfg.ValidateWithError(); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse yaml config: %w", err)
	}

	applyDefaults(cfg)
	applyEnvOverrides(cfg)
	if err := cfg.ValidateWithError(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func LoadConfigWithFallback(configPath string, logger *logrus.Logger) (*Config, error) {
	if logger != nil {
		logger.Infof("received config path: %q", configPath)
	}

	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file does not exist: %s", configPath)
		}
		return nil, fmt.Errorf("stat config file %s: %w", configPath, err)
	}

	cfg, err := LoadConfigFromFile(configPath)
	if err != nil {
		return nil, err
	}

	if logger != nil {
		logger.Infof("config loaded successfully")
		logger.Debugf("   - Management.BaseURL: %s", cfg.Management.BaseURL)
		logger.Debugf("   - RabbitMQ.Enabled: %v", cfg.RabbitMQ != nil && cfg.RabbitMQ.Enabled)
		if cfg.RabbitMQ != nil {
			logger.Debugf("   - RabbitMQ.URL: %s", cfg.RabbitMQ.URL)
		}
	}

	return cfg, nil
}

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
			APIKey:  "",
			Model:   "gemini-2.0-flash",
			BaseURL: "https://ai.linkcloudai.com/v1",
			Timeout: 120,
		},
		Management: ManagementConfig{
			BaseURL:      "http://getway.linkcloudai.com",
			ClientID:     "go-listing",
			ClientSecret: "",
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
			Enabled:           false,
			DataFreshnessDays: 7,
			SPAPI: SPAPIConfig{
				Enabled:                false,
				Region:                 "us-east-1",
				DefaultMarketplace:     "ATVPDKIKX0DER",
				DefaultFulfillmentType: "FBM",
				DefaultCondition:       "New",
			},
		},
		ProductImage: ProductImageConfig{
			WorkDir: "./tmp/productimage",
			Segmenter: ProductImageModelConfig{
				Enabled: false,
				Timeout: 45,
			},
			WhiteBackground: ProductImageModelConfig{
				Enabled: false,
				Timeout: 45,
			},
			Publisher: ProductImagePublisherConfig{
				Enabled:    true,
				Provider:   "local",
				OutputDir:  "./tmp/productimage-published",
				PublicBase: "",
			},
			Lifecycle: ProductImageLifecycleConfig{
				CleanupTemporaryFiles: true,
				ReuseExistingAssets:   true,
			},
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
				Monitor: MonitorConfig{
					Enabled:              false,
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
				Monitor: MonitorConfig{
					Enabled:              false,
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
		RabbitMQ: &RabbitMQConfig{
			Enabled:           false,
			URL:               "amqp://guest:guest@localhost:5672/",
			ReconnectInterval: 5 * time.Second,
			MaxReconnectTries: 10,
			Consumer: RabbitMQConsumerConfig{
				PrefetchCount: 5,
				PrefetchSize:  0,
				RetryDelay:    5 * time.Second,
				MaxRetries:    3,
				Queues:        []QueueConfig{},
			},
			Node: NodeConfig{
				MaxConcurrency:  10,
				HealthCheckPort: 8081,
				MetricsPort:     8082,
				LogLevel:        "info",
				ShutdownTimeout: 30 * time.Second,
			},
		},
	}
}

func applyDefaults(cfg *Config) {
	defaultCfg := NewDefaultConfig()

	if cfg.Browser.BrowserPath == "" {
		cfg.Browser = defaultCfg.Browser
	}
	if cfg.Worker.Concurrency == 0 {
		cfg.Worker = defaultCfg.Worker
	}
	if cfg.Processor.MaxRetries == 0 {
		cfg.Processor = defaultCfg.Processor
	}
	if cfg.OpenAI.APIKey == "" {
		cfg.OpenAI.Model = defaultCfg.OpenAI.Model
		if cfg.OpenAI.BaseURL == "" {
			cfg.OpenAI.BaseURL = defaultCfg.OpenAI.BaseURL
		}
		if cfg.OpenAI.Timeout == 0 {
			cfg.OpenAI.Timeout = defaultCfg.OpenAI.Timeout
		}
	}
	if cfg.Management.BaseURL == "" {
		cfg.Management.BaseURL = defaultCfg.Management.BaseURL
	}
	if cfg.Management.ClientID == "" {
		cfg.Management.ClientID = defaultCfg.Management.ClientID
	}
	if cfg.Management.TenantID == "" {
		cfg.Management.TenantID = defaultCfg.Management.TenantID
	}
	if cfg.Management.TokenURL == "" {
		cfg.Management.TokenURL = defaultCfg.Management.TokenURL
	}
	if !cfg.Amazon.Enabled && cfg.Amazon.DataFreshnessDays == 0 {
		cfg.Amazon = defaultCfg.Amazon
	}
	if cfg.ProductImage.WorkDir == "" {
		cfg.ProductImage.WorkDir = defaultCfg.ProductImage.WorkDir
	}
	if cfg.ProductImage.Segmenter.Timeout == 0 {
		cfg.ProductImage.Segmenter.Timeout = defaultCfg.ProductImage.Segmenter.Timeout
	}
	if cfg.ProductImage.WhiteBackground.Timeout == 0 {
		cfg.ProductImage.WhiteBackground.Timeout = defaultCfg.ProductImage.WhiteBackground.Timeout
	}
	if cfg.ProductImage.Publisher.Provider == "" {
		cfg.ProductImage.Publisher.Provider = defaultCfg.ProductImage.Publisher.Provider
	}
	if cfg.ProductImage.Publisher.OutputDir == "" {
		cfg.ProductImage.Publisher.OutputDir = defaultCfg.ProductImage.Publisher.OutputDir
	}
	if cfg.Updater.UpdateURL == "" {
		cfg.Updater = defaultCfg.Updater
	}
}
