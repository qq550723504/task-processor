package state

import (
	"task-processor/internal/core/logger"
	"context"
	"fmt"
	"sync"
	"time"

)

// ShopPauseInfo 店铺暂停信息
type ShopPauseInfo struct {
	Reason    string
	PausedAt  time.Time
	ResumeAt  time.Time
	IsPaused  bool
	PauseType string // 暂停类型: "auth_expired"(认证过期) 或 "quota_limit"(配额限制)
}

// ShopPauseManager 店铺暂停管理器
type ShopPauseManager struct {
	pauses      map[string]*ShopPauseInfo // key: tenantID:shopID
	mutex       sync.RWMutex
	storeClient StoreClient // 用于调用API
}

// StoreClient 店铺API客户端接口
type StoreClient interface {
	SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error)
	GetStorePauseStatus(id int64) (bool, error)
}

// NewShopPauseManager 创建店铺暂停管理器
func NewShopPauseManager() *ShopPauseManager {
	return &ShopPauseManager{
		pauses: make(map[string]*ShopPauseInfo),
	}
}

// SetStoreClient 设置店铺API客户端
func (m *ShopPauseManager) SetStoreClient(client StoreClient) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.storeClient = client
}

// PauseShop 暂停店铺（指定时长）
func (m *ShopPauseManager) PauseShop(tenantID, shopID int64, reason string, duration time.Duration) {
	now := time.Now()
	resumeAt := now.Add(duration)
	m.pauseShopInternal(tenantID, shopID, reason, resumeAt, "quota_limit")
}

// PauseShopForDuration 暂停店铺指定时长（如10分钟）
func (m *ShopPauseManager) PauseShopForDuration(tenantID, shopID int64, reason string, duration time.Duration) {
	now := time.Now()
	resumeAt := now.Add(duration)
	m.pauseShopInternal(tenantID, shopID, reason, resumeAt, "auth_expired")
	logger.GetGlobalLogger("app/state").Infof("店铺暂停 %v: 租户=%d, 店铺=%d, 原因=%s", duration, tenantID, shopID, reason)
}

// PauseShopUntilEndOfDay 暂停店铺到当日结束（23:59:59）
func (m *ShopPauseManager) PauseShopUntilEndOfDay(tenantID, shopID int64, reason string) {
	now := time.Now()
	// 计算当日的23:59:59
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())

	// 如果距离当天结束不足1分钟，不设置暂停（等待新的一天重置配额）
	if endOfDay.Sub(now) < time.Minute {
		logger.GetGlobalLogger("app/state").Infof("距离当天结束不足1分钟，跳过暂停设置，等待新的一天重置配额: 租户=%d, 店铺=%d", tenantID, shopID)
		return
	}

	m.pauseShopInternal(tenantID, shopID, reason, endOfDay, "quota_limit")
	logger.GetGlobalLogger("app/state").Infof("店铺暂停到当日结束: 租户=%d, 店铺=%d, 原因=%s, 恢复时间=%s", tenantID, shopID, reason, endOfDay.Format("2006-01-02 15:04:05"))
}

// PauseShopForAuthExpired 因认证过期暂停店铺（需要重新登录）
func (m *ShopPauseManager) PauseShopForAuthExpired(tenantID, shopID int64, reason string) {
	// 认证过期暂停，设置较长的恢复时间（24小时），实际会在登录成功后删除
	now := time.Now()
	resumeAt := now.Add(24 * time.Hour)
	m.pauseShopInternal(tenantID, shopID, reason, resumeAt, "auth_expired")
	logger.GetGlobalLogger("app/state").Infof("店铺因认证过期暂停: 租户=%d, 店铺=%d, 原因=%s", tenantID, shopID, reason)
}

// pauseShopInternal 内部暂停店铺方法
func (m *ShopPauseManager) pauseShopInternal(tenantID, shopID int64, reason string, resumeAt time.Time, pauseType string) {
	// 先更新本地状态
	m.mutex.Lock()
	key := fmt.Sprintf("%d:%d", tenantID, shopID)
	now := time.Now()

	m.pauses[key] = &ShopPauseInfo{
		Reason:    reason,
		PausedAt:  now,
		ResumeAt:  resumeAt,
		IsPaused:  true,
		PauseType: pauseType,
	}
	m.mutex.Unlock()

	// 释放锁后再调用API，避免长时间持有锁
	// 暂停键的值格式: {"type":"auth_expired|quota_limit","reason":"原因","timestamp":1234567890}
	if m.storeClient != nil {
		logger.GetGlobalLogger("app/state").Infof("正在设置店铺 %d 的暂停状态，类型=%s，恢复时间: %s", shopID, pauseType, resumeAt.Format("2006-01-02 15:04:05"))
		success, err := m.storeClient.SetStorePauseStatus(shopID, true, pauseType)
		if err != nil {
			logger.GetGlobalLogger("app/state").Errorf("设置店铺 %d 的暂停状态失败: %v", shopID, err)
			// TODO: 考虑添加重试机制或标记为待同步状态
		} else if success {
			logger.GetGlobalLogger("app/state").Infof("成功设置店铺 %d 的暂停状态 (类型: %s)", shopID, pauseType)
		} else {
			logger.GetGlobalLogger("app/state").Warnf("设置店铺 %d 的暂停状态返回失败", shopID)
		}
	}
}

