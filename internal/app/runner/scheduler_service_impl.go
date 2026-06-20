// Package runner 提供处理器和调度器的运行管理功能
package runner

import (
	"fmt"
	"time"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/logger"
)

// initializeResources 初始化资源
func (s *schedulerServiceImpl) initializeResources() error {
	log := logger.GetGlobalLogger("service.scheduler")
	log.Info("初始化调度服务资源...")

	// 验证店铺运行时能力是否已注入
	if s.storeRuntime == nil {
		return fmt.Errorf("店铺运行时未注入")
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
	s.schedulerManager = scheduler.NewManager(s.ctx, time.Duration(cfg.Worker.TaskTimeout)*time.Second)

	// 获取所有平台配置
	platformConfigs := s.getPlatformConfigs(cfg)

	// 启动所有平台的任务
	for _, platformConfig := range platformConfigs {
		if err := s.startPlatformTasks(platformConfig, cfg); err != nil {
			log.WithFields(map[string]any{
				logger.FieldPlatform: platformConfig.PlatformName,
			}).WithError(err).Error("启动调度任务失败")
			// 继续启动其他平台，不中断
			continue
		}
	}

	log.Info("✅ 调度任务启动完成")
	return nil
}
