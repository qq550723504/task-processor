package main

import (
	"fmt"
	"task-processor/common/config"

	"github.com/sirupsen/logrus"
)

// checkConfig 检查配置是否完整
func checkConfig(cfg *config.Config, logger *logrus.Logger) error {
	logger.Info("🔍 检查配置...")

	// 检查 Amazon 配置是否存在
	if cfg.Amazon.SPAPI.ClientID == "" {
		return fmt.Errorf("amazon.spapi.clientID 未配置")
	}

	if cfg.Amazon.SPAPI.ClientSecret == "" {
		return fmt.Errorf("amazon.spapi.clientSecret 未配置")
	}

	if cfg.Amazon.SPAPI.RefreshToken == "" {
		return fmt.Errorf("amazon.spapi.refreshToken 未配置")
	}

	if cfg.Amazon.SPAPI.Region == "" {
		return fmt.Errorf("amazon.spapi.region 未配置")
	}

	if cfg.Amazon.SPAPI.MarketplaceID == "" {
		return fmt.Errorf("amazon.spapi.marketplaceID 未配置")
	}

	logger.Info("✅ 配置检查通过")
	return nil
}

// printConfig 打印配置信息（隐藏敏感信息）
func printConfig(cfg *config.Config, logger *logrus.Logger) {
	logger.Info("📋 当前配置:")
	logger.Infof("   启用状态: %v", cfg.Amazon.SPAPI.Enabled)
	logger.Infof("   区域: %s", cfg.Amazon.SPAPI.Region)
	logger.Infof("   市场ID: %s", cfg.Amazon.SPAPI.MarketplaceID)
	logger.Infof("   Client ID: %s", maskString(cfg.Amazon.SPAPI.ClientID))
	logger.Infof("   Client Secret: %s", maskString(cfg.Amazon.SPAPI.ClientSecret))
	logger.Infof("   Refresh Token: %s", maskString(cfg.Amazon.SPAPI.RefreshToken))
	logger.Infof("   配送方式: %s", cfg.Amazon.SPAPI.DefaultFulfillmentType)
	logger.Infof("   产品状态: %s", cfg.Amazon.SPAPI.DefaultCondition)
}

// maskString 隐藏字符串中间部分
func maskString(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
