package temu

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/common/management"

	"github.com/sirupsen/logrus"
)

// SchedulerManager 调度器管理器
type SchedulerManager struct {
	schedulers       map[string]*PricingScheduler
	apiClients       map[string]*APIClient
	managementClient *management.ClientManager
	interval         time.Duration
	action           PricingAction
	mutex            sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	logger           *logrus.Entry
}

// NewSchedulerManager 创建调度器管理器
func NewSchedulerManager(managementClient *management.ClientManager, interval time.Duration, action PricingAction) *SchedulerManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &SchedulerManager{
		schedulers:       make(map[string]*PricingScheduler),
		apiClients:       make(map[string]*APIClient),
		managementClient: managementClient,
		interval:         interval,
		action:           action,
		ctx:              ctx,
		cancel:           cancel,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TEMUSchedulerManager",
			"action":    action,
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

	// 创建API客户端
	apiClient := NewAPIClient(tenantID, storeID, sm.managementClient)
	sm.apiClients[key] = apiClient

	// 创建调度器
	scheduler := NewPricingScheduler(apiClient, sm.managementClient, sm.interval, sm.action)
	sm.schedulers[key] = scheduler

	// 启动调度器
	scheduler.Start()

	sm.logger.Infof("成功添加并启动店铺调度器: %s", key)
	return nil
}

// RemoveStore 移除店铺调度器
func (sm *SchedulerManager) RemoveStore(tenantID, storeID int64) {
	key := fmt.Sprintf("%d:%d", tenantID, storeID)

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if scheduler, exists := sm.schedulers[key]; exists {
		scheduler.Stop()
		delete(sm.schedulers, key)
		delete(sm.apiClients, key)
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
	sm.apiClients = make(map[string]*APIClient)
	sm.logger.Info("所有调度器已停止")
}

// GetSchedulerCount 获取调度器数量
func (sm *SchedulerManager) GetSchedulerCount() int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return len(sm.schedulers)
}
