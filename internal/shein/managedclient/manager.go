package managedclient

import (
	"fmt"
	"sync"

	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/state"

	managementapi "task-processor/internal/infra/clients/management/api"
	sheinclient "task-processor/internal/shein/client"

	"github.com/sirupsen/logrus"
)

// ClientManager 店铺API客户端管理器
type ClientManager struct {
	clients          map[string]*sheinclient.APIClient
	cookieManager    *state.CookieManager
	managementClient *management.ClientManager
	mutex            sync.RWMutex
	logger           *logrus.Entry
}

// NewClientManager 创建新的客户端管理器
func NewClientManager(cookieManager *state.CookieManager, managementClient *management.ClientManager) *ClientManager {
	return &ClientManager{
		clients:          make(map[string]*sheinclient.APIClient),
		cookieManager:    cookieManager,
		managementClient: managementClient,
		logger:           logger.GetGlobalLogger("SHEINClientManager"),
	}
}

// GetClient 获取或创建店铺API客户端
func (cm *ClientManager) GetClient(shopID int64, storeInfo *managementapi.StoreRespDTO) (*sheinclient.APIClient, error) {
	key := fmt.Sprintf("shein:cookie:%d", shopID)

	cm.mutex.RLock()
	client, exists := cm.clients[key]
	cm.mutex.RUnlock()
	if exists {
		return client, nil
	}

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	client, exists = cm.clients[key]
	if exists {
		return client, nil
	}

	client, err := cm.createClient(shopID, storeInfo)
	if err != nil {
		return nil, err
	}

	cm.clients[key] = client
	cm.logger.Infof("成功创建并缓存店铺API客户端: 店铺=%d", shopID)
	return client, nil
}

// RemoveClient 删除指定店铺的客户端缓存
func (cm *ClientManager) RemoveClient(tenantID, shopID int64) {
	key := fmt.Sprintf("shein:cookie:%d:%d", tenantID, shopID)

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if _, exists := cm.clients[key]; exists {
		delete(cm.clients, key)
		cm.logger.Infof("已删除店铺API客户端缓存: 租户=%d, 店铺=%d", tenantID, shopID)
	}
}

// GetAllClients 获取所有已创建的客户端信息
func (cm *ClientManager) GetAllClients() map[string]*sheinclient.APIClient {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	clientsCopy := make(map[string]*sheinclient.APIClient, len(cm.clients))
	for key, client := range cm.clients {
		clientsCopy[key] = client
	}

	return clientsCopy
}

func (cm *ClientManager) createClient(shopID int64, storeInfo *managementapi.StoreRespDTO) (*sheinclient.APIClient, error) {
	cookieJSON, err := cm.cookieManager.GetCookie(shopID)
	if err != nil {
		cm.logger.Warnf("内存中没有Cookie (店铺=%d)，尝试从管理系统获取", shopID)

		if cm.managementClient != nil {
			if storeClient := cm.managementClient.GetStoreClient(); storeClient != nil {
				if cookieStr, err := storeClient.GetStoreCookie(shopID); err == nil && cookieStr != "" {
					cm.cookieManager.SetCookie(shopID, cookieStr)
					cm.logger.Infof("✅ 成功从管理系统获取并存储Cookie: 店铺=%d", shopID)
					cookieJSON = cookieStr
				} else {
					return nil, fmt.Errorf("无法获取Cookie:店铺=%d", shopID)
				}
			}
		}

		if cookieJSON == "" {
			return nil, fmt.Errorf("Cookie不存在: 店铺=%d", shopID)
		}
	}

	client := NewAPIClientWithStoreInfo(shopID, cm.managementClient, storeInfo)
	cm.logger.Infof("🔧 创建客户端: 店铺=%d, baseURL=%s", shopID, client.GetBaseURL())
	return client, nil
}
