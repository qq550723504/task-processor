// Package service 提供平台配置定义
package service

import (
	"time"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
)

// platformTaskConfig 平台任务配置
type platformTaskConfig struct {
	PlatformName    string                       // 平台名称
	Enabled         bool                         // 是否启用
	AutoPricing     config.AutoPricingConfig     // 自动核价配置
	FactoryCreator  func() scheduler.TaskFactory // 工厂创建函数
	EnhancedMessage string                       // 增强版消息（可选）
}

// getPlatformConfigs 获取所有平台配置
func (s *pricingServiceImpl) getPlatformConfigs(cfg *config.Config) []platformTaskConfig {
	configs := make([]platformTaskConfig, 0, 2)

	// TEMU 平台配置
	temuConfig := platformTaskConfig{
		PlatformName: "TEMU",
		Enabled:      cfg.Platforms.Temu.AutoPricing.Enabled,
		AutoPricing:  cfg.Platforms.Temu.AutoPricing,
		FactoryCreator: func() scheduler.TaskFactory {
			return s.createTemuFactory(cfg)
		},
	}
	configs = append(configs, temuConfig)

	// SHEIN 平台配置
	sheinConfig := platformTaskConfig{
		PlatformName: "SHEIN",
		Enabled:      cfg.Platforms.Shein.Enabled && cfg.Platforms.Shein.AutoPricing.Enabled,
		AutoPricing:  cfg.Platforms.Shein.AutoPricing,
		FactoryCreator: func() scheduler.TaskFactory {
			return s.createSheinFactory()
		},
	}
	configs = append(configs, sheinConfig)

	return configs
}

// getDefaultInterval 获取默认间隔
func getDefaultInterval(interval int) time.Duration {
	if interval <= 0 {
		return 30 * time.Minute //默认12小时
	}
	return time.Duration(interval) * time.Second
}
