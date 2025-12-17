package shops

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	management_api "task-processor/internal/common/management/api"
	"task-processor/internal/common/memory"
	shein_api "task-processor/internal/common/shein/api"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

// ClientManager 店铺API客户端管理器
type ClientManager struct {
	clients       map[string]shein_api.APIClient
	cookieManager *memory.CookieManager
	mutex         sync.RWMutex
}

// NewClientManager 创建新的客户端管理器
func NewClientManager(cookieManager *memory.CookieManager) *ClientManager {
	return &ClientManager{
		clients:       make(map[string]shein_api.APIClient),
		cookieManager: cookieManager,
	}
}

// GetClient 获取或创建店铺API客户端
func (cm *ClientManager) GetClient(tenantID, shopID int64, storeInfo *management_api.StoreRespDTO) (shein_api.APIClient, error) {

	key := fmt.Sprintf("shein:cookie:%d:%d", tenantID, shopID)

	// 先尝试读锁获取已存在的客户端
	cm.mutex.RLock()
	client, exists := cm.clients[key]
	cm.mutex.RUnlock()

	if exists {
		return client, nil
	}

	// 双重检查锁模式：再次检查以避免重复创建
	cm.mutex.Lock()
	client, exists = cm.clients[key]
	if exists {
		cm.mutex.Unlock()
		return client, nil
	}

	// 创建新的客户端
	client, err := cm.createClient(tenantID, shopID, storeInfo)
	if err != nil {
		cm.mutex.Unlock()
		// 直接返回原始错误，不包装，以便类型断言能够工作
		return nil, err
	}

	// 存储客户端
	cm.clients[key] = client
	logrus.Infof("已创建并存储店铺API客户端: %s", key)
	cm.mutex.Unlock()

	logrus.Infof("成功创建并缓存店铺API客户端: 租户=%d, 店铺=%d", tenantID, shopID)
	return client, nil
}

// RemoveClient 删除指定店铺的客户端缓存
func (cm *ClientManager) RemoveClient(tenantID, shopID int64) {
	key := fmt.Sprintf("shein:cookie:%d:%d", tenantID, shopID)

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if _, exists := cm.clients[key]; exists {
		delete(cm.clients, key)
		logrus.Infof("已删除店铺API客户端缓存: 租户=%d, 店铺=%d", tenantID, shopID)
	}
}

// GetAllClients 获取所有已创建的客户端信息
func (cm *ClientManager) GetAllClients() map[string]shein_api.APIClient {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// 创建一个副本以避免外部修改
	clientsCopy := make(map[string]shein_api.APIClient, len(cm.clients))
	for key, client := range cm.clients {
		clientsCopy[key] = client
	}

	return clientsCopy
}

// GetTenantShopPairs 从客户端键中提取所有租户和店铺对
func (cm *ClientManager) GetTenantShopPairs() []struct {
	TenantID int64
	ShopID   int64
} {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// 添加调试日志
	logrus.Infof("ClientManager中当前有 %d 个客户端", len(cm.clients))
	for key := range cm.clients {
		logrus.Infof("客户端键: %s", key)
	}

	var pairs []struct {
		TenantID int64
		ShopID   int64
	}

	for key := range cm.clients {
		// 解析键格式: shein:cookie:{tenantID}:{shopID}
		var tenantID, shopID int64
		if n, err := fmt.Sscanf(key, "shein:cookie:%d:%d", &tenantID, &shopID); err == nil && n == 2 {
			pairs = append(pairs, struct {
				TenantID int64
				ShopID   int64
			}{TenantID: tenantID, ShopID: shopID})
			logrus.Infof("解析到租户店铺对: 租户=%d, 店铺=%d", tenantID, shopID)
		} else {
			logrus.Infof("无法解析客户端键: %s, 错误: %v", key, err)
		}
	}

	logrus.Infof("最终获取到 %d 个租户店铺对", len(pairs))
	return pairs
}

