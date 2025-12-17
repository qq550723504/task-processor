package client

import (
	"context"
	"runtime"
	"sync"
	"task-processor/internal/common/management"
	"time"

	"github.com/sirupsen/logrus"
)

// PricingAction 核价操作类型
type PricingAction string

const (
	ActionSmart PricingAction = "smart" // 智能核价（根据利润率规则）
)

// PricingScheduler 自动核价调度器
type PricingScheduler struct {
	apiClient        *APIClient
	managementClient *management.ClientManager
	interval         time.Duration
	action           PricingAction
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	logger           *logrus.Entry
}

// NewPricingScheduler 创建自动核价调度器
func NewPricingScheduler(apiClient *APIClient, managementClient *management.ClientManager, interval time.Duration, action PricingAction) *PricingScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &PricingScheduler{
		apiClient:        apiClient,
		managementClient: managementClient,
		interval:         interval,
		action:           action,
		ctx:              ctx,
		cancel:           cancel,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TEMUPricingScheduler",
			"tenantID":  apiClient.GetTenantID(),
			"storeID":   apiClient.GetStoreID(),
			"action":    action,
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

	stats, err := s.apiClient.AutoProcessPendingPricesWithRules(s.managementClient)
	if err != nil {
		s.logger.WithError(err).Error("智能核价任务执行失败")
	} else {
		s.logger.Infof("智能核价任务执行成功，耗时: %v, 统计: 总数=%d, 接受=%d, 拒绝=%d, 重新报价=%d, 跳过=%d",
			time.Since(startTime), stats.TotalProcessed, stats.AcceptCount,
			stats.RejectCount, stats.ReappealCount, stats.SkipCount)
	}
}
