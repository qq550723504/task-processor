// Package config 提供配置管理功能
package config

// GetPlatformConfig 获取指定平台的完整配置
func (c *Config) GetPlatformConfig(platform string) *PlatformConfig {
	switch platform {
	case "temu":
		return &c.Platforms.Temu
	case "shein":
		return &c.Platforms.Shein
	default:
		return nil
	}
}

// GetPlatformSyncConfig 获取指定平台的同步配置
func (c *Config) GetPlatformSyncConfig(platform string) *SyncConfig {
	platformConfig := c.GetPlatformConfig(platform)
	if platformConfig == nil {
		return nil
	}
	return &platformConfig.Sync
}

// GetPlatformMonitorConfig 获取指定平台的监控配置
func (c *Config) GetPlatformMonitorConfig(platform string) *MonitorConfig {
	platformConfig := c.GetPlatformConfig(platform)
	if platformConfig == nil {
		return nil
	}
	return &platformConfig.Monitor
}

// GetPlatformAutoPricingConfig 获取指定平台的自动定价配置
func (c *Config) GetPlatformAutoPricingConfig(platform string) *AutoPricingConfig {
	platformConfig := c.GetPlatformConfig(platform)
	if platformConfig == nil {
		return nil
	}
	return &platformConfig.AutoPricing
}

// IsSyncEnabled 检查指定平台是否启用同步
func (c *Config) IsSyncEnabled(platform string) bool {
	config := c.GetPlatformSyncConfig(platform)
	return config != nil && config.Enabled
}

// IsMonitorEnabled 检查指定平台是否启用监控
func (c *Config) IsMonitorEnabled(platform string) bool {
	config := c.GetPlatformMonitorConfig(platform)
	return config != nil && config.Enabled
}

// IsAutoPricingEnabled 检查指定平台是否启用自动定价
func (c *Config) IsAutoPricingEnabled(platform string) bool {
	config := c.GetPlatformAutoPricingConfig(platform)
	return config != nil && config.Enabled
}

// GetEnabledPlatforms 获取启用了指定功能的平台列表
func (c *Config) GetEnabledPlatforms(feature string) []string {
	var platforms []string

	switch feature {
	case "sync":
		if c.IsSyncEnabled("temu") {
			platforms = append(platforms, "temu")
		}
		if c.IsSyncEnabled("shein") {
			platforms = append(platforms, "shein")
		}
	case "monitor":
		if c.IsMonitorEnabled("temu") {
			platforms = append(platforms, "temu")
		}
		if c.IsMonitorEnabled("shein") {
			platforms = append(platforms, "shein")
		}
	case "autoPricing":
		if c.IsAutoPricingEnabled("temu") {
			platforms = append(platforms, "temu")
		}
		if c.IsAutoPricingEnabled("shein") {
			platforms = append(platforms, "shein")
		}
	}

	return platforms
}
