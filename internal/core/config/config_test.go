package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// TestBrowserConfigDefaults 测试浏览器配置默认值
func TestBrowserConfigDefaults(t *testing.T) {
	// 重置viper
	viper.Reset()

	// 设置默认值
	setDefaults()

	// 验证浏览器默认值
	assert.True(t, viper.GetBool("browser.enabled"))
	assert.True(t, viper.GetBool("browser.headless"))
	assert.Equal(t, "./chrome/chrome.exe", viper.GetString("browser.browserPath"))
	assert.Equal(t, 3, viper.GetInt("browser.poolSize"))
	assert.Equal(t, 1920, viper.GetInt("browser.viewportWidth"))
	assert.Equal(t, 1080, viper.GetInt("browser.viewportHeight"))
	assert.Equal(t, "", viper.GetString("browser.proxyServer"))

	// 验证随机配置默认值
	assert.True(t, viper.GetBool("browser.randomConfig.enabled"))
	assert.Equal(t, "random", viper.GetString("browser.randomConfig.strategy"))
	assert.Equal(t, "windows_high_end", viper.GetString("browser.randomConfig.presetName"))
	assert.Equal(t, "random", viper.GetString("browser.randomConfig.fingerprintStrategy"))
	assert.True(t, viper.GetBool("browser.randomConfig.healthCheckEnabled"))
	assert.Equal(t, 3, viper.GetInt("browser.randomConfig.maxRetries"))
}

// TestAmazonConfigDefaults 测试Amazon配置默认值
func TestAmazonConfigDefaults(t *testing.T) {
	// 重置viper
	viper.Reset()

	// 设置默认值
	setDefaults()

	// 验证Amazon默认值
	assert.True(t, viper.GetBool("amazon.enabled"))
	assert.Equal(t, 7, viper.GetInt("amazon.dataFreshnessDays"))

	// 验证SPAPI默认值
	assert.False(t, viper.GetBool("amazon.spapi.enabled"))
	assert.Equal(t, "us-east-1", viper.GetString("amazon.spapi.region"))
	assert.Equal(t, "us", viper.GetString("amazon.spapi.defaultMarketplace"))
	assert.Equal(t, "FBM", viper.GetString("amazon.spapi.defaultFulfillmentType"))
	assert.Equal(t, "New", viper.GetString("amazon.spapi.defaultCondition"))
}

// TestConfigBuild 测试配置构建
func TestConfigBuild(t *testing.T) {
	// 重置viper
	viper.Reset()

	// 设置测试配置
	viper.Set("browser.enabled", true)
	viper.Set("browser.poolSize", 5)
	viper.Set("browser.randomConfig.enabled", true)
	viper.Set("browser.randomConfig.strategy", "stable")
	viper.Set("browser.randomConfig.presetName", "mac_high_end")
	viper.Set("amazon.enabled", true)
	viper.Set("amazon.dataFreshnessDays", 10)

	// 构建配置
	cfg := buildConfig()

	// 验证浏览器配置
	assert.True(t, cfg.Browser.Enabled)
	assert.Equal(t, 5, cfg.Browser.PoolSize)
	assert.True(t, cfg.Browser.RandomConfig.Enabled)
	assert.Equal(t, "stable", cfg.Browser.RandomConfig.Strategy)
	assert.Equal(t, "mac_high_end", cfg.Browser.RandomConfig.PresetName)

	// 验证Amazon配置
	assert.True(t, cfg.Amazon.Enabled)
	assert.Equal(t, 10, cfg.Amazon.DataFreshnessDays)

	// 验证兼容性字段（从浏览器配置复制）
	assert.Equal(t, 5, cfg.Amazon.PoolSize)
	assert.True(t, cfg.Amazon.RandomConfig.Enabled)
	assert.Equal(t, "stable", cfg.Amazon.RandomConfig.Strategy)
}

// TestConfigValidation 测试配置验证
func TestConfigValidation(t *testing.T) {
	// 测试有效配置
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
		Amazon: AmazonConfig{
			Enabled: true,
		},
	}

	errors := validConfig.Validate()
	assert.Empty(t, errors, "有效配置不应该有验证错误")

	// 测试无效配置
	invalidConfig := &Config{
		Browser: BrowserConfig{
			Enabled:        true,
			PoolSize:       0, // 无效值
			ViewportWidth:  0, // 无效值
			ViewportHeight: 0, // 无效值
			RandomConfig: BrowserRandomConfig{
				Enabled:             true,
				Strategy:            "invalid_strategy",    // 无效策略
				FingerprintStrategy: "invalid_fingerprint", // 无效指纹策略
				MaxRetries:          -1,                    // 无效重试次数
			},
		},
		Amazon: AmazonConfig{
			Enabled: true,
		},
	}

	errors = invalidConfig.Validate()
	assert.NotEmpty(t, errors, "无效配置应该有验证错误")
	assert.True(t, len(errors) >= 4, "应该检测到多个验证错误")
}

// TestBrowserConfigValidation 测试浏览器配置验证
func TestBrowserConfigValidation(t *testing.T) {
	// 测试有效的浏览器配置
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

	errors := validConfig.validateBrowserConfig()
	assert.Empty(t, errors, "有效的浏览器配置不应该有验证错误")

	// 测试无效的浏览器配置
	invalidConfig := &Config{
		Browser: BrowserConfig{
			Enabled:        true,
			PoolSize:       0, // 无效值
			ViewportWidth:  0, // 无效值
			ViewportHeight: 0, // 无效值
			RandomConfig: BrowserRandomConfig{
				Enabled:             true,
				Strategy:            "invalid_strategy",    // 无效策略
				FingerprintStrategy: "invalid_fingerprint", // 无效指纹策略
				MaxRetries:          -1,                    // 无效重试次数
			},
		},
	}

	errors = invalidConfig.validateBrowserConfig()
	assert.NotEmpty(t, errors, "无效的浏览器配置应该有验证错误")
	assert.True(t, len(errors) >= 3, "应该检测到多个验证错误")
}
