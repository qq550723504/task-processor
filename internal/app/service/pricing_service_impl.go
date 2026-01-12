// Package service 提供核价服务实现
package service

import (
	"fmt"
	"time"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/platforms/temu"
	"task-processor/internal/platforms/temu/api"
	"task-processor/internal/platforms/temu/api/models"
	"task-processor/internal/platforms/temu/services/pricing"

	"github.com/sirupsen/logrus"
)

// initializePricingResources 初始化核价资源
func (s *pricingServiceImpl) initializePricingResources() error {
	s.logger.Info("初始化核价资源...")

	// 获取共享的管理客户端
	s.managementClient = GetSharedManagementClientInstance()
	if s.managementClient == nil {
		return fmt.Errorf("无法获取共享管理客户端")
	}

	s.logger.Info("✅ 核价资源初始化完成")
	return nil
}

// startPricingHandlers 启动各平台核价处理器
func (s *pricingServiceImpl) startPricingHandlers() error {
	s.logger.Info("启动各平台核价处理器...")

	// 获取配置
	cfg := GetGlobalConfig()
	if cfg == nil {
		return fmt.Errorf("无法获取全局配置")
	}

	// 创建统一调度器
	s.scheduler = scheduler.NewSafeScheduler(s.ctx)

	// 启动TEMU核价任务
	if err := s.startTemuPricingTasks(cfg); err != nil {
		return fmt.Errorf("启动TEMU核价任务失败: %w", err)
	}

	// 启动调度器
	if err := s.scheduler.Start(); err != nil {
		return fmt.Errorf("启动调度器失败: %w", err)
	}

	s.logger.Info("✅ 核价处理器启动完成")
	return nil
}

// startTemuPricingTasks 启动TEMU核价任务
func (s *pricingServiceImpl) startTemuPricingTasks(cfg *config.Config) error {
	if !cfg.Platforms.Temu.AutoPricing.Enabled {
		s.logger.Info("⚠️ TEMU自动核价已禁用")
		return nil
	}

	s.logger.Info("启动TEMU核价任务...")

	// 获取核价间隔
	autoPricingInterval := time.Duration(cfg.Platforms.Temu.AutoPricing.Interval) * time.Second
	if autoPricingInterval <= 0 {
		autoPricingInterval = 30 * time.Minute
	}

	// 为每个店铺创建核价任务
	for _, storeID := range cfg.Management.StoreIDs {

		// 检查是否启用Amazon增强功能
		if cfg.Amazon.Enabled {
			// 获取Amazon处理器
			amazonProcessor := GetSharedAmazonProcessor(cfg, s.logger)
			if amazonProcessor != nil {
				// 创建配置提供者
				configProvider := temu.NewDefaultConfigProvider(&cfg.Amazon, amazonProcessor, &cfg.Platforms.Temu)

				// 立即执行一次核价任务
				s.logger.Infof("🚀 立即执行一次Amazon增强版TEMU核价任务 (店铺:%d)", storeID)
				go s.executeImmediatePricingTask(storeID, configProvider)

				// 添加Amazon增强版任务
				scheduler.AddTemuPricingTask(
					s.scheduler,
					s.managementClient,
					storeID,
					autoPricingInterval,
					configProvider,
				)
				s.logger.Infof("✅ 添加Amazon增强版TEMU核价任务 (店铺:%d)", storeID)
			} else {
				// 立即执行一次基础版核价任务
				s.logger.Infof("🚀 立即执行一次基础版TEMU核价任务 (店铺:%d)", storeID)
				go s.executeImmediatePricingTask(storeID, nil)

				// 添加基础版任务
				scheduler.AddTemuPricingTaskBasic(
					s.scheduler,
					s.managementClient,
					storeID,
					autoPricingInterval,
				)
				s.logger.Infof("✅ 添加基础版TEMU核价任务 (店铺:%d)", storeID)
			}
		} else {
			// 立即执行一次基础版核价任务
			s.logger.Infof("🚀 立即执行一次基础版TEMU核价任务 (店铺:%d)", storeID)
			go s.executeImmediatePricingTask(storeID, nil)

			// 添加基础版任务
			scheduler.AddTemuPricingTaskBasic(
				s.scheduler,
				s.managementClient,
				storeID,
				autoPricingInterval,
			)
			s.logger.Infof("✅ 添加基础版TEMU核价任务 (店铺:%d)", storeID)
		}
	}

	return nil
}

// executeImmediatePricingTask 立即执行核价任务
func (s *pricingServiceImpl) executeImmediatePricingTask(storeID int64, configProvider temu.ConfigProvider) {
	logger := s.logger.WithFields(logrus.Fields{
		"component": "ImmediatePricingTask",
		"storeID":   storeID,
	})

	logger.Info("开始立即执行TEMU智能核价任务")

	// 创建API客户端
	apiClient := api.NewAPIClient(storeID, s.managementClient)
	if apiClient == nil {
		logger.Error("创建TEMU API客户端失败")
		return
	}

	// 创建自动核价服务
	autoPricingService := pricing.NewAutoPricingService(apiClient)

	// 执行核价任务
	var stats *models.PricingStatistics
	var err error

	if configProvider != nil {
		// 使用Amazon增强版本
		logger.Info("使用Amazon增强版核价方法")
		stats, err = autoPricingService.AutoProcessPendingPricesWithRulesAndAmazon(s.managementClient, configProvider)
	} else {
		// 使用基础版本
		logger.Info("使用基础版核价方法")
		stats, err = autoPricingService.AutoProcessPendingPricesWithRules(s.managementClient)
	}

	if err != nil {
		logger.WithError(err).Error("立即执行TEMU智能核价任务失败")
		return
	}

	logger.Infof("🎉 立即执行TEMU智能核价任务成功，统计: 总数=%d, 接受=%d, 拒绝=%d, 重新报价=%d, 跳过=%d",
		stats.TotalProcessed, stats.AcceptCount, stats.RejectCount, stats.ReappealCount, stats.SkipCount)
}
