// Package runner 提供处理器和调度器的运行管理功能
package runner

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	managementapi "task-processor/internal/infra/clients/management/api"

	"github.com/sirupsen/logrus"
)

const storeDiscoveryPageSize = 200

type schedulerStoreClient interface {
	GetStore(storeID int64) (*managementapi.StoreRespDTO, error)
	PageStores(req *managementapi.StorePageReqDTO) (*managementapi.PageResult[*managementapi.StoreRespDTO], error)
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

	if platformConfig.AutoPricing.Enabled {
		count := s.startTasksByType(
			platformConfig.PlatformName,
			scheduler.TaskTypePricing,
			getDefaultInterval(platformConfig.AutoPricing.Interval),
			cfg,
		)
		totalTaskCount += count
		s.logger.Infof("✅ 成功启动 %d 个%s核价任务", count, platformConfig.PlatformName)
	}

	if platformConfig.ProductSync.Enabled {
		count := s.startTasksByType(
			platformConfig.PlatformName,
			scheduler.TaskTypeProductSync,
			getDefaultInterval(platformConfig.ProductSync.Interval),
			cfg,
		)
		totalTaskCount += count
		s.logger.Infof("✅ 成功启动 %d 个%s产品同步任务", count, platformConfig.PlatformName)
	}

	if platformConfig.InventorySync.Enabled {
		count := s.startTasksByType(
			platformConfig.PlatformName,
			scheduler.TaskTypeInventory,
			getDefaultInterval(platformConfig.InventorySync.Interval),
			cfg,
		)
		totalTaskCount += count
		s.logger.Infof("✅ 成功启动 %d 个%s库存同步任务", count, platformConfig.PlatformName)
	}

	if platformConfig.ActivityRegistration.Enabled {
		count := s.startTasksByType(
			platformConfig.PlatformName,
			scheduler.TaskTypeActivity,
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

// startTasksByType 按类型启动任务
func (s *schedulerServiceImpl) startTasksByType(
	platformName string,
	taskType scheduler.TaskType,
	interval time.Duration,
	cfg *config.Config,
) int {
	taskCount := 0
	storeIDs := resolveStoreIDsForTask(platformName, taskType, cfg.Management.StoreIDs, s.managementClient.GetStoreClient(), s.logger)

	for _, storeID := range storeIDs {
		if err := s.createStoreTask(platformName, storeID, taskType, interval); err != nil {
			s.logger.Debugf("创建%s任务失败 (店铺:%d, 类型:%s): %v",
				platformName, storeID, taskType, err)
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
	storeClient schedulerStoreClient,
	logger *logrus.Logger,
) []int64 {
	if len(configuredStoreIDs) > 0 {
		logger.Infof("%s平台调度任务使用 management.storeIDs 作为店铺白名单: %v", platformName, configuredStoreIDs)
		return dedupeAndSortStoreIDs(configuredStoreIDs)
	}

	if taskType != scheduler.TaskTypePricing {
		logger.Warnf("%s平台%s任务未配置 management.storeIDs，当前仅自动发现核价店铺，跳过动态建任务",
			platformName, taskType)
		return nil
	}

	discoveredStoreIDs, err := discoverAutoPricingStoreIDs(platformName, storeClient)
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
	return discoveredStoreIDs
}

func discoverAutoPricingStoreIDs(platformName string, storeClient schedulerStoreClient) ([]int64, error) {
	if storeClient == nil {
		return nil, fmt.Errorf("店铺客户端未初始化")
	}

	enableAutoPrice := true
	pageNo := 1
	storeIDs := make([]int64, 0, storeDiscoveryPageSize)

	for {
		page, err := storeClient.PageStores(&managementapi.StorePageReqDTO{
			Platform:        strings.ToLower(platformName),
			PageNo:          pageNo,
			PageSize:        storeDiscoveryPageSize,
			EnableAutoPrice: &enableAutoPrice,
		})
		if err != nil {
			return nil, err
		}
		if page == nil || len(page.List) == 0 {
			break
		}

		for _, store := range page.List {
			if store == nil || store.ID == 0 {
				continue
			}
			storeIDs = append(storeIDs, store.ID)
		}

		if page.Total > 0 && int64(pageNo*page.PageSize) >= page.Total {
			break
		}
		if len(page.List) < storeDiscoveryPageSize {
			break
		}
		pageNo++
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
	storeInfo, err := s.managementClient.GetStoreClient().GetStore(storeID)
	if err != nil {
		return fmt.Errorf("获取店铺信息失败: %w", err)
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
