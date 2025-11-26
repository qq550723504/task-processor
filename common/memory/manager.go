package memory

import (
	"task-processor/common/management"

	"github.com/sirupsen/logrus"
)

// MemoryManager 内存管理器（统一管理所有内存存储）
type MemoryManager struct {
	CookieManager     *CookieManager
	ShopPauseManager  *ShopPauseManager
	DailyCountManager *DailyCountManager
	ReListingQueue    *ReListingQueueManager
}

// NewMemoryManager 创建内存管理器
func NewMemoryManager(managementClientMgr *management.ClientManager) *MemoryManager {
	logrus.Info("初始化内存管理器...")

	manager := &MemoryManager{
		CookieManager:     NewCookieManager(),
		ShopPauseManager:  NewShopPauseManager(),
		DailyCountManager: NewDailyCountManager(managementClientMgr),
		ReListingQueue:    NewReListingQueueManager(),
	}

	// 启动清理任务
	manager.ShopPauseManager.StartCleanupTask()

	logrus.Info("内存管理器初始化完成")
	return manager
}

// GetStats 获取统计信息
func (m *MemoryManager) GetStats() map[string]interface{} {
	return map[string]any{
		"cookies_count":        len(m.CookieManager.GetAllCookies()),
		"paused_shops_count":   len(m.ShopPauseManager.pauses),
		"relisting_queue_keys": len(m.ReListingQueue.GetAllKeys()),
	}
}