// ResumeShop 恢复店铺
func (m *ShopPauseManager) ResumeShop(tenantID, shopID int64) {
	// 先更新本地状态
	m.mutex.Lock()
	key := fmt.Sprintf("%d:%d", tenantID, shopID)
	delete(m.pauses, key)
	m.mutex.Unlock()

	// 释放锁后再调用API
	if m.storeClient != nil {
		logger.GetGlobalLogger("app/state").Infof("正在恢复店铺 %d 的暂停状态...", shopID)
		success, err := m.storeClient.SetStorePauseStatus(shopID, false, "")
		if err != nil {
			logger.GetGlobalLogger("app/state").Errorf("恢复店铺 %d 的暂停状态失败: %v", shopID, err)
		} else if success {
			logger.GetGlobalLogger("app/state").Infof("成功恢复店铺 %d 的暂停状态", shopID)
		} else {
			logger.GetGlobalLogger("app/state").Warnf("恢复店铺 %d 的暂停状态返回失败", shopID)
		}
	}

	logger.GetGlobalLogger("app/state").Infof("店铺已恢复: 租户=%d, 店铺=%d", tenantID, shopID)
}

// IsShopPaused 检查店铺是否暂停
func (m *ShopPauseManager) IsShopPaused(tenantID, shopID int64) bool {
	// 优先从后台接口获取状态
	if m.storeClient != nil {
		isPaused, err := m.storeClient.GetStorePauseStatus(shopID)
		if err != nil {
			logger.GetGlobalLogger("app/state").Errorf("获取店铺 %d 的暂停状态失败: %v", shopID, err)
			// 如果接口调用失败，降级到本地内存查询
		} else {
			return isPaused
		}
	}

	// 降级方案：从本地内存获取
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	key := fmt.Sprintf("%d:%d", tenantID, shopID)
	info, exists := m.pauses[key]
	if !exists {
		return false
	}

	// 检查是否已过期
	if time.Now().After(info.ResumeAt) {
		// 已过期，需要清理（但这里不能修改，返回false）
		return false
	}

	return info.IsPaused
}

// GetPauseInfo 获取暂停信息（会检查是否过期）
func (m *ShopPauseManager) GetPauseInfo(tenantID, shopID int64) (*ShopPauseInfo, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	key := fmt.Sprintf("%d:%d", tenantID, shopID)
	info, exists := m.pauses[key]
	if !exists {
		return nil, false
	}

	// 检查是否已过期
	if time.Now().After(info.ResumeAt) {
		return nil, false
	}

	return info, true
}

// CleanupExpired 清理过期的暂停记录
func (m *ShopPauseManager) CleanupExpired() {
	// 先收集需要清理的记录
	m.mutex.Lock()
	now := time.Now()
	expiredShops := make([]struct {
		key      string
		tenantID int64
		shopID   int64
	}, 0)

	for key, info := range m.pauses {
		// 只清理过期的 quota_limit 类型暂停
		// auth_expired 类型需要等待登录成功后才能删除，不应该自动清理
		if now.After(info.ResumeAt) && info.PauseType == "quota_limit" {
			var tenantID, shopID int64
			if _, err := fmt.Sscanf(key, "%d:%d", &tenantID, &shopID); err == nil {
				expiredShops = append(expiredShops, struct {
					key      string
					tenantID int64
					shopID   int64
				}{key, tenantID, shopID})
			}
			delete(m.pauses, key)
			logger.GetGlobalLogger("app/state").Infof("清理过期的配额限制暂停记录: %s (类型: %s)", key, info.PauseType)
		} else if now.After(info.ResumeAt) && info.PauseType == "auth_expired" {
			// 认证过期类型的暂停不自动清理，记录日志提醒
			logger.GetGlobalLogger("app/state").Debugf("跳过认证过期类型的暂停记录清理: %s (需要等待登录成功)", key)
		}
	}
	m.mutex.Unlock()

	// 释放锁后再调用API
	if m.storeClient != nil {
		for _, shop := range expiredShops {
			logger.GetGlobalLogger("app/state").Infof("正在恢复店铺 %d 的暂停状态...", shop.shopID)
			success, err := m.storeClient.SetStorePauseStatus(shop.shopID, false, "")
			if err != nil {
				logger.GetGlobalLogger("app/state").Errorf("恢复店铺 %d 的暂停状态失败: %v", shop.shopID, err)
			} else if success {
				logger.GetGlobalLogger("app/state").Infof("成功恢复店铺 %d 的暂停状态", shop.shopID)
			} else {
				logger.GetGlobalLogger("app/state").Warnf("恢复店铺 %d 的暂停状态返回失败", shop.shopID)
			}
		}
	}
}

// StartCleanupTask 启动定期清理任务
func (m *ShopPauseManager) StartCleanupTask(ctx context.Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.GetGlobalLogger("app/state").Errorf("店铺暂停清理任务goroutine panic: %v", r)
			}
		}()

		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.GetGlobalLogger("app/state").Info("店铺暂停清理任务停止")
				return
			case <-ticker.C:
				m.CleanupExpired()
			}
		}
	}()
}
