package client

import (
	"fmt"
	"sync"
	"time"

	"task-processor/internal/infra/memory"
	"task-processor/internal/pkg/management"
	management_api "task-processor/internal/pkg/management/api"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

// ClientManager 店铺API客户端管理器
type ClientManager struct {
	clients          map[string]*APIClient
	cookieManager    *memory.CookieManager
	managementClient *management.ClientManager
	mutex            sync.RWMutex
	logger           *logrus.Entry
}

// NewClientManager 创建新的客户端管理器
func NewClientManager(cookieManager *memory.CookieManager, managementClient *management.ClientManager) *ClientManager {
	return &ClientManager{
		clients:          make(map[string]*APIClient),
		cookieManager:    cookieManager,
		managementClient: managementClient,
		logger: logrus.WithFields(logrus.Fields{
			"component": "SHEINClientManager",
		}),
	}
}

// GetClient 获取或创建店铺API客户端
func (cm *ClientManager) GetClient(shopID int64, storeInfo *management_api.StoreRespDTO) (*APIClient, error) {
	key := fmt.Sprintf("shein:cookie:%d", shopID)

	// 先尝试读锁获取已存在的客户端
	cm.mutex.RLock()
	client, exists := cm.clients[key]
	cm.mutex.RUnlock()

	if exists {
		return client, nil
	}

	// 双重检查锁模式：再次检查以避免重复创建
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	client, exists = cm.clients[key]
	if exists {
		return client, nil
	}

	// 创建新的客户端
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
func (cm *ClientManager) GetAllClients() map[string]*APIClient {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// 创建一个副本以避免外部修改
	clientsCopy := make(map[string]*APIClient, len(cm.clients))
	for key, client := range cm.clients {
		clientsCopy[key] = client
	}

	return clientsCopy
}

// createClient 创建新的店铺API客户端（使用已有的CookieManager）
func (cm *ClientManager) createClient(shopID int64, storeInfo *management_api.StoreRespDTO) (*APIClient, error) {
	// 尝试从内存获取Cookie
	cookieJSON, err := cm.cookieManager.GetCookie(shopID)
	if err != nil {
		// 内存中没有Cookie，尝试从管理系统获取
		cm.logger.Warnf("内存中没有Cookie (店铺=%d)，尝试从管理系统获取", shopID)

		if cm.managementClient != nil {
			if storeClient := cm.managementClient.GetStoreClient(); storeClient != nil {
				if cookieStr, err := storeClient.GetStoreCookie(shopID); err == nil && cookieStr != "" {
					// 成功获取，存储到内存
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

	// 确定baseURL
	baseURL := "https://sellerhub.shein.com"
	if storeInfo != nil && storeInfo.LoginUrl == "sso.geiwohuo.com" {
		baseURL = "https://sso.geiwohuo.com"
	}

	cm.logger.Infof("🔧 创建客户端: 店铺=%d, baseURL=%s", shopID, baseURL)

	// 创建HTTP客户端
	httpClient := req.C().
		SetCommonHeaders(map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36",
		}).
		SetTimeout(120 * time.Second)

	// 设置代理
	if storeInfo != nil && storeInfo.Proxy != "" {
		httpClient = httpClient.SetProxyURL(storeInfo.Proxy)
		cm.logger.Infof("🌐 设置代理: %s", storeInfo.Proxy)
	}

	// 解析并设置Cookie - 使用专门的CookieManager
	if cookieJSON != "" {
		// 创建临时的CookieManager来解析Cookie
		tempCookieManager := NewCookieManager(shopID, cm.managementClient)
		cookies, err := tempCookieManager.ParseCookieString(cookieJSON)
		if err != nil {
			cm.logger.Errorf("❌ Cookie解析失败: %v", err)
			return nil, fmt.Errorf("Cookie解析失败: %w", err)
		}

		if len(cookies) > 0 {
			httpClient = httpClient.SetCommonCookies(cookies...)
			cm.logger.Infof("🍪 成功设置Cookie: 店铺=%d, Cookie数量=%d", shopID, len(cookies))
		} else {
			cm.logger.Warnf("⚠️ 解析后Cookie数量为0: 店铺=%d", shopID)
		}
	}
	// 创建本地的APIClient而不是使用shein_api包
	return NewAPIClient(shopID, cm.managementClient), nil
}
