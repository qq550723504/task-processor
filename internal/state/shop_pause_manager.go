package state

import (
	"context"
	"fmt"
	"sync"
	"task-processor/internal/core/logger"
	"time"
)

// ShopPauseInfo 店铺暂停信息
type ShopPauseInfo struct {
	Reason    string
	PausedAt  time.Time
	ResumeAt  time.Time
	IsPaused  bool
	PauseType string
}

// ShopPauseManager 店铺暂停管理器
type ShopPauseManager struct {
	pauses               map[string]*ShopPauseInfo
	pendingRemoteResumes map[string]pendingRemoteResume
	mutex                sync.RWMutex
	storeClient          StoreClient
}

type pendingRemoteResume struct {
	tenantID int64
	shopID   int64
}

// StoreClient 店铺API客户端接口
type StoreClient interface {
	SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error)
	GetStorePauseStatus(id int64) (bool, error)
}

// NewShopPauseManager 创建店铺暂停管理器
func NewShopPauseManager() *ShopPauseManager {
	return &ShopPauseManager{
		pauses:               make(map[string]*ShopPauseInfo),
		pendingRemoteResumes: make(map[string]pendingRemoteResume),
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
	logger.GetGlobalLogger("state").Infof("店铺暂停 %v: 租户=%d, 店铺=%d, 原因=%s", duration, tenantID, shopID, reason)
}

// PauseShopUntilEndOfDay 暂停店铺到当日结束（23:59:59）
func (m *ShopPauseManager) PauseShopUntilEndOfDay(tenantID, shopID int64, reason string) {
	now := time.Now()
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())

	if endOfDay.Sub(now) < time.Minute {
		logger.GetGlobalLogger("state").Infof("距离当天结束不足1分钟，跳过暂停设置，等待新的一天重置配额: 租户=%d, 店铺=%d", tenantID, shopID)
		return
	}

	m.pauseShopInternal(tenantID, shopID, reason, endOfDay, "quota_limit")
	logger.GetGlobalLogger("state").Infof("店铺暂停到当日结束: 租户=%d, 店铺=%d, 原因=%s, 恢复时间=%s", tenantID, shopID, reason, endOfDay.Format("2006-01-02 15:04:05"))
}

// PauseShopForAuthExpired 因认证过期暂停店铺（需要重新登录）
func (m *ShopPauseManager) PauseShopForAuthExpired(tenantID, shopID int64, reason string) {
	now := time.Now()
	resumeAt := now.Add(24 * time.Hour)
	m.pauseShopInternal(tenantID, shopID, reason, resumeAt, "auth_expired")
	logger.GetGlobalLogger("state").Infof("店铺因认证过期暂停: 租户=%d, 店铺=%d, 原因=%s", tenantID, shopID, reason)
}

func (m *ShopPauseManager) pauseShopInternal(tenantID, shopID int64, reason string, resumeAt time.Time, pauseType string) {
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

	if m.storeClient != nil {
		logger.GetGlobalLogger("state").Infof("正在设置店铺 %d 的暂停状态，类型=%s，恢复时间: %s", shopID, pauseType, resumeAt.Format("2006-01-02 15:04:05"))
		success, err := m.storeClient.SetStorePauseStatus(shopID, true, pauseType)
		if err != nil {
			logger.GetGlobalLogger("state").Errorf("设置店铺 %d 的暂停状态失败: %v", shopID, err)
		} else if success {
			logger.GetGlobalLogger("state").Infof("成功设置店铺 %d 的暂停状态 (类型: %s)", shopID, pauseType)
		} else {
			logger.GetGlobalLogger("state").Warnf("设置店铺 %d 的暂停状态返回失败", shopID)
		}
	}
}

// ResumeShop 恢复店铺
func (m *ShopPauseManager) ResumeShop(tenantID, shopID int64) {
	m.mutex.Lock()
	key := fmt.Sprintf("%d:%d", tenantID, shopID)
	delete(m.pauses, key)
	m.mutex.Unlock()

	m.resumeRemotePauseStatus(tenantID, shopID)

	logger.GetGlobalLogger("state").Infof("店铺已恢复: 租户=%d, 店铺=%d", tenantID, shopID)
}