// createClient 创建新的店铺API客户端
func (cm *ClientManager) createClient(tenantID, shopID int64, storeInfo *management_api.StoreRespDTO) (shein_api.APIClient, error) {
	// 从内存获取Cookie
	cookieJSON, err := cm.cookieManager.GetCookie(tenantID, shopID)
	if err != nil {
		// Cookie 不存在，尝试从 API 获取
		logrus.Warnf("内存中没有Cookie (租户=%d, 店铺=%d)，尝试从API获取", tenantID, shopID)

		// 注意：这里需要从外部传入 managementClient 来调用 GetStoreCookie
		// 由于当前架构限制，我们返回一个特殊的错误，让调用方处理
		return nil, &shein_api.CookieError{
			TenantID: tenantID,
			ShopID:   shopID,
			Code:     "COOKIE_NOT_FOUND",
			Message:  fmt.Sprintf("Cookie不存在，需要从API获取: 租户=%d, 店铺=%d", tenantID, shopID),
		}
	}

	// 根据storeType设置baseURL和店铺类型
	var baseURL string
	var isSelfOperated bool
	if storeInfo != nil && storeInfo.LoginUrl == "sso.geiwohuo.com" {
		baseURL = "https://sso.geiwohuo.com"
		isSelfOperated = false
	} else {
		baseURL = "https://sellerhub.shein.com"
		isSelfOperated = true
	}

	// 解析键值对格式的Cookie数据
	var simpleCookies map[string]interface{}
	if err := json.Unmarshal([]byte(cookieJSON), &simpleCookies); err != nil {
		return nil, fmt.Errorf("解析Cookie JSON数据失败: %w", err)
	}

	// 转换为http.Cookie格式
	cookies := cm.convertToCookies(simpleCookies, isSelfOperated)

	// 初始化HTTP客户端
	httpClient := req.C().
		SetCommonHeaders(map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36",
		}).
		SetTimeout(120 * time.Second) // 增加到2分钟适应Amazon图片服务器

	// 检查是否有代理地址
	if storeInfo != nil && storeInfo.Proxy != "" {
		logrus.Infof("店铺 %d:%d 使用代理地址: %s", tenantID, shopID, storeInfo.Proxy)
		httpClient = httpClient.SetProxyURL(storeInfo.Proxy)
	}

	if cookies != nil {
		httpClient = httpClient.SetCommonCookies(cookies...)
	}

	logrus.Infof("成功创建HTTP客户端: 租户=%d, 店铺=%d", tenantID, shopID)

	// 返回实际的API客户端
	return NewShopAPIClient(baseURL, tenantID, shopID, httpClient), nil
}

// convertToCookies 将键值对Cookie数据转换为http.Cookie对象
func (cm *ClientManager) convertToCookies(cookieDict map[string]interface{}, isSelfOperated bool) []*http.Cookie {
	cookies := make([]*http.Cookie, 0, len(cookieDict))

	// 获取目标域名用于Cookie域名设置
	var targetDomain string
	if isSelfOperated {
		targetDomain = "shein.com"
	} else {
		targetDomain = "geiwohuo.com"
	}

	for name, value := range cookieDict {
		cookie := &http.Cookie{
			Name:  name,
			Value: fmt.Sprintf("%v", value),
		}

		// 根据目标域名动态设置Cookie域名
		if targetDomain == "geiwohuo.com" {
			switch name {
			case "accept-language", "ssoEnvDevice", "gmp_trace", "gsp_trace", "gsp_store_site",
				"issso", "SITE_ID", "sso_login_trace":
				cookie.Domain = "sso.geiwohuo.com"
				cookie.Path = "/"
			case "_ga_BY7EZRXJL2":
				cookie.Domain = ".geiwohuo.com"
				cookie.Path = "/"
			case "armorUuid", "_ga", "smidV2", "zpnvSrwrNdywdz", "_ga_SJ3CWCK1VV":
				cookie.Domain = ".geiwohuo.com"
				cookie.Path = "/"
			default:
				cookie.Domain = ".geiwohuo.com"
				cookie.Path = "/"
			}
		} else {
			// 自营店铺的Cookie域名设置
			switch name {
			case "accept-language", "ssoEnvDevice", "gmp_trace", "gsp_trace", "gsp_store_site",
				"issso", "SITE_ID", "sso_login_trace", "_ga_BY7EZRXJL2":
				cookie.Domain = "sellerhub.shein.com"
				cookie.Path = "/"
			case "armorUuid", "_ga", "smidV2", "tfstk", "zpnvSrwrNdywdz", "_ga_SJ3CWCK1VV", "sessionID_shein", "req_central_p2_dc":
				cookie.Domain = ".shein.com"
				cookie.Path = "/"
			default:
				cookie.Domain = ".shein.com"
				cookie.Path = "/"
			}
		}

		cookies = append(cookies, cookie)
	}

	return cookies
}
