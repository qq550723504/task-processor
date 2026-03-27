package state

import (
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/clients/management/api"

)

// DailyCountInfo 每日计数信息
type DailyCountInfo struct {
	Date  string
	Count int64
}

// DailyCountManager 每日上架计数管理器（API版）
type DailyCountManager struct {
	managementClientMgr *management.ClientManager
}

// NewDailyCountManager 创建每日计数管理器
func NewDailyCountManager(managementClientMgr *management.ClientManager) *DailyCountManager {
	return &DailyCountManager{
		managementClientMgr: managementClientMgr,
	}
}

// IncrementCount 增加计数
func (m *DailyCountManager) IncrementCount(tenantID, shopID int64, date string, increment int64) int64 {
	client := m.managementClientMgr.GetDailyListingCountClient()
	if client == nil {
		logger.GetGlobalLogger("app/state").Warn("每日上架数量客户端未初始化，返回默认值")
		return increment
	}

	// 先获取当前计数
	currentCount := m.GetCount(tenantID, shopID, date)
	newCount := currentCount + increment

	// 设置新的计数
	req := &api.DailyListingCountSetReqDTO{
		TenantID: tenantID,
		StoreID:  shopID,
		UserID:   tenantID, // 使用tenantID作为userID
		Date:     date,
		Count:    newCount,
	}

	if err := client.SetDailyListingCount(req); err != nil {
		logger.GetGlobalLogger("app/state").Errorf("设置每日上架数量失败: tenantID=%d, shopID=%d, date=%s, count=%d, error=%v",
			tenantID, shopID, date, newCount, err)
		return currentCount // 返回原来的计数
	}

	logger.GetGlobalLogger("app/state").Infof("成功增加每日上架数量: tenantID=%d, shopID=%d, date=%s, increment=%d, newCount=%d",
		tenantID, shopID, date, increment, newCount)
	return newCount
}

// GetCount 获取计数
func (m *DailyCountManager) GetCount(tenantID, shopID int64, date string) int64 {
	client := m.managementClientMgr.GetDailyListingCountClient()
	if client == nil {
		logger.GetGlobalLogger("app/state").Warn("每日上架数量客户端未初始化，返回默认值0")
		return 0
	}

	resp, err := client.GetDailyListingCount(tenantID, shopID, tenantID, date)
	if err != nil {
		logger.GetGlobalLogger("app/state").Errorf("获取每日上架数量失败: tenantID=%d, shopID=%d, date=%s, error=%v",
			tenantID, shopID, date, err)
		return 0
	}

	if resp == nil {
		logger.GetGlobalLogger("app/state").Warnf("获取每日上架数量返回空结果: tenantID=%d, shopID=%d, date=%s",
			tenantID, shopID, date)
		return 0
	}

	return resp.Count
}

// ResetCount 重置计数
func (m *DailyCountManager) ResetCount(tenantID, shopID int64, date string) {
	client := m.managementClientMgr.GetDailyListingCountClient()
	if client == nil {
		logger.GetGlobalLogger("app/state").Warn("每日上架数量客户端未初始化，无法重置计数")
		return
	}

	req := &api.DailyListingCountSetReqDTO{
		TenantID: tenantID,
		StoreID:  shopID,
		UserID:   tenantID, // 使用tenantID作为userID
		Date:     date,
		Count:    0,
	}

	if err := client.SetDailyListingCount(req); err != nil {
		logger.GetGlobalLogger("app/state").Errorf("重置每日上架数量失败: tenantID=%d, shopID=%d, date=%s, error=%v",
			tenantID, shopID, date, err)
		return
	}

	logger.GetGlobalLogger("app/state").Infof("每日计数已重置: 租户=%d, 店铺=%d, 日期=%s", tenantID, shopID, date)
}

// GetClient 获取每日上架数量客户端
func (m *DailyCountManager) GetClient() api.DailyListingCountAPI {
	if m.managementClientMgr == nil {
		return nil
	}
	return m.managementClientMgr.GetDailyListingCountClient()
}
