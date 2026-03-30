// Package browser 提供浏览器实例管理功能
package browser

import (
	"fmt"
	"sync"
	"task-processor/internal/core/logger"
	sharedbrowser "task-processor/internal/crawler/shared/browser"
	"time"
)

// InstanceManager 实例管理器
type InstanceManager struct {
	pool          *BrowserPool
	rebuildingMu  sync.Mutex
	rebuildingIDs map[int]bool // 正在重建中的实例ID，防止重复重建
}

// NewInstanceManager 创建实例管理器
func NewInstanceManager(pool *BrowserPool) *InstanceManager {
	return &InstanceManager{
		pool:          pool,
		rebuildingIDs: make(map[int]bool),
	}
}

// CreateInstance 创建浏览器实例
func (im *InstanceManager) CreateInstance(id int) (*BrowserInstance, error) {
	// 根据池配置决定使用的策略
	strategy := im.pool.poolConfig.FingerprintStrategy
	presetName := im.pool.poolConfig.PresetName
	cfg := im.pool.GetConfig()
	if cfg == nil {
		return nil, fmt.Errorf("浏览器池配置为空")
	}

	selectedProxy := im.pool.AcquireProxy(id)
	cfgCopy := *cfg
	cfgCopy.Browser = cfg.Browser
	if selectedProxy != "" {
		cfgCopy.Browser.ProxyServer = selectedProxy
	}

	// 使用新的配置管理器创建管理器
	manager := NewBrowserManagerWithConfig(&cfgCopy, strategy, presetName, id)

	// 如果启用随机指纹，为每个实例生成指纹
	if im.pool.UseRandomFingerprint() && im.pool.GetFingerprintGenerator() != nil {
		var fingerprint *sharedbrowser.FingerprintConfig

		// 根据策略决定指纹生成方式
		switch strategy {
		case "random":
			fingerprint = im.pool.GetFingerprintGenerator().GenerateRandomFingerprint("")
			logger.GetGlobalLogger("crawler/amazon").Infof("实例 %d 使用随机指纹", id)
		case "stable":
			userID := fmt.Sprintf("instance_%d", id)
			fingerprint = im.pool.GetFingerprintGenerator().GenerateStableFingerprint(userID)
			logger.GetGlobalLogger("crawler/amazon").Infof("实例 %d 使用稳定指纹 (用户ID: %s)", id, userID)
		default:
			// 默认使用随机指纹
			fingerprint = im.pool.GetFingerprintGenerator().GenerateRandomFingerprint("")
			logger.GetGlobalLogger("crawler/amazon").Infof("实例 %d 使用默认随机指纹", id)
		}

		manager.SetFingerprint(fingerprint)

	}

	if err := manager.Install(); err != nil {
		return nil, fmt.Errorf("初始化playwright失败: %w", err)
	}

	if err := manager.Launch(); err != nil {
		return nil, fmt.Errorf("启动浏览器失败: %w", err)
	}

	page, err := manager.NewPage()
	if err != nil {
		manager.Close()
		return nil, fmt.Errorf("创建页面失败: %w", err)
	}

	return &BrowserInstance{
		ID:           id,
		Manager:      manager,
		Page:         page,
		InUse:        false,
		CurrentProxy: selectedProxy,
	}, nil
}

// closeManagerWithTimeout 关闭 BrowserManager，最多等待 timeout，超时后强制继续。
// 防止 WebSocket 断连时 Close() 永久 hang 导致重建 goroutine 卡死。
func closeManagerWithTimeout(manager interface{ Close() }, instanceID int, timeout time.Duration) {
	log := logger.GetGlobalLogger("crawler/amazon")
	done := make(chan struct{}, 1)
	go func() {
		manager.Close()
		close(done)
	}()
	select {
	case <-done:
		log.Infof("已关闭浏览器实例 %d", instanceID)
	case <-time.After(timeout):
		log.Warnf("关闭浏览器实例 %d 超时（%v），强制继续", instanceID, timeout)
	}
}

// RecreateInstanceSync 同步重新创建浏览器实例（用于任务内重试）
func (im *InstanceManager) RecreateInstanceSync(oldInstance *BrowserInstance) *BrowserInstance {
	logger.GetGlobalLogger("crawler/amazon").Infof("开始同步重新创建浏览器实例 %d", oldInstance.ID)

	// 先关闭旧实例（加超时保护，防止 WebSocket 断连时 hang）
	if oldInstance.Manager != nil {
		closeManagerWithTimeout(oldInstance.Manager, oldInstance.ID, 10*time.Second)
	}

	// 等待短暂时间，让资源释放
	time.Sleep(2 * time.Second)

	// 创建新实例
	newInstance, err := im.CreateInstance(oldInstance.ID)
	if err != nil {
		logger.GetGlobalLogger("crawler/amazon").Errorf("同步重新创建浏览器实例 %d 失败: %v", oldInstance.ID, err)

		// 如果创建失败，等待更长时间后再次尝试
		logger.GetGlobalLogger("crawler/amazon").Infof("等待5秒后进行第二次创建尝试...")
		time.Sleep(5 * time.Second)

		newInstance, err = im.CreateInstance(oldInstance.ID)
		if err != nil {
			logger.GetGlobalLogger("crawler/amazon").Errorf("第二次同步重新创建浏览器实例 %d 失败: %v", oldInstance.ID, err)
			// 返回nil，让调用方处理
			return nil
		}
	}

	// 更新实例列表
	im.pool.UpdateInstance(oldInstance, newInstance)

	logger.GetGlobalLogger("crawler/amazon").Infof("✅ 成功同步重新创建浏览器实例 %d", oldInstance.ID)
	return newInstance
}

