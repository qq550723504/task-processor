// Package runner 提供处理器和调度器的运行管理功能
package runner

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/listingruntime"

	"github.com/sirupsen/logrus"
)

type schedulerStoreRuntime interface {
	GetStore(storeID int64) (*listingruntime.StoreInfo, error)
	ListAutoPricingStoreIDs(ctx context.Context, platformName string) ([]int64, error)
	ListScheduledTaskConfigs(ctx context.Context, platformName string, taskType scheduler.TaskType) ([]listingruntime.ScheduledTaskConfig, error)
	ListScheduledTaskConfigStates(ctx context.Context, platformName string, taskType scheduler.TaskType) ([]listingruntime.ScheduledTaskConfig, error)
}

type resolvedStoreTaskConfig struct {
	StoreID  int64
	Interval time.Duration
}

// startPlatformTasks 启动平台任务
func (s *schedulerServiceImpl) startPlatformTasks(
	platformConfig platformTaskConfig,
	cfg *config.Config,
) error {
	s.logger.Infof("启动%s平台调度任务...", platformConfig.PlatformName)

	// 创建并注册工厂
	factory := platformConfig.FactoryCreator()
	if err := s.schedulerManager.RegisterFactory(factory); err != nil {
		return fmt.Errorf("注册%s任务工厂失败: %w", platformConfig.PlatformName, err)
	}

	// 启动各类任务
	totalTaskCount := 0

	if s.shouldStartTasksByType(platformConfig.PlatformName, scheduler.TaskTypePricing, platformConfig.AutoPricing) {
		count := s.startTasksByType(
			platformConfig.PlatformName,
			scheduler.TaskTypePricing,
			platformConfig.AutoPricing.StoreIDs,
			getDefaultInterval(platformConfig.AutoPricing.Interval),
			cfg,
		)
		totalTaskCount += count
		s.logger.Infof("✅ 成功启动 %d 个%s核价任务", count, platformConfig.PlatformName)
	}

	if s.shouldStartTasksByType(platformConfig.PlatformName, scheduler.TaskTypeProductSync, platformConfig.ProductSync) {
		count := s.startTasksByType(
			platformConfig.PlatformName,
			scheduler.TaskTypeProductSync,
			platformConfig.ProductSync.StoreIDs,
			getDefaultInterval(platformConfig.ProductSync.Interval),
			cfg,
		)
		totalTaskCount += count
		s.logger.Infof("✅ 成功启动 %d 个%s产品同步任务", count, platformConfig.PlatformName)
	}

	if s.shouldStartTasksByType(platformConfig.PlatformName, scheduler.TaskTypeInventory, platformConfig.InventorySync) {
		count := s.startTasksByType(
			platformConfig.PlatformName,
			scheduler.TaskTypeInventory,
			platformConfig.InventorySync.StoreIDs,
			getDefaultInterval(platformConfig.InventorySync.Interval),
			cfg,
		)
		totalTaskCount += count
		s.logger.Infof("✅ 成功启动 %d 个%s库存同步任务", count, platformConfig.PlatformName)
	}

	if s.shouldStartTasksByType(platformConfig.PlatformName, scheduler.TaskTypeActivity, platformConfig.ActivityRegistration) {
		count := s.startTasksByType(
			platformConfig.PlatformName,
			scheduler.TaskTypeActivity,
			platformConfig.ActivityRegistration.StoreIDs,
			getDefaultInterval(platformConfig.ActivityRegistration.Interval),
			cfg,
		)
		totalTaskCount += count
		s.logger.Infof("✅ 成功启动 %d 个%s活动报名任务", count, platformConfig.PlatformName)
	}

	if totalTaskCount > 0 {
		s.logger.Infof("✅ %s平台共启动 %d 个调度任务", platformConfig.PlatformName, totalTaskCount)
	} else {
		s.logger.Warnf("⚠️ %s平台没有启动任何调度任务", platformConfig.PlatformName)
	}

	return nil
}

func (s *schedulerServiceImpl) shouldStartTasksByType(
	platformName string,
	taskType scheduler.TaskType,
	taskConfig taskTypeConfig,
) bool {
	if taskConfig.Enabled {
		return true
	}
	configs, err := listEnabledScheduledTaskConfigs(platformName, taskType, s.storeRuntime)
	if err != nil {
		s.logger.Warnf("%s平台%s任务读取后台配置失败: %v", platformName, taskType, err)
		return false
	}
	if len(configs) == 0 {
		return false
	}
	s.logger.Infof("%s平台%s任务静态开关未启用，但发现 %d 个后台启用配置",
		platformName, taskType, len(configs))
	return true
}

