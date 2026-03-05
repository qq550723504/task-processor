// Package service 提供调度服务实现
package service

import (
	"fmt"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/logger"
)

// initializeResources 初始化资源
func (s *schedulerServiceImpl) initializeResources() error {
	log := logger.GetGlobalLogger("service.scheduler")
	log.Info("初始化调度服务资源...")

	// 验证管理客户端是否已注入
	if s.managementClient == nil {
		return fmt.Errorf("管理客户端未注入")
	}

	log.Info("✅ 调度服务资源初始化完成")
	return nil
}

// startScheduledTasks 启动所有调度任务
func (s *schedulerServiceImpl) startScheduledTasks() error {
	log := logger.GetGlobalLogger("service.scheduler")
	log.Info("启动所有调度任务...")

	// 使用注入的配置
	cfg := s.config
	if cfg == nil {
		return fmt.Errorf("配置未注入")
	}

	// 创建统一调度器管理器
	s.schedulerManager = scheduler.NewManager(s.ctx)

	// 获取所有平台配置
	platformConfigs := s.getPlatformConfigs(cfg)

	// 启动所有平台的任务
	for _, platformConfig := range platformConfigs {
		if err := s.startPlatformTasks(platformConfig, cfg); err != nil {
			log.WithFields(map[string]interface{}{
				logger.FieldPlatform: platformConfig.PlatformName,
			}).WithError(err).Error("启动调度任务失败")
			// 继续启动其他平台，不中断
			continue
		}
	}

	log.Info("✅ 调度任务启动完成")
	return nil
}
