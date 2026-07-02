// Package runner 提供处理器和调度器的运行管理功能
package runner

import (
	"fmt"
	"os"
	"time"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	infralock "task-processor/internal/infra/lock"
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
	taskTimeout := time.Duration(cfg.Worker.TaskTimeout) * time.Second
	s.schedulerManager = scheduler.NewManager(s.ctx, taskTimeout)
	if err := s.configureSchedulerDistributedLock(cfg, taskTimeout); err != nil {
		return err
	}

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

func (s *schedulerServiceImpl) configureSchedulerDistributedLock(cfg *config.Config, taskTimeout time.Duration) error {
	if s == nil || s.schedulerManager == nil {
		return nil
	}
	if cfg == nil || cfg.Redis == nil || cfg.Redis.Host == "" {
		s.logger.Warn("调度任务未配置 Redis 分布式锁，当前仅能防止单进程内重复执行")
		return nil
	}
	locker, err := infralock.NewRedisLock(cfg.Redis, schedulerLockOwner(), s.logger)
	if err != nil {
		return fmt.Errorf("初始化调度任务分布式锁失败: %w", err)
	}
	lockTTL := taskTimeout + time.Minute
	if lockTTL <= time.Minute {
		lockTTL = 30 * time.Minute
	}
	s.schedulerManager.SetDistributedLock(locker, lockTTL)
	s.schedulerLockCloser = locker
	s.logger.WithField("lock_ttl", lockTTL.String()).Info("调度任务 Redis 分布式锁已启用")
	return nil
}

func schedulerLockOwner() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", hostname, os.Getpid())
}
