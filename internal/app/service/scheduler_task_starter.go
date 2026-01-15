// Package service 提供调度任务启动器
package service

import (
	"fmt"
	"time"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
)

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

	// 1. 启动核价任务
	if platformConfig.AutoPricing.Enabled {
		count, err := s.startTasksByType(
			platformConfig.PlatformName,
			scheduler.TaskTypePricing,
			getDefaultInterval(platformConfig.AutoPricing.Interval),
			cfg,
		)
		if err != nil {
			s.logger.Errorf("启动%s核价任务失败: %v", platformConfig.PlatformName, err)
		} else {
			totalTaskCount += count
			s.logger.Infof("✅ 成功启动 %d 个%s核价任务", count, platformConfig.PlatformName)
		}
	}

	// 2. 启动产品同步任务
	if platformConfig.ProductSync.Enabled {
		count, err := s.startTasksByType(
			platformConfig.PlatformName,
			scheduler.TaskTypeProductSync,
			getDefaultInterval(platformConfig.ProductSync.Interval),
			cfg,
		)
		if err != nil {
			s.logger.Errorf("启动%s产品同步任务失败: %v", platformConfig.PlatformName, err)
		} else {
			totalTaskCount += count
			s.logger.Infof("✅ 成功启动 %d 个%s产品同步任务", count, platformConfig.PlatformName)
		}
	}

	// 3. 启动库存同步任务
	if platformConfig.InventorySync.Enabled {
		count, err := s.startTasksByType(
			platformConfig.PlatformName,
			scheduler.TaskTypeInventory,
			getDefaultInterval(platformConfig.InventorySync.Interval),
			cfg,
		)
		if err != nil {
			s.logger.Errorf("启动%s库存同步任务失败: %v", platformConfig.PlatformName, err)
		} else {
			totalTaskCount += count
			s.logger.Infof("✅ 成功启动 %d 个%s库存同步任务", count, platformConfig.PlatformName)
		}
	}

	// 4. 启动活动报名任务
	if platformConfig.ActivityRegistration.Enabled {
		count, err := s.startTasksByType(
			platformConfig.PlatformName,
			scheduler.TaskTypeActivity,
			getDefaultInterval(platformConfig.ActivityRegistration.Interval),
			cfg,
		)
		if err != nil {
			s.logger.Errorf("启动%s活动报名任务失败: %v", platformConfig.PlatformName, err)
		} else {
			totalTaskCount += count
			s.logger.Infof("✅ 成功启动 %d 个%s活动报名任务", count, platformConfig.PlatformName)
		}
	}

	if totalTaskCount > 0 {
		s.logger.Infof("✅ %s平台共启动 %d 个调度任务", platformConfig.PlatformName, totalTaskCount)
	} else {
		s.logger.Warnf("⚠️ %s平台没有启动任何调度任务", platformConfig.PlatformName)
	}

	return nil
}

// startTasksByType 按类型启动任务
func (s *schedulerServiceImpl) startTasksByType(
	platformName string,
	taskType scheduler.TaskType,
	interval time.Duration,
	cfg *config.Config,
) (int, error) {
	taskCount := 0

	// 为每个店铺创建任务
	for _, storeID := range cfg.Management.StoreIDs {
		if err := s.createStoreTask(platformName, storeID, taskType, interval); err != nil {
			s.logger.Debugf("创建%s任务失败 (店铺:%d, 类型:%s): %v",
				platformName, storeID, taskType, err)
			continue
		}
		taskCount++
	}

	return taskCount, nil
}

// createStoreTask 为店铺创建任务
func (s *schedulerServiceImpl) createStoreTask(
	platformName string,
	storeID int64,
	taskType scheduler.TaskType,
	interval time.Duration,
) error {
	// 获取店铺信息
	storeInfo, err := s.managementClient.GetStoreClient().GetStore(storeID)
	if err != nil {
		return fmt.Errorf("获取店铺信息失败: %w", err)
	}

	// 只处理匹配平台的店铺
	if storeInfo.Platform != platformName {
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
