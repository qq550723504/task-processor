package runner

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/listingruntime"

	"github.com/sirupsen/logrus"
)

const dynamicScheduledTaskConfigSyncInterval = 10 * time.Second

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
			changed, err := s.syncEnabledScheduledTaskConfigs(ctx)
			if err != nil {
				s.logger.Warnf("同步后台定时任务配置失败: %v", err)
				continue
			}
			if changed > 0 {
				s.logger.Infof("同步后台定时任务配置完成，变更 %d 个任务", changed)
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

	changed := 0
	var firstErr error
	for _, module := range s.schedulerModules() {
		platformConfig, ok := module.build(s, s.config)
		if !ok {
			continue
		}
		for _, task := range platformScheduledTaskConfigs(platformConfig) {
			states, err := listScheduledTaskConfigStates(ctx, platformConfig.PlatformName, task.taskType, s.storeRuntime)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				s.logger.Warnf("%s平台%s任务读取后台配置状态失败: %v", platformConfig.PlatformName, task.taskType, err)
				states = nil
			}

			stopped := s.stopDisabledScheduledStoreTasks(platformConfig.PlatformName, task.taskType, states)
			changed += stopped

			if !task.config.Enabled && !hasEnabledScheduledTaskConfigState(states) {
				continue
			}
			if err := s.ensureSchedulerFactory(platformConfig); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			storeConfigs := resolveStoreTaskConfigsFromStates(
				platformConfig.PlatformName,
				task.taskType,
				task.config,
				getDefaultInterval(task.config.Interval),
				states,
				s.storeRuntime,
				s.logger,
			)
			for _, storeConfig := range storeConfigs {
				taskID, existingTask, exists := s.findScheduledStoreTask(platformConfig.PlatformName, storeConfig.StoreID, task.taskType)
				if exists && existingTask.GetInterval() == storeConfig.Interval {
					continue
				}
				replaced := false
				if exists {
					if err := s.schedulerManager.RemoveTask(taskID); err != nil {
						if firstErr == nil {
							firstErr = err
						}
						s.logger.Warnf("同步后台定时任务配置移除旧任务失败 (平台:%s 店铺:%d 类型:%s): %v",
							platformConfig.PlatformName, storeConfig.StoreID, task.taskType, err)
						continue
					}
					replaced = true
				}
				if err := s.createStoreTask(platformConfig.PlatformName, storeConfig.StoreID, task.taskType, storeConfig.Interval); err != nil {
					if firstErr == nil {
						firstErr = err
					}
					s.logger.Warnf("同步后台定时任务配置创建任务失败 (平台:%s 店铺:%d 类型:%s): %v",
						platformConfig.PlatformName, storeConfig.StoreID, task.taskType, err)
					continue
				}
				changed++
				if replaced {
					s.logger.Infof("同步后台定时任务配置已按新间隔重建任务 (平台:%s 店铺:%d 类型:%s 间隔:%s)",
						platformConfig.PlatformName, storeConfig.StoreID, task.taskType, storeConfig.Interval)
				}
			}
		}
	}
	return changed, firstErr
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

func (s *schedulerServiceImpl) hasScheduledStoreTask(storeID int64, taskType scheduler.TaskType) bool {
	_, _, ok := s.findScheduledStoreTask("", storeID, taskType)
	return ok
}

func (s *schedulerServiceImpl) findScheduledStoreTask(platformName string, storeID int64, taskType scheduler.TaskType) (string, scheduler.Task, bool) {
	if s == nil || s.schedulerManager == nil {
		return "", nil, false
	}
	for _, task := range s.schedulerManager.ListTasks() {
		if task.GetStoreID() != storeID || task.GetType() != taskType {
			continue
		}
		if platformName != "" && !strings.EqualFold(task.GetPlatform(), platformName) {
			continue
		}
		return task.GetID(), task, true
	}
	return "", nil, false
}

