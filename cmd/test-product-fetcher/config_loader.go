// Package main 提供配置加载功能
package main

import (
	"fmt"
	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*config.Config, error) {
	logrus.Infof("加载配置文件: %s", configPath)

	cfg := config.LoadConfigFromFile(configPath)
	if cfg == nil {
		return nil, fmt.Errorf("配置加载失败")
	}

	// 验证关键配置
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	logrus.Info("配置加载和验证成功")
	return cfg, nil
}

// validateConfig 验证配置
func validateConfig(cfg *config.Config) error {
	// 验证管理API配置
	if cfg.Management.BaseURL == "" {
		return fmt.Errorf("管理API BaseURL不能为空")
	}

	if cfg.Management.ClientID == "" {
		return fmt.Errorf("管理API ClientID不能为空")
	}

	if cfg.Management.ClientSecret == "" {
		return fmt.Errorf("管理API ClientSecret不能为空")
	}

	// 验证Amazon配置
	if !cfg.Amazon.Enabled {
		logrus.Warn("Amazon爬虫未启用，某些测试可能失败")
	}

	if cfg.Browser.BrowserPath == "" {
		logrus.Warn("浏览器路径未配置，将使用系统默认浏览器")
	}

	// 验证租户和店铺配置
	if len(cfg.Management.StoreIDs) == 0 {
		return fmt.Errorf("店铺ID列表不能为空")
	}

	logrus.WithFields(logrus.Fields{
		"management_url": cfg.Management.BaseURL,
		"amazon_enabled": cfg.Amazon.Enabled,
		"store_ids":      cfg.Management.StoreIDs,
	}).Info("配置验证通过")

	return nil
}
