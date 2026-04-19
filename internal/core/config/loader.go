package config

import (
	"fmt"
	"os"
	"strings"
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

func LoadAmazonCrawlerAPIConfigWithFallback(configPath string, logger *logrus.Logger) (*Config, error) {
	if logger != nil {
		logger.Infof("received amazon crawler api config path: %q", configPath)
	}

	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file does not exist: %s", configPath)
		}
		return nil, fmt.Errorf("stat config file %s: %w", configPath, err)
	}

	cfg, err := LoadConfigFromFileWithoutValidation(configPath)
	if err != nil {
		return nil, err
	}

	if err := validateAmazonCrawlerAPIConfig(cfg); err != nil {
		return nil, err
	}

	if logger != nil {
		logger.Infof("amazon crawler api config loaded successfully")
		logger.Debugf("   - Browser.Enabled: %v", cfg.Browser.Enabled)
		logger.Debugf("   - Amazon.Enabled: %v", cfg.Amazon.Enabled)
		logger.Debugf("   - Redis.Enabled: %v", cfg.Redis != nil)
	}

	return cfg, nil
}

func validateAmazonCrawlerAPIConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config validation failed:\n[General]\n  - config: configuration cannot be nil")
	}

	var errors []error
	errors = append(errors, ValidateBrowserConfig(&cfg.Browser)...)
	errors = append(errors, ValidateAmazonConfig(&cfg.Amazon)...)

	if len(errors) == 0 {
		return nil
	}

	return fmt.Errorf("config validation failed:\n%s", strings.Join(formatValidationErrors(errors), "\n"))
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
				MaxUsesPerInstance:  25,
			},
		},
		Amazon: AmazonConfig{
			Enabled:           false,
			DataFreshnessDays: 7,
			ProductDedupe: ProductDedupeConfig{
				LockTTLSeconds:     300,
				ResultTTLSeconds:   600,
				WaitTimeoutSeconds: 120,
				PollIntervalMillis: 500,
			},
			FailureArtifacts: FailureArtifactsConfig{
				Enabled:      false,
				Directory:    "./tmp/amazon-failure-artifacts",
				CaptureHTML:  true,
				MaxHTMLBytes: 262144,
			},
			RiskControl: AmazonRiskControlConfig{
				CaptchaRecreateThreshold:        1,
				AuthenticationRecreateThreshold: 1,
				BrowserCrashRecreateThreshold:   1,
				TimeoutRecreateThreshold:        3,
				NetworkRecreateThreshold:        2,
				ServerErrorRecreateThreshold:    3,
			},
			RegionGuard: AmazonRegionGuardConfig{
				Enabled:                 true,
				FailureThreshold:        3,
				EvaluationWindowSeconds: 300,
				CooldownSeconds:         180,
			},
			QualityControl: AmazonQualityControlConfig{
				RetryOnValidationFailure:   true,
				ValidationRetryMaxAttempts: 2,
			},
			ProxyPool: AmazonProxyPoolConfig{
				Enabled:                false,
				Strategy:               "round_robin",
				FailureCooldownSeconds: 300,
			},
			ConcurrencyControl: AmazonConcurrencyControlConfig{
				Enabled:               false,
				MaxInFlight:           12,
				MaxWaiting:            50,
				AcquireTimeoutSeconds: 20,
			},
			RemoteAPI: RemoteAPIConfig{
				Enabled: false,
				BaseURL: "",
				Timeout: 300,
			},
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
			Scene: ProductImageModelConfig{
				Enabled: false,
				Timeout: 60,
			},
			Publisher: ProductImagePublisherConfig{
				Enabled:    true,
				Provider:   "local",
				OutputDir:  "./tmp/productimage-published",
				PublicBase: "",
				S3:         ProductImagePublisherS3Config{},
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
				FetchMode:        "auto",
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
				FetchMode:        "auto",
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
			Alibaba1688: Alibaba1688Config{
				Enabled:  false,
				Timeout:  120,
				PoolSize: 2,
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
				Role:            NodeRoleHybrid,
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
	} else {
		if cfg.Amazon.RemoteAPI.Timeout == 0 {
			cfg.Amazon.RemoteAPI.Timeout = defaultCfg.Amazon.RemoteAPI.Timeout
		}
		if cfg.Amazon.ProductDedupe.LockTTLSeconds == 0 {
			cfg.Amazon.ProductDedupe.LockTTLSeconds = defaultCfg.Amazon.ProductDedupe.LockTTLSeconds
		}
		if cfg.Amazon.ProductDedupe.ResultTTLSeconds == 0 {
			cfg.Amazon.ProductDedupe.ResultTTLSeconds = defaultCfg.Amazon.ProductDedupe.ResultTTLSeconds
		}
		if cfg.Amazon.ProductDedupe.WaitTimeoutSeconds == 0 {
			cfg.Amazon.ProductDedupe.WaitTimeoutSeconds = defaultCfg.Amazon.ProductDedupe.WaitTimeoutSeconds
		}
		if cfg.Amazon.ProductDedupe.PollIntervalMillis == 0 {
			cfg.Amazon.ProductDedupe.PollIntervalMillis = defaultCfg.Amazon.ProductDedupe.PollIntervalMillis
		}
		if cfg.Amazon.FailureArtifacts.Directory == "" {
			cfg.Amazon.FailureArtifacts.Directory = defaultCfg.Amazon.FailureArtifacts.Directory
		}
		if cfg.Amazon.FailureArtifacts.MaxHTMLBytes == 0 {
			cfg.Amazon.FailureArtifacts.MaxHTMLBytes = defaultCfg.Amazon.FailureArtifacts.MaxHTMLBytes
		}
		if cfg.Amazon.RiskControl.CaptchaRecreateThreshold == 0 {
			cfg.Amazon.RiskControl.CaptchaRecreateThreshold = defaultCfg.Amazon.RiskControl.CaptchaRecreateThreshold
		}
		if cfg.Amazon.RiskControl.AuthenticationRecreateThreshold == 0 {
			cfg.Amazon.RiskControl.AuthenticationRecreateThreshold = defaultCfg.Amazon.RiskControl.AuthenticationRecreateThreshold
		}
		if cfg.Amazon.RiskControl.BrowserCrashRecreateThreshold == 0 {
			cfg.Amazon.RiskControl.BrowserCrashRecreateThreshold = defaultCfg.Amazon.RiskControl.BrowserCrashRecreateThreshold
		}
		if cfg.Amazon.RiskControl.TimeoutRecreateThreshold == 0 {
			cfg.Amazon.RiskControl.TimeoutRecreateThreshold = defaultCfg.Amazon.RiskControl.TimeoutRecreateThreshold
		}
		if cfg.Amazon.RiskControl.NetworkRecreateThreshold == 0 {
			cfg.Amazon.RiskControl.NetworkRecreateThreshold = defaultCfg.Amazon.RiskControl.NetworkRecreateThreshold
		}
		if cfg.Amazon.RiskControl.ServerErrorRecreateThreshold == 0 {
			cfg.Amazon.RiskControl.ServerErrorRecreateThreshold = defaultCfg.Amazon.RiskControl.ServerErrorRecreateThreshold
		}
		if cfg.Amazon.RegionGuard.FailureThreshold == 0 {
			cfg.Amazon.RegionGuard.FailureThreshold = defaultCfg.Amazon.RegionGuard.FailureThreshold
		}
		if cfg.Amazon.RegionGuard.EvaluationWindowSeconds == 0 {
			cfg.Amazon.RegionGuard.EvaluationWindowSeconds = defaultCfg.Amazon.RegionGuard.EvaluationWindowSeconds
		}
		if cfg.Amazon.RegionGuard.CooldownSeconds == 0 {
			cfg.Amazon.RegionGuard.CooldownSeconds = defaultCfg.Amazon.RegionGuard.CooldownSeconds
		}
		if cfg.Amazon.QualityControl.ValidationRetryMaxAttempts == 0 {
			cfg.Amazon.QualityControl.ValidationRetryMaxAttempts = defaultCfg.Amazon.QualityControl.ValidationRetryMaxAttempts
		}
		if cfg.Amazon.ProxyPool.Strategy == "" {
			cfg.Amazon.ProxyPool.Strategy = defaultCfg.Amazon.ProxyPool.Strategy
		}
		if cfg.Amazon.ProxyPool.FailureCooldownSeconds == 0 {
			cfg.Amazon.ProxyPool.FailureCooldownSeconds = defaultCfg.Amazon.ProxyPool.FailureCooldownSeconds
		}
		if cfg.Amazon.ConcurrencyControl.MaxInFlight == 0 {
			cfg.Amazon.ConcurrencyControl.MaxInFlight = defaultCfg.Amazon.ConcurrencyControl.MaxInFlight
		}
		if cfg.Amazon.ConcurrencyControl.MaxWaiting == 0 {
			cfg.Amazon.ConcurrencyControl.MaxWaiting = defaultCfg.Amazon.ConcurrencyControl.MaxWaiting
		}
		if cfg.Amazon.ConcurrencyControl.AcquireTimeoutSeconds == 0 {
			cfg.Amazon.ConcurrencyControl.AcquireTimeoutSeconds = defaultCfg.Amazon.ConcurrencyControl.AcquireTimeoutSeconds
		}
	}
	if cfg.Browser.RandomConfig.MaxUsesPerInstance == 0 {
		cfg.Browser.RandomConfig.MaxUsesPerInstance = defaultCfg.Browser.RandomConfig.MaxUsesPerInstance
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
	if cfg.ProductImage.Scene.Timeout == 0 {
		cfg.ProductImage.Scene.Timeout = defaultCfg.ProductImage.Scene.Timeout
	}
	if cfg.ProductImage.Publisher.Provider == "" {
		cfg.ProductImage.Publisher.Provider = defaultCfg.ProductImage.Publisher.Provider
	}
	if cfg.ProductImage.Publisher.OutputDir == "" {
		cfg.ProductImage.Publisher.OutputDir = defaultCfg.ProductImage.Publisher.OutputDir
	}
	if cfg.ProductImage.Publisher.S3.Region == "" {
		cfg.ProductImage.Publisher.S3.Region = defaultCfg.ProductImage.Publisher.S3.Region
	}
	if cfg.ProductImage.Publisher.S3.Endpoint == "" {
		cfg.ProductImage.Publisher.S3.Endpoint = defaultCfg.ProductImage.Publisher.S3.Endpoint
	}
	if cfg.Updater.UpdateURL == "" {
		cfg.Updater = defaultCfg.Updater
	}
	if cfg.RabbitMQ == nil {
		rabbitMQCopy := *defaultCfg.RabbitMQ
		cfg.RabbitMQ = &rabbitMQCopy
	} else {
		if cfg.RabbitMQ.URL == "" {
			cfg.RabbitMQ.URL = defaultCfg.RabbitMQ.URL
		}
		if cfg.RabbitMQ.ReconnectInterval == 0 {
			cfg.RabbitMQ.ReconnectInterval = defaultCfg.RabbitMQ.ReconnectInterval
		}
		if cfg.RabbitMQ.MaxReconnectTries == 0 {
			cfg.RabbitMQ.MaxReconnectTries = defaultCfg.RabbitMQ.MaxReconnectTries
		}
		if cfg.RabbitMQ.Consumer.PrefetchCount == 0 {
			cfg.RabbitMQ.Consumer.PrefetchCount = defaultCfg.RabbitMQ.Consumer.PrefetchCount
		}
		if cfg.RabbitMQ.Consumer.RetryDelay == 0 {
			cfg.RabbitMQ.Consumer.RetryDelay = defaultCfg.RabbitMQ.Consumer.RetryDelay
		}
		if cfg.RabbitMQ.Consumer.MaxRetries == 0 {
			cfg.RabbitMQ.Consumer.MaxRetries = defaultCfg.RabbitMQ.Consumer.MaxRetries
		}
		if cfg.RabbitMQ.Node.Role == "" {
			cfg.RabbitMQ.Node.Role = defaultCfg.RabbitMQ.Node.Role
		}
		if cfg.RabbitMQ.Node.MaxConcurrency == 0 {
			cfg.RabbitMQ.Node.MaxConcurrency = defaultCfg.RabbitMQ.Node.MaxConcurrency
		}
		if cfg.RabbitMQ.Node.HealthCheckPort == 0 {
			cfg.RabbitMQ.Node.HealthCheckPort = defaultCfg.RabbitMQ.Node.HealthCheckPort
		}
		if cfg.RabbitMQ.Node.MetricsPort == 0 {
			cfg.RabbitMQ.Node.MetricsPort = defaultCfg.RabbitMQ.Node.MetricsPort
		}
		if cfg.RabbitMQ.Node.LogLevel == "" {
			cfg.RabbitMQ.Node.LogLevel = defaultCfg.RabbitMQ.Node.LogLevel
		}
		if cfg.RabbitMQ.Node.ShutdownTimeout == 0 {
			cfg.RabbitMQ.Node.ShutdownTimeout = defaultCfg.RabbitMQ.Node.ShutdownTimeout
		}
	}
	if cfg.Platforms.Alibaba1688.Timeout == 0 {
		cfg.Platforms.Alibaba1688.Timeout = defaultCfg.Platforms.Alibaba1688.Timeout
	}
	if cfg.Platforms.Alibaba1688.PoolSize == 0 {
		cfg.Platforms.Alibaba1688.PoolSize = defaultCfg.Platforms.Alibaba1688.PoolSize
	}
}
