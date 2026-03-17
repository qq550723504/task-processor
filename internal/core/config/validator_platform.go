// Package validators 提供配置验证功能
package config

import (
	"fmt"
	"strings"
)

// ValidatePlatformsConfig 验证平台配置
func ValidatePlatformsConfig(platforms *PlatformsConfig) []error {
	var errors []error

	// 验证 TEMU 平台配置
	errors = append(errors, ValidatePlatformConfig("temu", &platforms.Temu)...)

	// 验证 SHEIN 平台配置
	errors = append(errors, ValidatePlatformConfig("shein", &platforms.Shein)...)

	return errors
}

// ValidatePlatformConfig 验证单个平台配置
func ValidatePlatformConfig(platformName string, platform *PlatformConfig) []error {
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
	if platform.SyncProduct.Enabled {
		if platform.SyncProduct.Interval <= 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.sync.interval", platformName),
				Message: fmt.Sprintf("%s 同步间隔必须大于 0", strings.ToUpper(platformName)),
			})
		}
		if platform.SyncProduct.BatchSize <= 0 {
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