func (s *schedulerServiceImpl) stopDisabledScheduledStoreTasks(platformName string, taskType scheduler.TaskType, states []listingruntime.ScheduledTaskConfig) int {
	stopped := 0
	for _, state := range states {
		if state.Enabled || state.StoreID == 0 {
			continue
		}
		if !strings.EqualFold(state.Platform, platformName) || state.TaskType != string(taskType) {
			continue
		}
		taskID, _, exists := s.findScheduledStoreTask(platformName, state.StoreID, taskType)
		if !exists {
			continue
		}
		if err := s.schedulerManager.RemoveTask(taskID); err != nil {
			s.logger.Warnf("后台定时任务配置已禁用但停止任务失败 (平台:%s 店铺:%d 类型:%s): %v",
				platformName, state.StoreID, taskType, err)
			continue
		}
		stopped++
	}
	return stopped
}

func hasEnabledScheduledTaskConfigState(states []listingruntime.ScheduledTaskConfig) bool {
	for _, state := range states {
		if state.Enabled {
			return true
		}
	}
	return false
}

func listScheduledTaskConfigStates(
	ctx context.Context,
	platformName string,
	taskType scheduler.TaskType,
	storeRuntime schedulerStoreRuntime,
) ([]listingruntime.ScheduledTaskConfig, error) {
	if storeRuntime == nil {
		return nil, nil
	}
	return storeRuntime.ListScheduledTaskConfigStates(ctx, platformName, taskType)
}

func resolveStoreTaskConfigsFromStates(
	platformName string,
	taskType scheduler.TaskType,
	taskConfig taskTypeConfig,
	defaultInterval time.Duration,
	states []listingruntime.ScheduledTaskConfig,
	storeRuntime schedulerStoreRuntime,
	logger *logrus.Logger,
) []resolvedStoreTaskConfig {
	configuredStoreIDs := []int64(nil)
	if taskConfig.Enabled {
		configuredStoreIDs = taskConfig.StoreIDs
	}
	byStoreID := make(map[int64]resolvedStoreTaskConfig)
	for _, storeID := range dedupeAndSortStoreIDs(configuredStoreIDs) {
		byStoreID[storeID] = resolvedStoreTaskConfig{StoreID: storeID, Interval: defaultInterval}
	}

	disabledStoreIDs := make(map[int64]struct{})
	for _, state := range states {
		if state.StoreID == 0 {
			continue
		}
		if !strings.EqualFold(state.Platform, platformName) || state.TaskType != string(taskType) {
			continue
		}
		if !state.Enabled {
			disabledStoreIDs[state.StoreID] = struct{}{}
			delete(byStoreID, state.StoreID)
			continue
		}
		interval := defaultInterval
		if state.IntervalSeconds > 0 {
			interval = time.Duration(state.IntervalSeconds) * time.Second
		}
		byStoreID[state.StoreID] = resolvedStoreTaskConfig{StoreID: state.StoreID, Interval: interval}
	}

	if len(byStoreID) == 0 && taskType == scheduler.TaskTypePricing && taskConfig.Enabled {
		discoveredStoreIDs, err := discoverAutoPricingStoreIDs(platformName, storeRuntime)
		if err != nil {
			logger.Warnf("%s平台自动发现已启用自动核价店铺失败: %v", platformName, err)
			return nil
		}
		for _, storeID := range discoveredStoreIDs {
			if _, disabled := disabledStoreIDs[storeID]; disabled {
				continue
			}
			byStoreID[storeID] = resolvedStoreTaskConfig{StoreID: storeID, Interval: defaultInterval}
		}
	}

	if len(byStoreID) == 0 {
		return nil
	}
	storeIDs := make([]int64, 0, len(byStoreID))
	for storeID := range byStoreID {
		if _, disabled := disabledStoreIDs[storeID]; disabled {
			continue
		}
		storeIDs = append(storeIDs, storeID)
	}
	sort.Slice(storeIDs, func(i, j int) bool {
		return storeIDs[i] < storeIDs[j]
	})

	result := make([]resolvedStoreTaskConfig, 0, len(storeIDs))
	for _, storeID := range storeIDs {
		result = append(result, byStoreID[storeID])
	}
	if len(result) == 0 {
		logger.Warnf("%s平台%s任务后台状态未解析出启用店铺", platformName, taskType)
	}
	return result
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
