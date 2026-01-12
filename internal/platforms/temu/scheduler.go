package temu

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/temu/api"
	"task-processor/internal/platforms/temu/api/models"
	"time"

	"github.com/sirupsen/logrus"
)

// PricingAction 核价操作类型
type PricingAction string

// PricingScheduler 自动核价调度器
type PricingScheduler struct {
	apiClient        api.APIClientInterface
	managementClient *management.ClientManager
	configProvider   ConfigProvider // 配置提供者，用于Amazon增强功能
	interval         time.Duration
	action           PricingAction
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	logger           *logrus.Entry
}

// NewPricingScheduler 创建自动核价调度器
func NewPricingScheduler(ctx context.Context, apiClient api.APIClientInterface, managementClient *management.ClientManager, interval time.Duration, action PricingAction) *PricingScheduler {
	schedulerCtx, cancel := context.WithCancel(ctx)

	return &PricingScheduler{
		apiClient:        apiClient,
		managementClient: managementClient,
		configProvider:   nil, // 默认不使用Amazon增强功能
		interval:         interval,
		action:           action,
		ctx:              schedulerCtx,
		cancel:           cancel,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TEMUPricingScheduler",
			"storeID":   apiClient.GetStoreID(),
			"action":    action,
		}),
	}
}

// NewPricingSchedulerWithAmazon 创建支持Amazon的自动核价调度器
func NewPricingSchedulerWithAmazon(
	ctx context.Context,
	apiClient api.APIClientInterface,
	managementClient *management.ClientManager,
	configProvider ConfigProvider,
	interval time.Duration,
	action PricingAction,
) *PricingScheduler {
	schedulerCtx, cancel := context.WithCancel(ctx)

	return &PricingScheduler{
		apiClient:        apiClient,
		managementClient: managementClient,
		configProvider:   configProvider,
		interval:         interval,
		action:           action,
		ctx:              schedulerCtx,
		cancel:           cancel,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TEMUPricingScheduler",
			"storeID":   apiClient.GetStoreID(),
			"action":    action,
			"amazon":    configProvider != nil,
		}),
	}
}

// Start 启动定时任务
func (s *PricingScheduler) Start() {
	s.logger.Infof("启动自动核价定时任务，间隔: %v", s.interval)

	s.wg.Add(1)
	go s.run()
}

// Stop 停止定时任务
func (s *PricingScheduler) Stop() {
	s.logger.Info("停止自动核价定时任务")
	s.cancel()
	s.wg.Wait()
	s.logger.Info("自动核价定时任务已停止")
}

// run 运行定时任务
func (s *PricingScheduler) run() {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorf("自动核价定时任务发生panic: %v", r)
		}
	}()

	// 立即执行一次
	s.executeTask()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("收到停止信号，退出定时任务")
			return
		case <-ticker.C:
			s.executeTask()
		}
	}
}

// executeTask 执行核价任务
func (s *PricingScheduler) executeTask() {
	defer func() {
		if r := recover(); r != nil {
			// 获取堆栈信息
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			s.logger.Errorf("执行核价任务时发生panic: %v\n堆栈:\n%s", r, string(buf[:n]))
		}
	}()

	startTime := time.Now()

	s.logger.Info("开始执行智能核价任务")

	if s.apiClient == nil {
		s.logger.Error("apiClient为空，无法执行核价任务")
		return
	}

	if s.managementClient == nil {
		s.logger.Error("managementClient为空，无法执行核价任务")
		return
	}

	stats, err := s.executeWithBestAvailableMethod()
	if err != nil {
		s.logger.WithError(err).Error("❌ 智能核价任务执行失败")
	} else {
		s.logger.Infof("🎉 智能核价任务执行成功，耗时: %v, 统计: 总数=%d, 接受=%d, 拒绝=%d, 重新报价=%d, 跳过=%d",
			time.Since(startTime), stats.TotalProcessed, stats.AcceptCount,
			stats.RejectCount, stats.ReappealCount, stats.SkipCount)
	}
}

// executeWithBestAvailableMethod 使用最佳可用方法执行核价
func (s *PricingScheduler) executeWithBestAvailableMethod() (*models.PricingStatistics, error) {
	// 直接在这里实现核价逻辑，避免循环导入
	// 这里应该调用 AutoPricingService，但为了避免循环导入，我们暂时直接实现

	s.logger.Info("开始执行核价任务")

	// TODO: 这里需要重构架构来避免循环导入
	// 暂时返回错误，提示需要使用正确的调用方式
	return nil, fmt.Errorf("当前调度器架构需要重构以避免循环导入，请使用 internal/app/scheduler 中的统一调度器")
}

// configProviderAdapter 配置提供者适配器，用于适配不同的ConfigProvider接口
type configProviderAdapter struct {
	provider ConfigProvider
}

func (c *configProviderAdapter) GetAmazonConfig() any {
	return c.provider.GetAmazonConfig()
}

func (c *configProviderAdapter) GetAmazonProcessor() any {
	return c.provider.GetAmazonProcessor()
}
