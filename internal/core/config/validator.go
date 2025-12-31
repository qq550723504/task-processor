// Package config 提供配置管理功能
package config

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// ValidationError 配置验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("配置验证失败 [%s]: %s", e.Field, e.Message)
}

// Validate 验证配置
func (c *Config) Validate() []error {
	var errors []error

	// 验证 Worker 配置
	errors = append(errors, c.validateWorkerConfig()...)

	// 验证 Management 配置
	errors = append(errors, c.validateManagementConfig()...)

	// 验证 OpenAI 配置
	errors = append(errors, c.validateOpenAIConfig()...)

	// 验证浏览器配置
	errors = append(errors, c.validateBrowserConfig()...)

	// 验证 Amazon 配置
	errors = append(errors, c.validateAmazonConfig()...)

	// 验证平台配置
	errors = append(errors, c.validatePlatformsConfig()...)

	return errors
}

// validateWorkerConfig 验证工作池配置
func (c *Config) validateWorkerConfig() []error {
	var errors []error

	if c.Worker.Concurrency <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "worker.concurrency",
			Message: "并发数必须大于 0",
		})
	}

	if c.Worker.Concurrency > 100 {
		errors = append(errors, &ValidationError{
			Field:   "worker.concurrency",
			Message: "并发数不应超过 100",
		})
	}

	if c.Worker.BufferSize <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "worker.bufferSize",
			Message: "缓冲区大小必须大于 0",
		})
	}

	if c.Worker.TaskInterval <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "worker.taskInterval",
			Message: "任务获取间隔必须大于 0",
		})
	}

	return errors
}

// validateManagementConfig 验证管理系统配置
func (c *Config) validateManagementConfig() []error {
	var errors []error

	if c.Management.BaseURL == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.baseURL",
			Message: "管理系统 URL 不能为空",
		})
	}

	if c.Management.ClientID == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.clientID",
			Message: "客户端 ID 不能为空",
		})
	}

	if c.Management.ClientSecret == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.clientSecret",
			Message: "客户端密钥不能为空",
		})
	}

	if c.Management.TenantID == "" {
		errors = append(errors, &ValidationError{
			Field:   "management.tenantID",
			Message: "租户 ID 不能为空",
		})
	}

	return errors
}

// validateOpenAIConfig 验证OpenAI配置
func (c *Config) validateOpenAIConfig() []error {
	var errors []error

	if c.OpenAI.APIKey != "" && c.OpenAI.Model == "" {
		errors = append(errors, &ValidationError{
			Field:   "openai.model",
			Message: "OpenAI 模型不能为空",
		})
	}

	return errors
}

// validateAmazonConfig 验证Amazon配置
func (c *Config) validateAmazonConfig() []error {
	var errors []error

	if !c.Amazon.Enabled {
		return errors
	}

	return errors
}

// validateBrowserRandomConfig 验证浏览器随机配置（已废弃，使用 validateNewBrowserRandomConfig）
func (c *Config) validateBrowserRandomConfig() []error {
	// 这个函数已废弃，为了向后兼容保留
	return []error{}
}

// ValidateAndLog 验证配置并记录错误
func (c *Config) ValidateAndLog(logger *logrus.Logger) bool {
	errors := c.Validate()
	if len(errors) == 0 {
		logger.Info("✅ 配置验证通过")
		return true
	}

	logger.Error("❌ 配置验证失败:")
	for _, err := range errors {
		logger.Error("  - " + err.Error())
	}

	return false
}

// ValidateOrPanic 验证配置，如果失败则 panic
func (c *Config) ValidateOrPanic() {
	errors := c.Validate()
	if len(errors) == 0 {
		return
	}

	var messages []string
	for _, err := range errors {
		messages = append(messages, err.Error())
	}

	panic(fmt.Sprintf("配置验证失败:\n%s", strings.Join(messages, "\n")))
}

// validatePlatformsConfig 验证平台配置
func (c *Config) validatePlatformsConfig() []error {
	var errors []error

	// 验证 TEMU 平台配置
	errors = append(errors, c.validatePlatformConfig("temu", &c.Platforms.Temu)...)

	// 验证 SHEIN 平台配置
	errors = append(errors, c.validatePlatformConfig("shein", &c.Platforms.Shein)...)

	return errors
}