// RecreateInstanceAsync 异步重新创建浏览器实例（用于后台健康检查）
func (im *InstanceManager) RecreateInstanceAsync(oldInstance *BrowserInstance) {
	log := logger.GetGlobalLogger("crawler/amazon")

	// 防止对同一实例重复触发重建
	im.rebuildingMu.Lock()
	if im.rebuildingIDs[oldInstance.ID] {
		im.rebuildingMu.Unlock()
		log.Warnf("浏览器实例 %d 已在重建中，跳过重复触发", oldInstance.ID)
		return
	}
	im.rebuildingIDs[oldInstance.ID] = true
	im.rebuildingMu.Unlock()

	log.Infof("开始异步重新创建浏览器实例 %d", oldInstance.ID)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("重新创建浏览器实例goroutine panic (实例ID: %d): %v", oldInstance.ID, r)
			}
			im.rebuildingMu.Lock()
			delete(im.rebuildingIDs, oldInstance.ID)
			im.rebuildingMu.Unlock()
		}()

		// 先关闭旧实例（加超时保护，防止 WebSocket 断连时 hang）
		if oldInstance.Manager != nil {
			closeManagerWithTimeout(oldInstance.Manager, oldInstance.ID, 10*time.Second)
		}

		// 带退避的重建循环，最多尝试5次，确保实例不会永久丢失
		delays := []time.Duration{2, 10, 30, 60, 120}
		var newInstance *BrowserInstance
		var err error
		for attempt, delay := range delays {
			log.Infof("[重建] 实例 %d 第 %d 次尝试，等待 %ds...", oldInstance.ID, attempt+1, delay)
			time.Sleep(delay * time.Second)
			newInstance, err = im.CreateInstance(oldInstance.ID)
			if err == nil {
				log.Infof("[重建] 实例 %d 第 %d 次创建成功", oldInstance.ID, attempt+1)
				break
			}
			log.Warnf("第 %d 次重建浏览器实例 %d 失败: %v", attempt+1, oldInstance.ID, err)
		}

		if newInstance == nil {
			log.Errorf("浏览器实例 %d 所有重建尝试均失败，池将永久缩容，请检查浏览器环境", oldInstance.ID)
			return
		}

		// 更新实例列表
		im.pool.UpdateInstance(oldInstance, newInstance)

		// 将新实例放回可用池
		ch := im.pool.GetAvailableChannel()
		if ch == nil {
			log.Warnf("浏览器池已关闭，关闭重建的实例 %d", newInstance.ID)
			newInstance.Manager.Close()
			return
		}
		chLen := len(ch)
		chCap := cap(ch)
		log.Infof("[重建] 准备放回实例 %d，当前通道: len=%d, cap=%d", newInstance.ID, chLen, chCap)
		select {
		case ch <- newInstance:
			log.Infof("✅ 成功异步重新创建浏览器实例 %d，通道: len=%d→%d", newInstance.ID, chLen, len(ch))
		default:
			// 通道满说明池已经有足够实例，不需要这个多余的实例
			log.Warnf("[重建] 浏览器池通道已满(len=%d/cap=%d)，关闭多余的重建实例 %d", chLen, chCap, newInstance.ID)
			newInstance.Manager.Close()
		}
	}()
}

// CloseInstance 关闭浏览器实例（加超时保护，防止 WebSocket 断连时 hang）
func (im *InstanceManager) CloseInstance(instance *BrowserInstance) {
	if instance == nil {
		return
	}

	if instance.Manager != nil {
		closeManagerWithTimeout(instance.Manager, instance.ID, 10*time.Second)
	}
}

// ValidateInstance 验证浏览器实例
func (im *InstanceManager) ValidateInstance(instance *BrowserInstance) bool {
	if instance == nil {
		return false
	}

	if instance.Manager == nil {
		logger.GetGlobalLogger("crawler/amazon").Infof("浏览器实例 %d 管理器为空", instance.ID)
		return false
	}

	if instance.Page == nil {
		logger.GetGlobalLogger("crawler/amazon").Infof("浏览器实例 %d 页面为空", instance.ID)
		return false
	}

	return true
}

// GetInstanceInfo 获取实例信息
func (im *InstanceManager) GetInstanceInfo(instance *BrowserInstance) map[string]any {
	if instance == nil {
		return map[string]any{
			"valid": false,
			"error": "instance is nil",
		}
	}

	info := map[string]any{
		"id":              instance.ID,
		"in_use":          instance.InUse,
		"current_zipcode": instance.CurrentZipcode,
		"valid":           im.ValidateInstance(instance),
	}

	if instance.Manager != nil {
		info["manager_exists"] = true
	} else {
		info["manager_exists"] = false
	}

	if instance.Page != nil {
		info["page_exists"] = true
	} else {
		info["page_exists"] = false
	}

	return info
}

// RestartInstance 重启浏览器实例
func (im *InstanceManager) RestartInstance(instance *BrowserInstance) (*BrowserInstance, error) {
	if instance == nil {
		return nil, fmt.Errorf("实例为空")
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("重启浏览器实例 %d", instance.ID)

	// 关闭旧实例
	im.CloseInstance(instance)

	// 等待资源释放
	time.Sleep(1 * time.Second)

	// 创建新实例
	newInstance, err := im.CreateInstance(instance.ID)
	if err != nil {
		return nil, fmt.Errorf("重启实例失败: %w", err)
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("✅ 成功重启浏览器实例 %d", instance.ID)
	return newInstance, nil
}
