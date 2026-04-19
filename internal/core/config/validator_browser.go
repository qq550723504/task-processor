// Package validators 提供配置验证功能
package config

import (
	"fmt"
	"slices"
)

// ValidateBrowserConfig 验证浏览器配置
func ValidateBrowserConfig(browser *BrowserConfig) []error {
	var errors []error

	if !browser.Enabled {
		return errors
	}

	if browser.PoolSize <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "browser.poolSize",
			Message: "浏览器池大小必须大于 0",
		})
	}

	if browser.ViewportWidth <= 0 || browser.ViewportHeight <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "browser.viewport",
			Message: "视口尺寸必须大于 0",
		})
	}

	// 验证随机配置
	errors = append(errors, ValidateBrowserRandomConfig(&browser.RandomConfig)...)

	return errors
}

// ValidateBrowserRandomConfig 验证浏览器随机配置
func ValidateBrowserRandomConfig(randomConfig *BrowserRandomConfig) []error {
	var errors []error

	if !randomConfig.Enabled {
		return errors
	}

	// 验证策略
	validStrategies := []string{"random", "stable", "preset", "windows"}
	if !slices.Contains(validStrategies, randomConfig.Strategy) {
		errors = append(errors, &ValidationError{
			Field:   "browser.randomConfig.strategy",
			Message: fmt.Sprintf("无效的配置策略: %s，支持的策略: %v", randomConfig.Strategy, validStrategies),
		})
	}

	// 验证指纹策略
	validFingerprintStrategies := []string{"random", "stable"}
	if !slices.Contains(validFingerprintStrategies, randomConfig.FingerprintStrategy) {
		errors = append(errors, &ValidationError{
			Field:   "browser.randomConfig.fingerprintStrategy",
			Message: fmt.Sprintf("无效的指纹策略: %s，支持的策略: %v", randomConfig.FingerprintStrategy, validFingerprintStrategies),
		})
	}

	// 验证预设名称（当策略为preset时）
	if randomConfig.Strategy == "preset" {
		validPresets := []string{"windows_high_end", "windows_mid_range", "mac_high_end"}
		if !slices.Contains(validPresets, randomConfig.PresetName) {
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

	if randomConfig.MaxUsesPerInstance < 0 {
		errors = append(errors, &ValidationError{
			Field:   "browser.randomConfig.maxUsesPerInstance",
			Message: "单实例最大复用次数不能为负数",
		})
	}

	return errors
}