// validatePlatformConfig 验证单个平台配置
func (c *Config) validatePlatformConfig(platformName string, platform *PlatformConfig) []error {
	var errors []error

	// 验证自动定价配置
	if platform.AutoPricing.Enabled {
		if platform.AutoPricing.Interval <= 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.autoPricing.interval", platformName),
				Message: fmt.Sprintf("%s 自动定价间隔必须大于 0", strings.ToUpper(platformName)),
			})
		}
		if platform.AutoPricing.BatchSize <= 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.autoPricing.batchSize", platformName),
				Message: fmt.Sprintf("%s 自动定价批量大小必须大于 0", strings.ToUpper(platformName)),
			})
		}
	}

	// 验证同步配置
	if platform.Sync.Enabled {
		if platform.Sync.Interval <= 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.sync.interval", platformName),
				Message: fmt.Sprintf("%s 同步间隔必须大于 0", strings.ToUpper(platformName)),
			})
		}
		if platform.Sync.BatchSize <= 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.sync.batchSize", platformName),
				Message: fmt.Sprintf("%s 同步批量大小必须大于 0", strings.ToUpper(platformName)),
			})
		}
	}

	// 验证监控配置
	if platform.Monitor.Enabled {
		if platform.Monitor.CheckInterval <= 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.monitor.checkInterval", platformName),
				Message: fmt.Sprintf("%s 监控检查间隔必须大于 0", strings.ToUpper(platformName)),
			})
		}
		if platform.Monitor.BatchSize <= 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.monitor.batchSize", platformName),
				Message: fmt.Sprintf("%s 监控批量大小必须大于 0", strings.ToUpper(platformName)),
			})
		}
		if platform.Monitor.PriceChangeThreshold < 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.monitor.priceChangeThreshold", platformName),
				Message: fmt.Sprintf("%s 价格变化阈值不能为负数", strings.ToUpper(platformName)),
			})
		}
	}

	return errors
}

// validateBrowserConfig 验证浏览器配置
func (c *Config) validateBrowserConfig() []error {
	var errors []error

	if !c.Browser.Enabled {
		return errors
	}

	if c.Browser.PoolSize <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "browser.poolSize",
			Message: "浏览器池大小必须大于 0",
		})
	}

	if c.Browser.ViewportWidth <= 0 || c.Browser.ViewportHeight <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "browser.viewport",
			Message: "视口尺寸必须大于 0",
		})
	}

	// 验证随机配置
	errors = append(errors, c.validateNewBrowserRandomConfig()...)

	return errors
}

// validateNewBrowserRandomConfig 验证新的浏览器随机配置
func (c *Config) validateNewBrowserRandomConfig() []error {
	var errors []error

	randomConfig := c.Browser.RandomConfig

	if !randomConfig.Enabled {
		return errors
	}

	// 验证策略
	validStrategies := []string{"random", "stable", "preset", "windows"}
	isValidStrategy := false
	for _, strategy := range validStrategies {
		if randomConfig.Strategy == strategy {
			isValidStrategy = true
			break
		}
	}
	if !isValidStrategy {
		errors = append(errors, &ValidationError{
			Field:   "browser.randomConfig.strategy",
			Message: fmt.Sprintf("无效的配置策略: %s，支持的策略: %v", randomConfig.Strategy, validStrategies),
		})
	}

	// 验证指纹策略
	validFingerprintStrategies := []string{"random", "stable"}
	isValidFingerprintStrategy := false
	for _, strategy := range validFingerprintStrategies {
		if randomConfig.FingerprintStrategy == strategy {
			isValidFingerprintStrategy = true
			break
		}
	}
	if !isValidFingerprintStrategy {
		errors = append(errors, &ValidationError{
			Field:   "browser.randomConfig.fingerprintStrategy",
			Message: fmt.Sprintf("无效的指纹策略: %s，支持的策略: %v", randomConfig.FingerprintStrategy, validFingerprintStrategies),
		})
	}

	// 验证预设名称（当策略为preset时）
	if randomConfig.Strategy == "preset" {
		validPresets := []string{"windows_high_end", "windows_mid_range", "mac_high_end"}
		isValidPreset := false
		for _, preset := range validPresets {
			if randomConfig.PresetName == preset {
				isValidPreset = true
				break
			}
		}
		if !isValidPreset {
			errors = append(errors, &ValidationError{
				Field:   "browser.randomConfig.presetName",
				Message: fmt.Sprintf("无效的预设名称: %s，支持的预设: %v", randomConfig.PresetName, validPresets),
			})
		}
	}

	// 验证最大重试次数
	if randomConfig.MaxRetries < 0 {
		errors = append(errors, &ValidationError{
			Field:   "browser.randomConfig.maxRetries",
			Message: "最大重试次数不能为负数",
		})
	}

	if randomConfig.MaxRetries > 10 {
		errors = append(errors, &ValidationError{
			Field:   "browser.randomConfig.maxRetries",
			Message: "最大重试次数不应超过 10",
		})
	}

	return errors
}
