package client

import (
	"fmt"
	"sync"

	"task-processor/internal/common/management"

	"github.com/sirupsen/logrus"
)

// APIClientManager API客户端管理器
type APIClientManager struct {
	clients          map[string]*APIClient
	managementClient *management.ClientManager
	mutex            sync.RWMutex
	logger           *logrus.Entry
}

// NewAPIClientManager 创建新的API客户端管理器
func NewAPIClientManager(managementClient *management.ClientManager) *APIClientManager {
	return &APIClientManager{
		clients:          make(map[string]*APIClient),
		managementClient: managementClient,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TEMUAPIClientManager",
		}),
	}
}

// GetClient 获取或创建API客户端
func (m *APIClientManager) GetClient(tenantID, storeID int64) (*APIClient, error) {
	key := fmt.Sprintf("%d:%d", tenantID, storeID)

	// 先尝试读锁获取已存在的客户端
	m.mutex.RLock()
	client, exists := m.clients[key]
	m.mutex.RUnlock()

	if exists {
		return client, nil
	}

	// 双重检查锁模式：再次检查以避免重复创建
	m.mutex.Lock()
	defer m.mutex.Unlock()

	client, exists = m.clients[key]
	if exists {
		return client, nil
	}

	// 创建新的客户端
	client = NewAPIClient(tenantID, storeID, m.managementClient)
	m.clients[key] = client

	m.logger.Infof("成功创建并缓存API客户端: 租户=%d, 店铺=%d", tenantID, storeID)
	return client, nil
}

// RemoveClient 删除指定店铺的客户端缓存
func (m *APIClientManager) RemoveClient(tenantID, storeID int64) {
	key := fmt.Sprintf("%d:%d", tenantID, storeID)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.clients[key]; exists {
		delete(m.clients, key)
		m.logger.Infof("已删除API客户端缓存: 租户=%d, 店铺=%d", tenantID, storeID)
	}
}

// GetAllClients 获取所有已创建的客户端信息
func (m *APIClientManager) GetAllClients() map[string]*APIClient {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 创建一个副本以避免外部修改
	clientsCopy := make(map[string]*APIClient, len(m.clients))
	for key, client := range m.clients {
		clientsCopy[key] = client
	}

	return clientsCopy
}

// GetTenantShopPairs 从客户端键中提取所有租户和店铺对
func (m *APIClientManager) GetTenantShopPairs() []struct {
	TenantID int64
	StoreID  int64
} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	m.logger.Infof("APIClientManager中当前有 %d 个客户端", len(m.clients))

	var pairs []struct {
		TenantID int64
		StoreID  int64
	}

	for key := range m.clients {
		// 解析键格式: {tenantID}:{storeID}
		var tenantID, storeID int64
		if n, err := fmt.Sscanf(key, "%d:%d", &tenantID, &storeID); err == nil && n == 2 {
			pairs = append(pairs, struct {
				TenantID int64
				StoreID  int64
			}{TenantID: tenantID, StoreID: storeID})
			m.logger.Debugf("解析到租户店铺对: 租户=%d, 店铺=%d", tenantID, storeID)
		} else {
			m.logger.Warnf("无法解析客户端键: %s", key)
		}
	}

	m.logger.Infof("获取到 %d 个租户店铺对", len(pairs))
	return pairs
}