// startTasksByType 按类型启动任务
func (s *schedulerServiceImpl) startTasksByType(
	platformName string,
	taskType scheduler.TaskType,
	configuredStoreIDs []int64,
	interval time.Duration,
	cfg *config.Config,
) int {
	taskCount := 0
	storeConfigs := resolveStoreTaskConfigs(platformName, taskType, configuredStoreIDs, interval, s.storeRuntime, s.logger)

	for _, storeConfig := range storeConfigs {
		if err := s.createStoreTask(platformName, storeConfig.StoreID, taskType, storeConfig.Interval); err != nil {
			s.logger.Debugf("创建%s任务失败 (店铺:%d, 类型:%s): %v",
				platformName, storeConfig.StoreID, taskType, err)
			continue
		}
		taskCount++
	}

	return taskCount
}

func resolveStoreIDsForTask(
	platformName string,
	taskType scheduler.TaskType,
	configuredStoreIDs []int64,
	storeRuntime schedulerStoreRuntime,
	logger *logrus.Logger,
) []int64 {
	configs := resolveStoreTaskConfigs(platformName, taskType, configuredStoreIDs, 0, storeRuntime, logger)
	storeIDs := make([]int64, 0, len(configs))
	for _, config := range configs {
		storeIDs = append(storeIDs, config.StoreID)
	}
	return storeIDs
}

func resolveStoreTaskConfigs(
	platformName string,
	taskType scheduler.TaskType,
	configuredStoreIDs []int64,
	defaultInterval time.Duration,
	storeRuntime schedulerStoreRuntime,
	logger *logrus.Logger,
) []resolvedStoreTaskConfig {
	byStoreID := make(map[int64]resolvedStoreTaskConfig)
	if len(configuredStoreIDs) > 0 {
		storeIDs := dedupeAndSortStoreIDs(configuredStoreIDs)
		if len(storeIDs) == 0 {
			logger.Warnf("%s平台%s任务配置了店铺列表但没有有效店铺ID", platformName, taskType)
		} else {
			logger.Infof("%s平台%s任务使用配置的 %d 个店铺: %v",
				platformName, taskType, len(storeIDs), storeIDs)
			for _, storeID := range storeIDs {
				byStoreID[storeID] = resolvedStoreTaskConfig{StoreID: storeID, Interval: defaultInterval}
			}
		}
	}

	adminConfigs, err := listEnabledScheduledTaskConfigs(platformName, taskType, storeRuntime)
	if err != nil {
		logger.Warnf("%s平台%s任务读取后台配置失败: %v", platformName, taskType, err)
	} else if len(adminConfigs) > 0 {
		logger.Infof("%s平台%s任务读取到 %d 个后台启用配置", platformName, taskType, len(adminConfigs))
		for _, config := range adminConfigs {
			if config.StoreID == 0 || !config.Enabled {
				continue
			}
			if !strings.EqualFold(config.Platform, platformName) || config.TaskType != string(taskType) {
				continue
			}
			interval := defaultInterval
			if config.IntervalSeconds > 0 {
				interval = time.Duration(config.IntervalSeconds) * time.Second
			}
			byStoreID[config.StoreID] = resolvedStoreTaskConfig{StoreID: config.StoreID, Interval: interval}
		}
	}

	if len(byStoreID) == 0 && taskType == scheduler.TaskTypePricing {
		discoveredStoreIDs, err := discoverAutoPricingStoreIDs(platformName, storeRuntime)
		if err != nil {
			logger.Warnf("%s平台自动发现已启用自动核价店铺失败: %v", platformName, err)
			return nil
		}
		if len(discoveredStoreIDs) == 0 {
			logger.Warnf("%s平台未发现已启用自动核价的店铺", platformName)
			return nil
		}

		logger.Infof("%s平台自动发现到 %d 个已启用自动核价的店铺: %v",
			platformName, len(discoveredStoreIDs), discoveredStoreIDs)
		for _, storeID := range discoveredStoreIDs {
			byStoreID[storeID] = resolvedStoreTaskConfig{StoreID: storeID, Interval: defaultInterval}
		}
	}

	if len(byStoreID) == 0 {
		logger.Warnf("%s平台%s任务未配置店铺列表，跳过动态建任务", platformName, taskType)
		return nil
	}

	storeIDs := make([]int64, 0, len(byStoreID))
	for storeID := range byStoreID {
		storeIDs = append(storeIDs, storeID)
	}
	sort.Slice(storeIDs, func(i, j int) bool {
		return storeIDs[i] < storeIDs[j]
	})

	result := make([]resolvedStoreTaskConfig, 0, len(storeIDs))
	for _, storeID := range storeIDs {
		result = append(result, byStoreID[storeID])
	}
	return result
}

