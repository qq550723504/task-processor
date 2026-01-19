// Package scheduler 提供TEMU产品同步定时任务调度器
package scheduler

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/temu/api/models"
	"task-processor/internal/platforms/temu/api/services"
	schedulerservice "task-processor/internal/platforms/temu/service/scheduler"

	"github.com/sirupsen/logrus"
)

// SyncScheduler TEMU产品同步调度器
type SyncScheduler struct {
	managementClient *management.ClientManager
	productAPI       *services.ProductAPI
	serviceFactory   *schedulerservice.ServiceFactory
	logger           *logrus.Entry
	isRunning        bool
	stopChan         chan struct{}
}

// NewSyncScheduler 创建TEMU产品同步调度器
func NewSyncScheduler(
	managementClient *management.ClientManager,
	productAPI *services.ProductAPI,
	serviceFactory *schedulerservice.ServiceFactory,
) *SyncScheduler {
	return &SyncScheduler{
		managementClient: managementClient,
		productAPI:       productAPI,
		serviceFactory:   serviceFactory,
		logger:           logrus.WithField("component", "TemuSyncScheduler"),
		stopChan:         make(chan struct{}),
	}
}

// Start 启动定时同步任务
func (s *SyncScheduler) Start(ctx context.Context, interval time.Duration) error {
	if s.isRunning {
		return fmt.Errorf("TEMU同步调度器已在运行")
	}

	s.isRunning = true
	s.logger.WithField("interval", interval).Info("启动TEMU产品同步定时任务")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 立即执行一次同步
	if err := s.executeSync(ctx); err != nil {
		s.logger.WithError(err).Error("首次同步执行失败")
	}

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("收到上下文取消信号，停止TEMU同步调度器")
			s.isRunning = false
			return ctx.Err()

		case <-s.stopChan:
			s.logger.Info("收到停止信号，停止TEMU同步调度器")
			s.isRunning = false
			return nil

		case <-ticker.C:
			s.logger.Debug("定时器触发，开始执行TEMU产品同步")
			if err := s.executeSync(ctx); err != nil {
				s.logger.WithError(err).Error("定时同步执行失败")
			}
		}
	}
}

// Stop 停止定时同步任务
func (s *SyncScheduler) Stop() {
	if !s.isRunning {
		s.logger.Warn("TEMU同步调度器未在运行")
		return
	}

	s.logger.Info("正在停止TEMU同步调度器")
	close(s.stopChan)
}

// ExecuteSyncNow 立即执行一次同步
func (s *SyncScheduler) ExecuteSyncNow(ctx context.Context) error {
	s.logger.Info("手动触发TEMU产品同步")
	return s.executeSync(ctx)
}

// executeSync 执行同步逻辑
func (s *SyncScheduler) executeSync(ctx context.Context) error {
	startTime := time.Now()
	s.logger.Info("开始执行TEMU产品同步任务")

	// 创建产品同步服务
	syncService := s.serviceFactory.CreateProductSyncService()
	if syncService == nil {
		return fmt.Errorf("无法创建产品同步服务")
	}

	// 1. 获取产品列表
	products, err := syncService.FetchProductList(ctx)
	if err != nil {
		return fmt.Errorf("获取TEMU产品列表失败: %w", err)
	}

	if len(products) == 0 {
		s.logger.Info("没有找到TEMU产品，跳过同步")
		return nil
	}

	s.logger.Infof("获取到 %d 个TEMU产品", len(products))

	// 2. 获取需要同步的店铺列表
	stores, err := s.getActiveStores(ctx)
	if err != nil {
		return fmt.Errorf("获取活跃店铺列表失败: %w", err)
	}

	totalSynced := 0
	for _, store := range stores {
		synced, err := s.syncToStore(ctx, syncService, products, store)
		if err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"tenant_id": store.TenantID,
				"store_id":  store.ID,
			}).Error("同步到店铺失败")
			continue
		}
		totalSynced += synced
	}

	duration := time.Since(startTime)
	s.logger.WithFields(logrus.Fields{
		"total_products": len(products),
		"total_synced":   totalSynced,
		"stores_count":   len(stores),
		"duration":       duration,
	}).Info("TEMU产品同步任务完成")

	return nil
}

// syncToStore 同步产品到指定店铺
func (s *SyncScheduler) syncToStore(ctx context.Context, syncService schedulerservice.ProductSyncService, products []models.TemuProductResponse, store *StoreInfo) (int, error) {
	s.logger.WithFields(logrus.Fields{
		"tenant_id": store.TenantID,
		"store_id":  store.ID,
	}).Info("开始同步产品到店铺")

	// 转换产品格式
	productDataList, err := syncService.ConvertProducts(ctx, products, store.TenantID, store.ID)
	if err != nil {
		return 0, fmt.Errorf("转换产品格式失败: %w", err)
	}

	// 保存产品
	savedCount, err := syncService.SaveProducts(ctx, productDataList)
	if err != nil {
		return 0, fmt.Errorf("保存产品失败: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id":   store.TenantID,
		"store_id":    store.ID,
		"saved_count": savedCount,
		"total_count": len(productDataList),
	}).Info("店铺产品同步完成")

	return savedCount, nil
}

// getActiveStores 获取活跃的TEMU店铺列表
func (s *SyncScheduler) getActiveStores(ctx context.Context) ([]*StoreInfo, error) {
	// TODO: 实现获取活跃TEMU店铺的逻辑
	// 这里应该调用管理系统API获取启用了产品同步的TEMU店铺

	// 临时返回示例数据
	return []*StoreInfo{
		{
			ID:       1,
			TenantID: 1,
			Platform: "TEMU",
			Status:   "active",
		},
	}, nil
}

// StoreInfo 店铺信息
type StoreInfo struct {
	ID       int64  `json:"id"`
	TenantID int64  `json:"tenant_id"`
	Platform string `json:"platform"`
	Status   string `json:"status"`
}
