// Package utils 提供工具方法
package utils

import (
	commonConfig "task-processor/common/config"
	internalConfig "task-processor/internal/config"
)

// ConvertAmazonConfig 将内部配置转换为common配置
func ConvertAmazonConfig(cfg *internalConfig.AmazonConfig) *commonConfig.AmazonConfig {
	return &commonConfig.AmazonConfig{
		Enabled:           cfg.Enabled,
		Headless:          cfg.Headless,
		BrowserPath:       cfg.BrowserPath,
		PoolSize:          cfg.PoolSize,
		Zipcodes:          cfg.Zipcodes,
		ViewportWidth:     cfg.ViewportWidth,
		ViewportHeight:    cfg.ViewportHeight,
		ProxyServer:       cfg.ProxyServer,
		DataFreshnessDays: cfg.DataFreshnessDays,
		SPAPI: commonConfig.SPAPIConfig{
			Enabled:                cfg.SPAPI.Enabled,
			Sandbox:                cfg.SPAPI.Sandbox,
			Region:                 cfg.SPAPI.Region,
			MarketplaceID:          cfg.SPAPI.MarketplaceID,
			ClientID:               cfg.SPAPI.ClientID,
			ClientSecret:           cfg.SPAPI.ClientSecret,
			RefreshToken:           cfg.SPAPI.RefreshToken,
			DefaultFulfillmentType: cfg.SPAPI.DefaultFulfillmentType,
			DefaultCondition:       cfg.SPAPI.DefaultCondition,
		},
	}
}
