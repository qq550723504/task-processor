// Package scheduler 提供SHEIN平台的调度器管理功能
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/pkg/management"

	"github.com/sirupsen/logrus"
)

// SchedulerManager SHEIN调度器管理器
type SchedulerManager struct {
	schedulers       map[string]*PricingScheduler
	managementClient *management.ClientManager
	cookiePrewarmer  *CookiePrewarmer
	interval         time.Duration
	mutex            sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	logger           *logrus.Entry
}

// NewSchedulerManager 创建调度器管理器
func NewSchedulerManager(ctx context.Context, managementClient *management.ClientManager, interval time.Duration) *SchedulerManager {
	managerCtx, cancel := context.WithCancel(ctx)

	return &SchedulerManager{
		schedulers:       make(map[string]*PricingScheduler),
		managementClient: managementClient,
		cookiePrewarmer:  NewCookiePrewarmer(managementClient),
		interval:         interval,
		ctx:              managerCtx,
		cancel:           cancel,
		logger: logrus.WithFields(logrus.Fields{
			"component": "SHEINSchedulerManager",
		}),
	}
}

// AddStore 添加店铺调度器
func (sm *SchedulerManager) AddStore(tenantID, storeID int64) error {
	key := fmt.Sprintf("%d:%d", tenantID, storeID)

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// 检查是否已存在
	if _, exists := sm.schedulers[key]; exists {
		sm.logger.Infof("店铺调度器已存在: %s", key)
		return nil
	}

	// 创建调度器（使用预热器的客户端管理器）
	scheduler := NewPricingSchedulerWithPrewarmer(sm.ctx, tenantID, storeID, sm.managementClient, sm.cookiePrewarmer, sm.interval)
	sm.schedulers[key] = scheduler

	// 启动调度器
	scheduler.Start()

	sm.logger.Infof("成功添加并启动店铺调度器: %s", key)
	return nil
}

// PrewarmCookies 预热指定店铺的Cookie
func (sm *SchedulerManager) PrewarmCookies(storeIDs []int64) []PrewarmResult {
	sm.logger.Infof("开始批量预热 %d 个店铺的Cookie", len(storeIDs))

	// 使用预热器预热Cookie
	results := sm.cookiePrewarmer.PrewarmCookies(sm.ctx, storeIDs)

	// 统计结果
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			sm.logger.Warnf("店铺 %s (ID: %d) Cookie预热失败: %s",
				result.StoreName, result.StoreID, result.ErrorMessage)
		}
	}

	sm.logger.Infof("Cookie批量预热完成 - 成功: %d, 失败: %d",
		successCount, len(results)-successCount)

	return results
}

// RemoveStore 移除店铺调度器
func (sm *SchedulerManager) RemoveStore(tenantID, storeID int64) {
	key := fmt.Sprintf("%d:%d", tenantID, storeID)

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if scheduler, exists := sm.schedulers[key]; exists {
		scheduler.Stop()
		delete(sm.schedulers, key)
		sm.logger.Infof("成功移除店铺调度器: %s", key)
	}
}

// StopAll 停止所有调度器
func (sm *SchedulerManager) StopAll() {
	sm.logger.Info("停止所有调度器")
	sm.cancel()

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	for key, scheduler := range sm.schedulers {
		scheduler.Stop()
		sm.logger.Infof("已停止调度器: %s", key)
	}

	sm.schedulers = make(map[string]*PricingScheduler)
	sm.logger.Info("所有调度器已停止")
}

// GetSchedulerCount 获取调度器数量
func (sm *SchedulerManager) GetSchedulerCount() int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return len(sm.schedulers)
}
