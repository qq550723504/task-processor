package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserConfigDefaults(t *testing.T) {
	v := viper.New()
	setDefaults(v)

	assert.True(t, v.GetBool("browser.enabled"))
	assert.True(t, v.GetBool("browser.headless"))
	assert.Equal(t, "./chrome/chrome.exe", v.GetString("browser.browserPath"))
	assert.Equal(t, 3, v.GetInt("browser.poolSize"))
	assert.Equal(t, 1920, v.GetInt("browser.viewportWidth"))
	assert.Equal(t, 1080, v.GetInt("browser.viewportHeight"))
	assert.Equal(t, "", v.GetString("browser.proxyServer"))
	assert.True(t, v.GetBool("browser.randomConfig.enabled"))
	assert.Equal(t, "random", v.GetString("browser.randomConfig.strategy"))
	assert.Equal(t, "windows_high_end", v.GetString("browser.randomConfig.presetName"))
	assert.Equal(t, "random", v.GetString("browser.randomConfig.fingerprintStrategy"))
	assert.True(t, v.GetBool("browser.randomConfig.healthCheckEnabled"))
	assert.Equal(t, 3, v.GetInt("browser.randomConfig.maxRetries"))
	assert.Equal(t, 25, v.GetInt("browser.randomConfig.maxUsesPerInstance"))
}

func TestAmazonConfigDefaults(t *testing.T) {
	v := viper.New()
	setDefaults(v)

	assert.False(t, v.GetBool("amazon.enabled"))
	assert.Equal(t, 7, v.GetInt("amazon.dataFreshnessDays"))
	assert.Equal(t, 1, v.GetInt("amazon.riskControl.captchaRecreateThreshold"))
	assert.Equal(t, 1, v.GetInt("amazon.riskControl.authenticationRecreateThreshold"))
	assert.Equal(t, 1, v.GetInt("amazon.riskControl.browserCrashRecreateThreshold"))
	assert.Equal(t, 3, v.GetInt("amazon.riskControl.timeoutRecreateThreshold"))
	assert.True(t, v.GetBool("amazon.regionGuard.enabled"))
	assert.Equal(t, 3, v.GetInt("amazon.regionGuard.failureThreshold"))
	assert.Equal(t, 300, v.GetInt("amazon.regionGuard.evaluationWindowSeconds"))
	assert.Equal(t, 180, v.GetInt("amazon.regionGuard.cooldownSeconds"))
	assert.True(t, v.GetBool("amazon.qualityControl.retryOnValidationFailure"))
	assert.Equal(t, 2, v.GetInt("amazon.qualityControl.validationRetryMaxAttempts"))
	assert.False(t, v.GetBool("amazon.spapi.enabled"))
	assert.Equal(t, "us-east-1", v.GetString("amazon.spapi.region"))
	assert.Equal(t, "ATVPDKIKX0DER", v.GetString("amazon.spapi.defaultMarketplace"))
	assert.Equal(t, "FBM", v.GetString("amazon.spapi.defaultFulfillmentType"))
	assert.Equal(t, "New", v.GetString("amazon.spapi.defaultCondition"))
}

func TestRabbitMQConfigDefaults(t *testing.T) {
	v := viper.New()
	setDefaults(v)

	assert.False(t, v.GetBool("rabbitmq.enabled"))
	assert.Equal(t, "amqp://guest:guest@localhost:5672/", v.GetString("rabbitmq.url"))
	assert.Equal(t, 10, v.GetInt("rabbitmq.maxReconnectTries"))
	assert.Equal(t, 5, v.GetInt("rabbitmq.consumer.prefetchCount"))
	assert.Equal(t, 3, v.GetInt("rabbitmq.consumer.maxRetries"))
	assert.Equal(t, 10, v.GetInt("rabbitmq.node.maxConcurrency"))
	assert.Equal(t, 8081, v.GetInt("rabbitmq.node.healthCheckPort"))
	assert.Equal(t, 8082, v.GetInt("rabbitmq.node.metricsPort"))
	assert.Equal(t, "info", v.GetString("rabbitmq.node.logLevel"))
}

func TestAlibaba1688ConfigDefaults(t *testing.T) {
	v := viper.New()
	setDefaults(v)

	assert.False(t, v.GetBool("platforms.alibaba1688.enabled"))
	assert.Equal(t, 120, v.GetInt("platforms.alibaba1688.timeout"))
	assert.Equal(t, 2, v.GetInt("platforms.alibaba1688.poolSize"))
}

func TestPlatformFetchModeDefaults(t *testing.T) {
	v := viper.New()
	setDefaults(v)

	assert.Equal(t, "auto", v.GetString("platforms.temu.fetchMode"))
	assert.Equal(t, "auto", v.GetString("platforms.shein.fetchMode"))
}

