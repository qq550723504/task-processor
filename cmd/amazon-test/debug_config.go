package main

import (
	"fmt"
	"task-processor/common/config"

	"github.com/sirupsen/logrus"
)

// debugConfig 调试配置加载
func debugConfig(cfg *config.Config, logger *logrus.Logger) {
	logger.Info("🔍 配置调试信息:")
	logger.Info("----------------------------------------")

	// Amazon 基础配置
	logger.Infof("Amazon.Enabled: %v", cfg.Amazon.Enabled)
	logger.Infof("Amazon.Headless: %v", cfg.Amazon.Headless)

	// SPAPI 配置
	logger.Info("")
	logger.Info("SPAPI 配置:")
	logger.Infof("  Enabled: %v", cfg.Amazon.SPAPI.Enabled)
	logger.Infof("  Sandbox: %v", cfg.Amazon.SPAPI.Sandbox)
	logger.Infof("  Region: %s", cfg.Amazon.SPAPI.Region)
	logger.Infof("  MarketplaceID: %s", cfg.Amazon.SPAPI.MarketplaceID)

	// 凭证信息（部分隐藏）
	logger.Info("")
	logger.Info("凭证信息:")
	logger.Infof("  ClientID: %s", formatCredential(cfg.Amazon.SPAPI.ClientID))
	logger.Infof("  ClientSecret: %s", formatCredential(cfg.Amazon.SPAPI.ClientSecret))
	logger.Infof("  RefreshToken: %s", formatCredential(cfg.Amazon.SPAPI.RefreshToken))

	// 其他配置
	logger.Info("")
	logger.Infof("  DefaultFulfillmentType: %s", cfg.Amazon.SPAPI.DefaultFulfillmentType)
	logger.Infof("  DefaultCondition: %s", cfg.Amazon.SPAPI.DefaultCondition)

	logger.Info("----------------------------------------")

	// 检查凭证格式
	logger.Info("")
	logger.Info("🔍 凭证格式检查:")
	checkCredentialFormat(cfg, logger)
}

// formatCredential 格式化凭证显示
func formatCredential(credential string) string {
	if credential == "" {
		return "[未配置]"
	}

	length := len(credential)
	if length <= 10 {
		return "****"
	}

	// 显示前缀和长度
	prefix := credential[:10]
	return fmt.Sprintf("%s... (长度: %d)", prefix, length)
}

// checkCredentialFormat 检查凭证格式
func checkCredentialFormat(cfg *config.Config, logger *logrus.Logger) {
	// 检查 ClientID 格式
	if cfg.Amazon.SPAPI.ClientID == "" {
		logger.Error("  ❌ ClientID 未配置")
	} else if len(cfg.Amazon.SPAPI.ClientID) < 20 {
		logger.Warn("  ⚠️  ClientID 长度过短，可能不正确")
	} else {
		logger.Info("  ✅ ClientID 格式正常")
	}

	// 检查 ClientSecret 格式
	if cfg.Amazon.SPAPI.ClientSecret == "" {
		logger.Error("  ❌ ClientSecret 未配置")
	} else if len(cfg.Amazon.SPAPI.ClientSecret) < 20 {
		logger.Warn("  ⚠️  ClientSecret 长度过短，可能不正确")
	} else {
		// 检查是否误用了 ClientID 作为 ClientSecret
		if len(cfg.Amazon.SPAPI.ClientSecret) > 30 &&
			cfg.Amazon.SPAPI.ClientSecret[:5] == "amzn1" {
			logger.Warn("  ⚠️  ClientSecret 格式异常，看起来像 ClientID")
			logger.Warn("      ClientSecret 应该是一个随机字符串，不是 amzn1 开头")
		} else {
			logger.Info("  ✅ ClientSecret 格式正常")
		}
	}

	// 检查 RefreshToken 格式
	if cfg.Amazon.SPAPI.RefreshToken == "" {
		logger.Error("  ❌ RefreshToken 未配置")
	} else if len(cfg.Amazon.SPAPI.RefreshToken) < 50 {
		logger.Warn("  ⚠️  RefreshToken 长度过短，可能不正确")
	} else if cfg.Amazon.SPAPI.RefreshToken[:5] != "Atzr|" {
		logger.Warn("  ⚠️  RefreshToken 格式异常，应该以 'Atzr|' 开头")
	} else {
		logger.Info("  ✅ RefreshToken 格式正常")
	}
}
