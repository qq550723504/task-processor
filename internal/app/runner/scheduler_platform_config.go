// Package runner 提供处理器和调度器的运行管理功能
package runner

import (
	"strings"
	"time"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
)

// taskTypeConfig 任务类型配置
type taskTypeConfig struct {
	Enabled  bool
	Interval int
	StoreIDs []int64
}

// platformTaskConfig 平台任务配置
type platformTaskConfig struct {
	PlatformName         string
	Enabled              bool
	AutoPricing          taskTypeConfig
	ProductSync          taskTypeConfig
	InventorySync        taskTypeConfig
	ActivityRegistration taskTypeConfig
	FactoryCreator       func() scheduler.TaskFactory
}

// getPlatformConfigs 获取所有平台配置
func (s *schedulerServiceImpl) getPlatformConfigs(cfg *config.Config) []platformTaskConfig {
	configs := make([]platformTaskConfig, 0, len(s.schedulerModules()))
	for _, module := range s.schedulerModules() {
		if !module.enabled(cfg) && !s.hasEnabledScheduledTaskConfigsForPlatform(strings.ToUpper(module.name)) {
			continue
		}
		platformConfig, ok := module.build(s, cfg)
		if !ok {
			continue
		}
		configs = append(configs, platformConfig)
	}
	return configs
}

func (s *schedulerServiceImpl) hasEnabledScheduledTaskConfigsForPlatform(platformName string) bool {
	for _, taskType := range []scheduler.TaskType{
		scheduler.TaskTypePricing,
		scheduler.TaskTypeProductSync,
		scheduler.TaskTypeInventory,
		scheduler.TaskTypeActivity,
	} {
		configs, err := listEnabledScheduledTaskConfigs(platformName, taskType, s.storeRuntime)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("%s平台%s任务读取后台配置失败: %v", platformName, taskType, err)
			}
			continue
		}
		if len(configs) > 0 {
			if s.logger != nil {
				s.logger.Infof("%s平台静态 schedulerEnabled 未启用，但发现 %d 个%s后台启用配置",
					platformName, len(configs), taskType)
			}
			return true
		}
	}
	return false
}

// buildPlatformTaskConfig 从通用的 PlatformConfig 构建 platformTaskConfig
func buildPlatformTaskConfig(name string, pc config.PlatformConfig, factory func() scheduler.TaskFactory) platformTaskConfig {
	return platformTaskConfig{
		PlatformName: name,
		Enabled:      true,
		AutoPricing: taskTypeConfig{
			Enabled:  pc.AutoPricing.Enabled,
			Interval: pc.AutoPricing.Interval,
		},
		ProductSync: taskTypeConfig{
			Enabled:  pc.ProductSync.Enabled,
			Interval: pc.ProductSync.Interval,
			StoreIDs: pc.ProductSync.StoreIDs,
		},
		InventorySync: taskTypeConfig{
			Enabled:  pc.InventorySync.Enabled,
			Interval: pc.InventorySync.Interval,
			StoreIDs: pc.InventorySync.StoreIDs,
		},
		ActivityRegistration: taskTypeConfig{
			Enabled:  pc.ActivityRegistration.Enabled,
			Interval: pc.ActivityRegistration.Interval,
			StoreIDs: pc.ActivityRegistration.StoreIDs,
		},
		FactoryCreator: factory,
	}
}

// getDefaultInterval 获取默认间隔
func getDefaultInterval(interval int) time.Duration {
	if interval <= 0 {
		return 86400 * time.Minute
	}
	return time.Duration(interval) * time.Second
}