func TestConfigBuild(t *testing.T) {
	v := viper.New()
	v.Set("browser.enabled", true)
	v.Set("browser.poolSize", 5)
	v.Set("browser.randomConfig.enabled", true)
	v.Set("browser.randomConfig.strategy", "stable")
	v.Set("browser.randomConfig.presetName", "mac_high_end")
	v.Set("amazon.enabled", true)
	v.Set("amazon.dataFreshnessDays", 10)
	v.Set("platforms.shein.fetchMode", "local")
	v.Set("productimage.workDir", "./tmp/images")
	v.Set("productimage.segmenter.enabled", true)
	v.Set("productimage.segmenter.endpoint", "http://segmenter.local")
	v.Set("productimage.whiteBackground.timeout", 90)
	v.Set("productimage.publisher.outputDir", "./published")
	v.Set("productimage.publisher.publicBase", "https://cdn.example.com/productimage")
	v.Set("productimage.lifecycle.cleanupTemporaryFiles", true)
	v.Set("productimage.lifecycle.reuseExistingAssets", true)

	cfg := BuildConfig(v)

	assert.True(t, cfg.Browser.Enabled)
	assert.Equal(t, 5, cfg.Browser.PoolSize)
	assert.True(t, cfg.Browser.RandomConfig.Enabled)
	assert.Equal(t, "stable", cfg.Browser.RandomConfig.Strategy)
	assert.Equal(t, "mac_high_end", cfg.Browser.RandomConfig.PresetName)
	assert.True(t, cfg.Amazon.Enabled)
	assert.Equal(t, 10, cfg.Amazon.DataFreshnessDays)
	assert.Equal(t, "local", cfg.Platforms.Shein.FetchMode)
	assert.Equal(t, "./tmp/images", cfg.ProductImage.WorkDir)
	assert.True(t, cfg.ProductImage.Segmenter.Enabled)
	assert.Equal(t, "http://segmenter.local", cfg.ProductImage.Segmenter.Endpoint)
	assert.Equal(t, 90, cfg.ProductImage.WhiteBackground.Timeout)
	assert.Equal(t, "./published", cfg.ProductImage.Publisher.OutputDir)
	assert.Equal(t, "https://cdn.example.com/productimage", cfg.ProductImage.Publisher.PublicBase)
	assert.True(t, cfg.ProductImage.Lifecycle.CleanupTemporaryFiles)
	assert.True(t, cfg.ProductImage.Lifecycle.ReuseExistingAssets)
}

func TestConfigBuildIncludesProductImagePublisherS3Config(t *testing.T) {
	v := viper.New()
	v.Set("productimage.publisher.provider", "s3")
	v.Set("productimage.publisher.publicBase", "https://cdn.example.com/productimage")
	v.Set("productimage.publisher.s3.bucket", "listingkit-assets")
	v.Set("productimage.publisher.s3.region", "ap-southeast-1")
	v.Set("productimage.publisher.s3.endpoint", "https://s3.example.com")
	v.Set("productimage.publisher.s3.accessKeyID", "test-access-key")
	v.Set("productimage.publisher.s3.secretAccessKey", "test-secret-key")
	v.Set("productimage.publisher.s3.usePathStyle", true)

	cfg := BuildConfig(v)

	assert.Equal(t, "s3", cfg.ProductImage.Publisher.Provider)
	assert.Equal(t, "https://cdn.example.com/productimage", cfg.ProductImage.Publisher.PublicBase)
	assert.Equal(t, "listingkit-assets", cfg.ProductImage.Publisher.S3.Bucket)
	assert.Equal(t, "ap-southeast-1", cfg.ProductImage.Publisher.S3.Region)
	assert.Equal(t, "https://s3.example.com", cfg.ProductImage.Publisher.S3.Endpoint)
	assert.Equal(t, "test-access-key", cfg.ProductImage.Publisher.S3.AccessKeyID)
	assert.Equal(t, "test-secret-key", cfg.ProductImage.Publisher.S3.SecretAccessKey)
	assert.True(t, cfg.ProductImage.Publisher.S3.UsePathStyle)
}