func listEnabledScheduledTaskConfigs(
	platformName string,
	taskType scheduler.TaskType,
	storeRuntime schedulerStoreRuntime,
) ([]listingruntime.ScheduledTaskConfig, error) {
	if storeRuntime == nil {
		return nil, nil
	}
	return storeRuntime.ListScheduledTaskConfigs(context.Background(), platformName, taskType)
}

func discoverAutoPricingStoreIDs(platformName string, storeRuntime schedulerStoreRuntime) ([]int64, error) {
	if storeRuntime == nil {
		return nil, fmt.Errorf("店铺运行时未初始化")
	}
	storeIDs, err := storeRuntime.ListAutoPricingStoreIDs(context.Background(), platformName)
	if err != nil {
		return nil, err
	}
	return dedupeAndSortStoreIDs(storeIDs), nil
}

func dedupeAndSortStoreIDs(storeIDs []int64) []int64 {
	if len(storeIDs) == 0 {
		return nil
	}

	seen := make(map[int64]struct{}, len(storeIDs))
	result := make([]int64, 0, len(storeIDs))
	for _, storeID := range storeIDs {
		if storeID == 0 {
			continue
		}
		if _, exists := seen[storeID]; exists {
			continue
		}
		seen[storeID] = struct{}{}
		result = append(result, storeID)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}

// createStoreTask 为店铺创建任务
func (s *schedulerServiceImpl) createStoreTask(
	platformName string,
	storeID int64,
	taskType scheduler.TaskType,
	interval time.Duration,
) error {
	// 获取店铺信息
	storeInfo, err := s.storeRuntime.GetStore(storeID)
	if err != nil {
		return fmt.Errorf("获取店铺信息失败: %w", err)
	}
	if storeInfo == nil {
		return fmt.Errorf("店铺信息不存在")
	}

	// 只处理匹配平台的店铺（大小写不敏感比较，兼容后端返回 "shein"/"SHEIN"/"Shein" 等格式）
	if !strings.EqualFold(storeInfo.Platform, platformName) {
		s.logger.Debugf("店铺 %d 平台不匹配: 期望=%s, 实际=%s，跳过", storeID, platformName, storeInfo.Platform)
		return nil
	}

	// 创建任务配置
	taskConfig := scheduler.TaskConfig{
		TaskType:  taskType,
		Platform:  storeInfo.Platform,
		TenantID:  storeInfo.TenantID,
		StoreID:   storeID,
		Interval:  interval,
		Enabled:   true,
		AutoStart: true,
	}

	// 创建并启动任务
	if err := s.schedulerManager.CreateAndStartTask(taskConfig); err != nil {
		return fmt.Errorf("创建任务失败: %w", err)
	}

	s.logger.Debugf("✅ 添加%s任务 (店铺:%d, 类型:%s)", platformName, storeID, taskType)
	return nil
}

type schedulerStoreRuntimeAdapter struct {
	runtime SchedulerRuntimeProvider
}

func (a schedulerStoreRuntimeAdapter) GetStore(storeID int64) (*listingruntime.StoreInfo, error) {
	if a.runtime == nil {
		return nil, fmt.Errorf("scheduler runtime is not initialized")
	}
	storeService := a.runtime.GetRuntimeStoreService()
	if storeService == nil {
		return nil, fmt.Errorf("runtime store service is not initialized")
	}
	return storeService.GetStore(storeID)
}

func (a schedulerStoreRuntimeAdapter) ListAutoPricingStoreIDs(ctx context.Context, platformName string) ([]int64, error) {
	if a.runtime == nil {
		return nil, fmt.Errorf("scheduler runtime is not initialized")
	}
	return a.runtime.ListRuntimeAutoPricingStoreIDs(ctx, platformName)
}

func (a schedulerStoreRuntimeAdapter) ListScheduledTaskConfigs(ctx context.Context, platformName string, taskType scheduler.TaskType) ([]listingruntime.ScheduledTaskConfig, error) {
	if a.runtime == nil {
		return nil, fmt.Errorf("scheduler runtime is not initialized")
	}
	return a.runtime.ListRuntimeScheduledTaskConfigs(ctx, platformName, taskType)
}

func (a schedulerStoreRuntimeAdapter) ListScheduledTaskConfigStates(ctx context.Context, platformName string, taskType scheduler.TaskType) ([]listingruntime.ScheduledTaskConfig, error) {
	if a.runtime == nil {
		return nil, fmt.Errorf("scheduler runtime is not initialized")
	}
	return a.runtime.ListRuntimeScheduledTaskConfigStates(ctx, platformName, taskType)
}
