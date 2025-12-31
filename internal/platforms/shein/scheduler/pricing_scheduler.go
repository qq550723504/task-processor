// Package scheduler 提供SHEIN平台的核价调度功能
package scheduler

import (
	"context"
	"runtime"
	"sync"
	"time"

	shops "task-processor/internal/common/shein"
	"task-processor/internal/infra/memory"
	"task-processor/internal/pkg/management"
	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/modules"

	"github.com/sirupsen/logrus"
)

// PricingScheduler SHEIN自动核价调度器
type PricingScheduler struct {
	tenantID         int64
	storeID          int64
	managementClient *management.ClientManager
	shopClientMgr    *shops.ClientManager
	interval         time.Duration
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	logger           *logrus.Entry

	// 保留原有的组件化设计
	productFetcher  *modules.AutoPricingProductFetcher
	ruleEvaluator   *modules.AutoPricingRuleEvaluator
	priceCalculator *modules.AutoPricingCalculator
	discussHandler  *modules.AutoPricingDiscussHandler
}

// NewPricingScheduler 创建自动核价调度器
func NewPricingScheduler(ctx context.Context, tenantID, storeID int64, managementClient *management.ClientManager, interval time.Duration) *PricingScheduler {
	schedulerCtx, cancel := context.WithCancel(ctx)

	// 创建店铺客户端管理器
	cookieManager := memory.NewCookieManager()
	shopClientMgr := shops.NewClientManager(cookieManager, managementClient)

	return &PricingScheduler{
		tenantID:         tenantID,
		storeID:          storeID,
		managementClient: managementClient,
		shopClientMgr:    shopClientMgr,
		interval:         interval,
		ctx:              schedulerCtx,
		cancel:           cancel,
		logger: logrus.WithFields(logrus.Fields{
			"component": "SHEINPricingScheduler",
			"tenantID":  tenantID,
			"storeID":   storeID,
		}),
		// 初始化组件
		productFetcher:  modules.NewAutoPricingProductFetcher(shopClientMgr),
		ruleEvaluator:   modules.NewAutoPricingRuleEvaluator(managementClient),
		priceCalculator: modules.NewAutoPricingCalculator(),
		discussHandler:  modules.NewAutoPricingDiscussHandler(),
	}
}

// NewPricingSchedulerWithPrewarmer 使用预热器创建自动核价调度器
func NewPricingSchedulerWithPrewarmer(ctx context.Context, tenantID, storeID int64, managementClient *management.ClientManager, prewarmer *CookiePrewarmer, interval time.Duration) *PricingScheduler {
	schedulerCtx, cancel := context.WithCancel(ctx)

	// 使用预热器的客户端管理器
	shopClientMgr := prewarmer.GetShopClientManager()

	return &PricingScheduler{
		tenantID:         tenantID,
		storeID:          storeID,
		managementClient: managementClient,
		shopClientMgr:    shopClientMgr,
		interval:         interval,
		ctx:              schedulerCtx,
		cancel:           cancel,
		logger: logrus.WithFields(logrus.Fields{
			"component": "SHEINPricingScheduler",
			"tenantID":  tenantID,
			"storeID":   storeID,
		}),
		// 初始化组件
		productFetcher:  modules.NewAutoPricingProductFetcher(shopClientMgr),
		ruleEvaluator:   modules.NewAutoPricingRuleEvaluator(managementClient),
		priceCalculator: modules.NewAutoPricingCalculator(),
		discussHandler:  modules.NewAutoPricingDiscussHandler(),
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
	s.logger.Info("开始执行SHEIN自动核价任务")

	// 获取店铺信息
	storeInfo, err := s.managementClient.GetStoreClient().GetStore(s.storeID)
	if err != nil {
		s.logger.Errorf("获取店铺信息失败: %v", err)
		return
	}

	s.logger.Infof("店铺 %s (ID: %d) 自动核价状态: %v", storeInfo.Name, storeInfo.ID,
		storeInfo.EnableAutoPrice != nil && *storeInfo.EnableAutoPrice)

	// 检查是否启用自动核价
	if storeInfo.EnableAutoPrice != nil && !*storeInfo.EnableAutoPrice {
		s.logger.Infof("店铺 %s 未启用自动核价功能，跳过", storeInfo.Name)
		return
	}

	// 执行核价逻辑
	stats := s.performAutoPricing(storeInfo)

	s.logger.Infof("SHEIN自动核价任务执行完成，耗时: %v, 统计: 总数=%d, 接受=%d, 拒绝=%d, 跳过=%d",
		time.Since(startTime), stats.TotalProcessed, stats.AcceptCount,
		stats.RejectCount, stats.SkipCount)
}

// PricingStatistics 核价统计信息
type PricingStatistics struct {
	TotalProcessed int
	AcceptCount    int
	RejectCount    int
	SkipCount      int
}

// performAutoPricing 执行自动核价操作
func (s *PricingScheduler) performAutoPricing(storeInfo *managementapi.StoreRespDTO) *PricingStatistics {
	stats := &PricingStatistics{}

	// 获取待核价的产品列表
	products, err := s.productFetcher.GetPendingPricingProducts(s.tenantID, s.storeID, storeInfo)
	if err != nil {
		s.logger.Errorf("获取待核价产品失败: %v", err)
		return stats
	}

	if len(products) == 0 {
		s.logger.Infof("店铺 %s 没有待核价的产品", storeInfo.Name)
		return stats
	}

	s.logger.Infof("店铺 %s 获取到 %d 个待核价产品", storeInfo.Name, len(products))

	// 处理每个产品的核价逻辑
	for _, product := range products {
		s.processProductPricing(storeInfo, product, stats)
	}

	return stats
}

// processProductPricing 处理单个产品的核价逻辑
func (s *PricingScheduler) processProductPricing(storeInfo *managementapi.StoreRespDTO, product interface{}, stats *PricingStatistics) {
	stats.TotalProcessed++

	// 获取店铺API客户端
	client, err := s.shopClientMgr.GetClient(s.tenantID, s.storeID, storeInfo)
	if err != nil {
		s.logger.Errorf("获取店铺API客户端失败: %v", err)
		stats.SkipCount++
		return
	}

	// 获取自动核价规则并评估
	action, reason, batchReq := s.ruleEvaluator.EvaluateProductPricing(s.tenantID, s.storeID, product)

	// 更新统计信息
	switch action {
	case "accept":
		stats.AcceptCount++
	case "reject":
		stats.RejectCount++
	case "skip":
		stats.SkipCount++
	}

	// 如果有处理请求，则调用批量处理成本讨论接口
	if batchReq != nil && action != "skip" {
		if err := s.discussHandler.HandleCostDiscuss(client, batchReq); err != nil {
			s.logger.Errorf("处理成本讨论失败: %v", err)
		}
	}

	s.logger.Infof("产品核价完成: %s", reason)
}
