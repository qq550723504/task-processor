package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

func LoadFromBytes(data []byte) (*Config, error) {
	tryLoadDotEnv()
	logDeprecatedEnvUsage()

	v := newViper()
	if len(data) > 0 {
		if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("parse yaml config: %w", err)
		}
	}

	return loadWithViper(v)
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
			BaseURL:      "https://api.shuomiai.com",
			ClientID:     "go-listing",
			ClientSecret: "",
			TokenURL:     "https://api.shuomiai.com/admin-api/system/oauth2/token",
			Scopes:       []string{"user.read"},
			TenantID:     "1",
		},
		Browser: BrowserConfig{
			Enabled:        true,
			Headless:       true,
			BrowserPath:    "./chrome/chrome.exe",
			UserDataDir:    "",
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
		Debug: DebugConfig{
			SavePublishJSON:      false,
			ProductEnrichMockLLM: false,
		},
		ListingKit: ListingKitConfig{},
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
				LoginService: LoginServiceConfig{
					MaxConcurrentLogins: 3,
					ProfileRootDir:      "./tmp/shein-login/profiles",
					ArtifactDir:         "./tmp/shein-login/artifacts",
					DefaultHeadless:     true,
				},
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
