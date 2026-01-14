// Package service 提供任务启动器
package service

import (
	"fmt"
	"time"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
)

// startPlatformTasks 启动平台任务（通用方法）
func (s *pricingServiceImpl) startPlatformTasks(
	platformConfig platformTaskConfig,
	cfg *config.Config,
) error {
	// 检查是否启用
	if !platformConfig.AutoPricing.Enabled {
		s.logger.Infof("⚠️ %s自动核价已禁用", platformConfig.PlatformName)
		return nil
	}

	s.logger.Infof("启动%s核价任务...", platformConfig.PlatformName)

	// 获取核价间隔
	interval := getDefaultInterval(platformConfig.AutoPricing.Interval)

	// 创建并注册工厂
	factory := platformConfig.FactoryCreator()
	if err := s.schedulerManager.RegisterFactory(factory); err != nil {
		return fmt.Errorf("注册%s任务工厂失败: %w", platformConfig.PlatformName, err)
	}

	// 为每个店铺创建任务
	taskCount := 0
	for _, storeID := range cfg.Management.StoreIDs {
		if err := s.createStoreTask(platformConfig.PlatformName, storeID, interval); err != nil {
			s.logger.Errorf("创建%s核价任务失败 (店铺:%d): %v", platformConfig.PlatformName, storeID, err)
			continue
		}
		taskCount++
	}

	if taskCount > 0 {
		s.logger.Infof("✅ 成功启动 %d 个%s核价任务 (间隔:%v)", taskCount, platformConfig.PlatformName, interval)
	} else {
		s.logger.Warnf("⚠️ 没有为%s平台创建任何任务", platformConfig.PlatformName)
	}

	return nil
}

// createStoreTask 为店铺创建任务
func (s *pricingServiceImpl) createStoreTask(
	platformName string,
	storeID int64,
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
		TaskType:  scheduler.TaskTypePricing,
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

	s.logger.Infof("✅ 添加%s核价任务 (店铺:%d)", platformName, storeID)
	return nil
}
