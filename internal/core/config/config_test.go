package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
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
}

func TestAmazonConfigDefaults(t *testing.T) {
	v := viper.New()
	setDefaults(v)

	assert.False(t, v.GetBool("amazon.enabled"))
	assert.Equal(t, 7, v.GetInt("amazon.dataFreshnessDays"))
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

func TestConfigBuild(t *testing.T) {
	v := viper.New()
	v.Set("browser.enabled", true)
	v.Set("browser.poolSize", 5)
	v.Set("browser.randomConfig.enabled", true)
	v.Set("browser.randomConfig.strategy", "stable")
	v.Set("browser.randomConfig.presetName", "mac_high_end")
	v.Set("amazon.enabled", true)
	v.Set("amazon.dataFreshnessDays", 10)
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
	assert.Equal(t, "./tmp/images", cfg.ProductImage.WorkDir)
	assert.True(t, cfg.ProductImage.Segmenter.Enabled)
	assert.Equal(t, "http://segmenter.local", cfg.ProductImage.Segmenter.Endpoint)
	assert.Equal(t, 90, cfg.ProductImage.WhiteBackground.Timeout)
	assert.Equal(t, "./published", cfg.ProductImage.Publisher.OutputDir)
	assert.Equal(t, "https://cdn.example.com/productimage", cfg.ProductImage.Publisher.PublicBase)
	assert.True(t, cfg.ProductImage.Lifecycle.CleanupTemporaryFiles)
	assert.True(t, cfg.ProductImage.Lifecycle.ReuseExistingAssets)
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
			},
		},
		Amazon: AmazonConfig{
			Enabled:           true,
			DataFreshnessDays: 7,
			CrawlTimeout:      30,
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
			},
		},
	}

	errors = ValidateBrowserConfig(&invalidConfig.Browser)
	assert.NotEmpty(t, errors)
	assert.True(t, len(errors) >= 3)
}
