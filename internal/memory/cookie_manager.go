// Package memory 提供内存缓存管理功能
package memory

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// CookieInfo Cookie信息
type CookieInfo struct {
	Cookie     string
	UpdateTime time.Time
}

// CookieManager Cookie管理器（内存版）
type CookieManager struct {
	cookies map[string]*CookieInfo // key: tenantID:shopID
	mutex   sync.RWMutex
}

// NewCookieManager 创建Cookie管理器实例
func NewCookieManager() *CookieManager {
	return &CookieManager{
		cookies: make(map[string]*CookieInfo),
	}
}

// SetCookie 设置指定租户和店铺的Cookie信息
func (m *CookieManager) SetCookie(tenantID, shopID int64, cookie string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := fmt.Sprintf("%d:%d", tenantID, shopID)
	m.cookies[key] = &CookieInfo{
		Cookie:     cookie,
		UpdateTime: time.Now(),
	}

	logrus.Infof("Cookie已保存到内存: 租户=%d, 店铺=%d", tenantID, shopID)
}

// GetCookie 获取指定租户和店铺的Cookie信息
// 如果Cookie不存在，返回错误
func (m *CookieManager) GetCookie(tenantID, shopID int64) (string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	key := fmt.Sprintf("%d:%d", tenantID, shopID)
	info, exists := m.cookies[key]
	if !exists {
		return "", fmt.Errorf("Cookie不存在: 租户=%d, 店铺=%d", tenantID, shopID)
	}

	return info.Cookie, nil
}

// DeleteCookie 删除指定租户和店铺的Cookie信息
func (m *CookieManager) DeleteCookie(tenantID, shopID int64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := fmt.Sprintf("%d:%d", tenantID, shopID)
	delete(m.cookies, key)

	logrus.Infof("Cookie已从内存删除: 租户=%d, 店铺=%d", tenantID, shopID)
}

// GetAllCookies 获取所有Cookie信息（用于调试和监控）
func (m *CookieManager) GetAllCookies() map[string]*CookieInfo {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*CookieInfo)
	for k, v := range m.cookies {
		result[k] = v
	}
	return result
}