// IsShopPaused 检查店铺是否暂停
func (m *ShopPauseManager) IsShopPaused(tenantID, shopID int64) bool {
	if m.storeClient != nil {
		isPaused, err := m.storeClient.GetStorePauseStatus(shopID)
		if err != nil {
			logger.GetGlobalLogger("state").Errorf("获取店铺 %d 的暂停状态失败: %v", shopID, err)
		} else {
			return isPaused
		}
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	key := fmt.Sprintf("%d:%d", tenantID, shopID)
	info, exists := m.pauses[key]
	if !exists {
		return false
	}

	if time.Now().After(info.ResumeAt) {
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

	if time.Now().After(info.ResumeAt) {
		return nil, false
	}

	return info, true
}

// CleanupExpired 清理过期的暂停记录
func (m *ShopPauseManager) CleanupExpired() {
	m.retryPendingRemoteResumes()

	m.mutex.Lock()
	now := time.Now()
	expiredShops := make([]struct {
		key      string
		tenantID int64
		shopID   int64
	}, 0)

	for key, info := range m.pauses {
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
			logger.GetGlobalLogger("state").Infof("清理过期的配额限制暂停记录: %s (类型: %s)", key, info.PauseType)
		} else if now.After(info.ResumeAt) && info.PauseType == "auth_expired" {
			logger.GetGlobalLogger("state").Debugf("跳过认证过期类型的暂停记录清理: %s (需要等待登录成功)", key)
		}
	}
	m.mutex.Unlock()

	for _, shop := range expiredShops {
		m.resumeRemotePauseStatus(shop.tenantID, shop.shopID)
	}
}

// StartCleanupTask 启动定期清理任务
func (m *ShopPauseManager) StartCleanupTask(ctx context.Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.GetGlobalLogger("state").Errorf("店铺暂停清理任务goroutine panic: %v", r)
			}
		}()

		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.GetGlobalLogger("state").Info("店铺暂停清理任务停止")
				return
			case <-ticker.C:
				m.CleanupExpired()
			}
		}
	}()
}

func (m *ShopPauseManager) resumeRemotePauseStatus(tenantID, shopID int64) {
	if m.storeClient == nil {
		return
	}

	logger.GetGlobalLogger("state").Infof("正在恢复店铺 %d 的暂停状态...", shopID)
	success, err := m.storeClient.SetStorePauseStatus(shopID, false, "")
	if err != nil {
		logger.GetGlobalLogger("state").Errorf("恢复店铺 %d 的暂停状态失败: %v", shopID, err)
		m.enqueueRemoteResumeRetry(tenantID, shopID)
		return
	}
	if !success {
		logger.GetGlobalLogger("state").Warnf("恢复店铺 %d 的暂停状态返回失败", shopID)
		m.enqueueRemoteResumeRetry(tenantID, shopID)
		return
	}

	logger.GetGlobalLogger("state").Infof("成功恢复店铺 %d 的暂停状态", shopID)
	m.clearRemoteResumeRetry(tenantID, shopID)
}

func (m *ShopPauseManager) enqueueRemoteResumeRetry(tenantID, shopID int64) {
	key := fmt.Sprintf("%d:%d", tenantID, shopID)

	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.pendingRemoteResumes[key] = pendingRemoteResume{
		tenantID: tenantID,
		shopID:   shopID,
	}
}

func (m *ShopPauseManager) clearRemoteResumeRetry(tenantID, shopID int64) {
	key := fmt.Sprintf("%d:%d", tenantID, shopID)

	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.pendingRemoteResumes, key)
}

func (m *ShopPauseManager) retryPendingRemoteResumes() {
	if m.storeClient == nil {
		return
	}

	m.mutex.RLock()
	pending := make([]pendingRemoteResume, 0, len(m.pendingRemoteResumes))
	for _, item := range m.pendingRemoteResumes {
		pending = append(pending, item)
	}
	m.mutex.RUnlock()

	for _, item := range pending {
		m.resumeRemotePauseStatus(item.tenantID, item.shopID)
	}
}