func TestConfigValidation(t *testing.T) {
	validConfig := &Config{
		Worker: WorkerConfig{
			Concurrency:      5,
			BufferSize:       100,
			TaskInterval:     60,
			MaxFetchPerCycle: 5,
		},
		Management: ManagementConfig{
			BaseURL:      "http://example.com",
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			TokenURL:     "http://example.com/token",
			Scopes:       []string{"user.read"},
			TenantID:     "1",
		},
		OpenAI: OpenAIConfig{
			APIKey:  "test-key",
			Model:   "test-model",
			BaseURL: "http://example.com/v1",
			Timeout: 30,
		},
		Browser: BrowserConfig{
			Enabled:        true,
			PoolSize:       3,
			ViewportWidth:  1920,
			ViewportHeight: 1080,
			RandomConfig: BrowserRandomConfig{
				Enabled:             true,
				Strategy:            "random",
				PresetName:          "windows_high_end",
				FingerprintStrategy: "random",
				MaxRetries:          3,
				MaxUsesPerInstance:  25,
			},
		},
		Amazon: AmazonConfig{
			Enabled:           true,
			DataFreshnessDays: 7,
			CrawlTimeout:      30,
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
		},
		Platforms: PlatformsConfig{
			Temu: PlatformConfig{
				Enabled: true,
				AutoPricing: AutoPricingConfig{
					Enabled:   false,
					Interval:  300,
					BatchSize: 100,
				},
				ProductSync: ScheduledTaskConfig{
					Enabled:  true,
					Interval: 60,
				},
				Monitor: MonitorConfig{
					Enabled:              true,
					CheckInterval:        1440,
					BatchSize:            100,
					PriceChangeThreshold: 10.0,
				},
			},
			Shein: PlatformConfig{
				Enabled: true,
				AutoPricing: AutoPricingConfig{
					Enabled:   false,
					Interval:  300,
					BatchSize: 100,
				},
				ProductSync: ScheduledTaskConfig{
					Enabled:  false,
					Interval: 60,
				},
				Monitor: MonitorConfig{
					Enabled:              false,
					CheckInterval:        1440,
					BatchSize:            100,
					PriceChangeThreshold: 10.0,
				},
			},
		},
	}

	errors := validConfig.Validate()
	assert.Empty(t, errors)

	invalidConfig := &Config{
		Browser: BrowserConfig{
			Enabled:        true,
			PoolSize:       0,
			ViewportWidth:  0,
			ViewportHeight: 0,
			RandomConfig: BrowserRandomConfig{
				Enabled:             true,
				Strategy:            "invalid_strategy",
				FingerprintStrategy: "invalid_fingerprint",
				MaxRetries:          -1,
				MaxUsesPerInstance:  -1,
			},
		},
		Amazon: AmazonConfig{
			Enabled: true,
		},
	}

	errors = invalidConfig.Validate()
	assert.NotEmpty(t, errors)
	assert.True(t, len(errors) >= 4)
}

func TestBrowserConfigValidation(t *testing.T) {
	validConfig := &Config{
		Browser: BrowserConfig{
			Enabled:        true,
			PoolSize:       3,
			ViewportWidth:  1920,
			ViewportHeight: 1080,
			RandomConfig: BrowserRandomConfig{
				Enabled:             true,
				Strategy:            "random",
				PresetName:          "windows_high_end",
				FingerprintStrategy: "random",
				MaxRetries:          3,
				MaxUsesPerInstance:  25,
			},
		},
	}

	errors := ValidateBrowserConfig(&validConfig.Browser)
	assert.Empty(t, errors)

	invalidConfig := &Config{
		Browser: BrowserConfig{
			Enabled:        true,
			PoolSize:       0,
			ViewportWidth:  0,
			ViewportHeight: 0,
			RandomConfig: BrowserRandomConfig{
				Enabled:             true,
				Strategy:            "invalid_strategy",
				FingerprintStrategy: "invalid_fingerprint",
				MaxRetries:          -1,
				MaxUsesPerInstance:  -1,
			},
		},
	}

	errors = ValidateBrowserConfig(&invalidConfig.Browser)
	assert.NotEmpty(t, errors)
	assert.True(t, len(errors) >= 3)
}

func TestLoadFromBytesAppliesRabbitMQDefaultsWhenSectionMissing(t *testing.T) {
	cfg, err := LoadFromBytes([]byte(`
openai:
  apiKey: "test-key"
  model: "test-model"
  baseURL: "http://example.com/v1"
  timeout: 30
management:
  baseURL: "http://example.com"
  clientID: "test-client"
  clientSecret: "test-secret"
  tokenURL: "http://example.com/token"
  scopes: ["user.read"]
`))
	require.NoError(t, err)
	require.NotNil(t, cfg.RabbitMQ)

	assert.False(t, cfg.RabbitMQ.Enabled)
	assert.Equal(t, 5, cfg.RabbitMQ.Consumer.PrefetchCount)
	assert.Equal(t, 10, cfg.RabbitMQ.Node.MaxConcurrency)
	assert.Equal(t, 8081, cfg.RabbitMQ.Node.HealthCheckPort)
	assert.Equal(t, 8082, cfg.RabbitMQ.Node.MetricsPort)
}
