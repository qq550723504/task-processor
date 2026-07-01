package runner

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/app/scheduler"
)

const dynamicScheduledTaskConfigSyncInterval = time.Minute

func (s *schedulerServiceImpl) runScheduledTaskConfigSyncLoop(ctx context.Context) {
	if s == nil {
		return
	}
	ticker := time.NewTicker(dynamicScheduledTaskConfigSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			created, err := s.syncEnabledScheduledTaskConfigs(ctx)
			if err != nil {
				s.logger.Warnf("同步后台定时任务配置失败: %v", err)
				continue
			}
			if created > 0 {
				s.logger.Infof("同步后台定时任务配置完成，新增启动 %d 个任务", created)
			}
		}
	}
}

func (s *schedulerServiceImpl) syncEnabledScheduledTaskConfigs(ctx context.Context) (int, error) {
	if s == nil {
		return 0, fmt.Errorf("scheduler service is nil")
	}
	if s.schedulerManager == nil {
		return 0, fmt.Errorf("scheduler manager is not initialized")
	}
	if s.storeRuntime == nil {
		return 0, fmt.Errorf("store runtime is not initialized")
	}
	if s.config == nil {
		return 0, fmt.Errorf("config is not initialized")
	}

	created := 0
	var firstErr error
	for _, module := range s.schedulerModules() {
		platformConfig, ok := module.build(s, s.config)
		if !ok {
			continue
		}
		for _, task := range platformScheduledTaskConfigs(platformConfig) {
			if !task.config.Enabled && !s.hasEnabledScheduledTaskConfigs(platformConfig.PlatformName, task.taskType, &firstErr) {
				continue
			}
			if err := s.ensureSchedulerFactory(platformConfig); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			configuredStoreIDs := []int64(nil)
			if task.config.Enabled {
				configuredStoreIDs = task.config.StoreIDs
			}
			storeConfigs := resolveStoreTaskConfigs(
				platformConfig.PlatformName,
				task.taskType,
				configuredStoreIDs,
				getDefaultInterval(task.config.Interval),
				s.storeRuntime,
				s.logger,
			)
			for _, storeConfig := range storeConfigs {
				if s.hasScheduledStoreTask(storeConfig.StoreID, task.taskType) {
					continue
				}
				before := s.schedulerManager.GetTaskCount()
				if err := s.createStoreTask(platformConfig.PlatformName, storeConfig.StoreID, task.taskType, storeConfig.Interval); err != nil {
					if firstErr == nil {
						firstErr = err
					}
					s.logger.Warnf("同步后台定时任务配置创建任务失败 (平台:%s 店铺:%d 类型:%s): %v",
						platformConfig.PlatformName, storeConfig.StoreID, task.taskType, err)
					continue
				}
				if s.schedulerManager.GetTaskCount() > before {
					created++
				}
			}
		}
	}
	return created, firstErr
}

func (s *schedulerServiceImpl) ensureSchedulerFactory(platformConfig platformTaskConfig) error {
	if platformConfig.FactoryCreator == nil {
		return fmt.Errorf("%s平台任务工厂未配置", platformConfig.PlatformName)
	}
	factory := platformConfig.FactoryCreator()
	if factory == nil {
		return fmt.Errorf("%s平台任务工厂为空", platformConfig.PlatformName)
	}
	if _, err := s.schedulerManager.GetRegistry().GetFactory(factory.SupportedPlatform()); err == nil {
		return nil
	}
	return s.schedulerManager.RegisterFactory(factory)
}

func (s *schedulerServiceImpl) hasEnabledScheduledTaskConfigs(platformName string, taskType scheduler.TaskType, firstErr *error) bool {
	configs, err := listEnabledScheduledTaskConfigs(platformName, taskType, s.storeRuntime)
	if err != nil {
		if firstErr != nil && *firstErr == nil {
			*firstErr = err
		}
		s.logger.Warnf("%s平台%s任务读取后台配置失败: %v", platformName, taskType, err)
		return false
	}
	return len(configs) > 0
}

func (s *schedulerServiceImpl) hasScheduledStoreTask(storeID int64, taskType scheduler.TaskType) bool {
	if s == nil || s.schedulerManager == nil {
		return false
	}
	for _, task := range s.schedulerManager.ListTasks() {
		if task.GetStoreID() == storeID && task.GetType() == taskType {
			return true
		}
	}
	return false
}

type platformScheduledTaskConfig struct {
	taskType scheduler.TaskType
	config   taskTypeConfig
}

func platformScheduledTaskConfigs(platformConfig platformTaskConfig) []platformScheduledTaskConfig {
	return []platformScheduledTaskConfig{
		{taskType: scheduler.TaskTypePricing, config: platformConfig.AutoPricing},
		{taskType: scheduler.TaskTypeProductSync, config: platformConfig.ProductSync},
		{taskType: scheduler.TaskTypeInventory, config: platformConfig.InventorySync},
		{taskType: scheduler.TaskTypeActivity, config: platformConfig.ActivityRegistration},
	}
}
