// Package service 提供核价服务实现
package service

import (
	"fmt"

	"task-processor/internal/app/scheduler"
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

	// 创建统一调度器管理器
	s.schedulerManager = scheduler.NewManager(s.ctx)

	// 获取所有平台配置
	platformConfigs := s.getPlatformConfigs(cfg)

	// 启动所有平台的任务
	for _, platformConfig := range platformConfigs {
		if err := s.startPlatformTasks(platformConfig, cfg); err != nil {
			return fmt.Errorf("启动%s核价任务失败: %w", platformConfig.PlatformName, err)
		}
	}

	s.logger.Info("✅ 核价处理器启动完成")
	return nil
}
