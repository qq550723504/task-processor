package state

import (
	"fmt"
	"sync"

	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management"
	api "task-processor/internal/listingadmin"
)

type DailyCountClientProvider interface {
	GetDailyListingCountClient() *management.DailyListingCountAPIClient
}

// DailyCountInfo 每日计数信息
type DailyCountInfo struct {
	Date  string
	Count int64
}

// DailyQuotaReservation 原子额度预占结果
type DailyQuotaReservation struct {
	Allowed      bool
	NewCount     int64
	Remaining    int64
	ReachedLimit bool
}

// DailyCountManager 每日上架计数管理器（API版）
type DailyCountManager struct {
	clientProvider DailyCountClientProvider
	locksMu        sync.Mutex
	locks          map[string]*sync.Mutex
}

// NewDailyCountManager 创建每日计数管理器
func NewDailyCountManager(clientProvider DailyCountClientProvider) *DailyCountManager {
	return &DailyCountManager{
		clientProvider: clientProvider,
		locks:          make(map[string]*sync.Mutex),
	}
}

// IncrementCount 增加计数
func (m *DailyCountManager) IncrementCount(tenantID, shopID int64, date string, increment int64) int64 {
	lock := m.getLock(tenantID, shopID, date)
	lock.Lock()
	defer lock.Unlock()

	client := m.GetClient()
	if client == nil {
		logger.GetGlobalLogger("state").Warn("每日上架数量客户端未初始化，返回默认值")
		return increment
	}

	currentCount := m.GetCount(tenantID, shopID, date)
	newCount := currentCount + increment

	req := &api.DailyListingCountSetReqDTO{
		TenantID: tenantID,
		StoreID:  shopID,
		UserID:   tenantID,
		Date:     date,
		Count:    newCount,
	}

	if err := client.SetDailyListingCount(req); err != nil {
		logger.GetGlobalLogger("state").Errorf("设置每日上架数量失败: tenantID=%d, shopID=%d, date=%s, count=%d, error=%v",
			tenantID, shopID, date, newCount, err)
		return currentCount
	}

	logger.GetGlobalLogger("state").Infof("成功增加每日上架数量: tenantID=%d, shopID=%d, date=%s, increment=%d, newCount=%d",
		tenantID, shopID, date, increment, newCount)
	return newCount
}

// TryReserveQuota 原子预占每日额度
func (m *DailyCountManager) TryReserveQuota(tenantID, shopID int64, date string, increment, limit int64) (*DailyQuotaReservation, error) {
	client := m.GetClient()
	if client == nil {
		logger.GetGlobalLogger("state").Warn("每日上架数量客户端未初始化，无法预占额度")
		return nil, fmt.Errorf("daily listing count client is not initialized")
	}

	resp, err := client.TryConsumeDailyQuota(&api.TryConsumeDailyQuotaReqDTO{
		TenantID:  tenantID,
		StoreID:   shopID,
		UserID:    tenantID,
		Date:      date,
		Increment: increment,
		Limit:     limit,
	})
	if err != nil {
		logger.GetGlobalLogger("state").Errorf("原子预占每日上架额度失败: tenantID=%d, shopID=%d, date=%s, increment=%d, limit=%d, error=%v",
			tenantID, shopID, date, increment, limit, err)
		return nil, err
	}

	return &DailyQuotaReservation{
		Allowed:      resp.Allowed,
		NewCount:     resp.NewCount,
		Remaining:    resp.Remaining,
		ReachedLimit: resp.ReachedLimit,
	}, nil
}

// RollbackReservedQuota 回滚预占额度
func (m *DailyCountManager) RollbackReservedQuota(tenantID, shopID int64, date string, decrement int64) (int64, error) {
	client := m.GetClient()
	if client == nil {
		logger.GetGlobalLogger("state").Warn("每日上架数量客户端未初始化，无法回滚额度")
		return 0, fmt.Errorf("daily listing count client is not initialized")
	}

	newCount, err := client.RollbackDailyQuota(&api.RollbackDailyQuotaReqDTO{
		TenantID:  tenantID,
		StoreID:   shopID,
		UserID:    tenantID,
		Date:      date,
		Decrement: decrement,
	})
	if err != nil {
		logger.GetGlobalLogger("state").Errorf("回滚每日上架额度失败: tenantID=%d, shopID=%d, date=%s, decrement=%d, error=%v",
			tenantID, shopID, date, decrement, err)
		return 0, err
	}

	logger.GetGlobalLogger("state").Infof("成功回滚每日上架额度: tenantID=%d, shopID=%d, date=%s, decrement=%d, newCount=%d",
		tenantID, shopID, date, decrement, newCount)
	return newCount, nil
}

func (m *DailyCountManager) getLock(tenantID, shopID int64, date string) *sync.Mutex {
	key := fmt.Sprintf("%d:%d:%s", tenantID, shopID, date)

	m.locksMu.Lock()
	defer m.locksMu.Unlock()

	if lock, ok := m.locks[key]; ok {
		return lock
	}

	lock := &sync.Mutex{}
	m.locks[key] = lock
	return lock
}

// GetCount 获取计数
func (m *DailyCountManager) GetCount(tenantID, shopID int64, date string) int64 {
	client := m.GetClient()
	if client == nil {
		logger.GetGlobalLogger("state").Warn("每日上架数量客户端未初始化，返回默认值0")
		return 0
	}

	resp, err := client.GetDailyListingCount(tenantID, shopID, tenantID, date)
	if err != nil {
		logger.GetGlobalLogger("state").Errorf("获取每日上架数量失败: tenantID=%d, shopID=%d, date=%s, error=%v",
			tenantID, shopID, date, err)
		return 0
	}

	if resp == nil {
		logger.GetGlobalLogger("state").Warnf("获取每日上架数量返回空结果: tenantID=%d, shopID=%d, date=%s",
			tenantID, shopID, date)
		return 0
	}

	return resp.Count
}

// ResetCount 重置计数
func (m *DailyCountManager) ResetCount(tenantID, shopID int64, date string) {
	client := m.GetClient()
	if client == nil {
		logger.GetGlobalLogger("state").Warn("每日上架数量客户端未初始化，无法重置计数")
		return
	}

	req := &api.DailyListingCountSetReqDTO{
		TenantID: tenantID,
		StoreID:  shopID,
		UserID:   tenantID,
		Date:     date,
		Count:    0,
	}

	if err := client.SetDailyListingCount(req); err != nil {
		logger.GetGlobalLogger("state").Errorf("重置每日上架数量失败: tenantID=%d, shopID=%d, date=%s, error=%v",
			tenantID, shopID, date, err)
		return
	}

	logger.GetGlobalLogger("state").Infof("每日计数已重置: 租户=%d, 店铺=%d, 日期=%s", tenantID, shopID, date)
}

// GetClient 获取每日上架数量客户端
func (m *DailyCountManager) GetClient() api.DailyListingCountAPI {
	if m.clientProvider == nil {
		return nil
	}
	return m.clientProvider.GetDailyListingCountClient()
}
