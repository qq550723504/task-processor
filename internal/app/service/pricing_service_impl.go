// Package service 提供核价服务实现
package service

import (
	"fmt"
	"time"

	"task-processor/internal/core/config"
	sheinHandlers "task-processor/internal/platforms/shein/handlers"
	temuHandlers "task-processor/internal/platforms/temu/handlers"
)

// initializePricingResources 初始化核价资源
func (s *pricingServiceImpl) initializePricingResources() error {
	s.logger.Info("初始化核价资源...")

	// 获取共享的管理客户端
	s.managementClient = GetSharedManagementClientInstance()
	if s.managementClient == nil {
		return fmt.Errorf("无法获取共享管理客户端")
	}

	s.logger.Info("✅ 核价资源初始化完成")
	return nil
}

// startPricingHandlers 启动各平台核价处理器
func (s *pricingServiceImpl) startPricingHandlers() error {
	s.logger.Info("启动各平台核价处理器...")

	// 获取配置
	cfg := GetGlobalConfig()
	if cfg == nil {
		return fmt.Errorf("无法获取全局配置")
	}

	// 启动TEMU核价处理器
	if err := s.startTemuPricingHandler(cfg); err != nil {
		return fmt.Errorf("启动TEMU核价处理器失败: %w", err)
	}

	// 启动SHEIN核价处理器
	if err := s.startSheinPricingHandler(cfg); err != nil {
		return fmt.Errorf("启动SHEIN核价处理器失败: %w", err)
	}

	s.logger.Info("✅ 核价处理器启动完成")
	return nil
}

// startTemuPricingHandler 启动TEMU核价处理器
func (s *pricingServiceImpl) startTemuPricingHandler(cfg *config.Config) error {
	if !cfg.Platforms.Temu.AutoPricing.Enabled {
		s.logger.Info("⚠️ TEMU自动核价已禁用")
		return nil
	}

	s.logger.Info("启动TEMU核价处理器...")

	// 创建TEMU核价处理器
	s.temuAutoPricingHandler = temuHandlers.NewAutoPricingHandler(s.managementClient, cfg.Management.StoreIDs)

	// 启动核价处理器
	autoPricingInterval := time.Duration(cfg.Platforms.Temu.AutoPricing.Interval) * time.Second
	if autoPricingInterval <= 0 {
		autoPricingInterval = 30 * time.Minute
	}

	s.logger.Infof("启动TEMU自动核价处理器，间隔: %v", autoPricingInterval)
	go s.temuAutoPricingHandler.Start(s.ctx, autoPricingInterval)

	return nil
}

// startSheinPricingHandler 启动SHEIN核价处理器
func (s *pricingServiceImpl) startSheinPricingHandler(cfg *config.Config) error {
	if !cfg.Platforms.Shein.AutoPricing.Enabled {
		s.logger.Info("⚠️ SHEIN自动核价已禁用")
		return nil
	}

	s.logger.Info("启动SHEIN核价处理器...")
	// 创建SHEIN核价处理器
	s.sheinAutoPricingHandler = sheinHandlers.NewAutoPricingHandler(s.managementClient, cfg.Management.StoreIDs)

	// 启动核价处理器
	autoPricingInterval := time.Duration(cfg.Platforms.Shein.AutoPricing.Interval) * time.Second
	if autoPricingInterval <= 0 {
		autoPricingInterval = 30 * time.Minute
	}

	s.logger.Infof("启动SHEIN自动核价处理器，间隔: %v", autoPricingInterval)
	go s.sheinAutoPricingHandler.Start(s.ctx, autoPricingInterval)
	return nil
}
