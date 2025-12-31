// Package shops 提供SHEIN平台的API客户端
package shops

import (
	"fmt"
	"net/http"
	"task-processor/internal/pkg/management"
	"time"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

// APIClient SHEIN API客户端（参考TEMU设计）
type APIClient struct {
	tenantID         int64
	storeID          int64
	managementClient *management.ClientManager
	httpClient       *req.Client
	logger           *logrus.Entry
	shopAPIClient    *ShopAPIClient
	cookieManager    *CookieManager
	cookies          []*http.Cookie
}

// NewAPIClient 创建SHEIN API客户端（参考TEMU实现）
func NewAPIClient(tenantID, storeID int64, managementClient *management.ClientManager) *APIClient {
	logger := logrus.WithFields(logrus.Fields{
		"component": "SheinAPIClient",
		"tenantID":  tenantID,
		"storeID":   storeID,
	})

	// 创建HTTP客户端
	httpClient := req.C().
		SetTimeout(30*time.Second).
		SetCommonRetryCount(3).
		SetCommonRetryBackoffInterval(1*time.Second, 5*time.Second)

	apiClient := &APIClient{
		tenantID:         tenantID,
		storeID:          storeID,
		managementClient: managementClient,
		httpClient:       httpClient,
		logger:           logger,
		cookieManager:    NewCookieManager(storeID, managementClient),
	}

	// 获取店铺配置信息（包括代理设置和端点设置）
	var baseURL string = "https://sellerhub.shein.com" // 默认使用自营店铺端点

	if managementClient != nil {
		storeClient := managementClient.GetStoreClient()
		if storeClient != nil {
			if storeInfo, err := storeClient.GetStore(storeID); err != nil {
				apiClient.logger.WithError(err).Warn("获取店铺配置失败，将使用默认配置")
			} else if storeInfo != nil {
				// 设置代理
				if storeInfo.Proxy != "" {
					apiClient.httpClient.SetProxyURL(storeInfo.Proxy)
					apiClient.logger.Infof("店铺 %d 配置了代理地址: %s", storeID, storeInfo.Proxy)
				}

				// 根据店铺的loginUrl来设置客户端的端点
				if storeInfo.LoginUrl == "sso.geiwohuo.com" {
					baseURL = "https://sso.geiwohuo.com"
					apiClient.logger.Infof("店铺 %d 使用第三方端点: %s", storeID, baseURL)
				} else {
					baseURL = "https://sellerhub.shein.com"
					apiClient.logger.Infof("店铺 %d 使用自营端点: %s", storeID, baseURL)
				}
			}
		}
	}

	// 创建店铺API客户端
	apiClient.shopAPIClient = NewShopAPIClient(
		baseURL,
		tenantID,
		storeID,
		apiClient.httpClient,
	)

	// 在初始化时测试管理系统连接并加载Cookie（参考TEMU实现）
	if err := apiClient.cookieManager.TestConnection(); err != nil {
		apiClient.logger.WithError(err).Error("管理系统连接测试失败，跳过Cookie加载")
	} else {
		// 连接正常，尝试加载Cookie
		if cookies, err := apiClient.cookieManager.LoadCookies(); err != nil {
			apiClient.logger.WithError(err).Error("初始化时加载Cookie失败")
		} else if cookies != nil {

			apiClient.SetCookies(cookies)
			apiClient.logger.Info("成功在初始化时加载Cookie")
		} else {
			apiClient.logger.Info("初始化时未找到Cookie数据")
		}
	}

	return apiClient
}

// SetCookies 设置Cookie（参考TEMU实现）
func (c *APIClient) SetCookies(cookies []*http.Cookie) {
	c.cookies = cookies
	// 使用req包的SetCommonCookies来设置全局Cookie
	c.httpClient.SetCommonCookies(cookies...)

	c.logger.WithField("cookieNum", len(cookies)).Info("设置Cookie")
}

// ReloadCookies 重新加载Cookie
func (c *APIClient) ReloadCookies() error {
	cookies, err := c.cookieManager.LoadCookies()
	if err != nil {
		c.logger.WithError(err).Error("重新加载Cookie失败")
		return fmt.Errorf("重新加载Cookie失败: %w", err)
	}

	if cookies != nil {
		c.SetCookies(cookies)
		c.logger.Info("成功重新加载Cookie")
	} else {
		c.logger.Info("未找到Cookie数据")
	}

	return nil
}

// loadCookies 加载Cookie（已废弃，使用CookieManager替代）
// Deprecated: 使用 ReloadCookies() 方法替代
func (c *APIClient) loadCookies() {
	c.logger.Warn("使用了已废弃的loadCookies方法，建议使用ReloadCookies()")

	// 使用新的Cookie管理器重新加载
	if err := c.ReloadCookies(); err != nil {
		c.logger.WithError(err).Error("通过CookieManager重新加载Cookie失败")
	}
}

// GetShopAPIClient 获取店铺API客户端
func (c *APIClient) GetShopAPIClient() *ShopAPIClient {
	return c.shopAPIClient
}

// HasCookies 检查是否有Cookie（改进版本）
func (c *APIClient) HasCookies() bool {
	return len(c.cookies) > 0
}

// GetCookieCount 获取Cookie数量（改进版本）
func (c *APIClient) GetCookieCount() int {
	return len(c.cookies)
}

// GetTenantID 获取租户ID
func (c *APIClient) GetTenantID() int64 {
	return c.tenantID
}

// GetStoreID 获取店铺ID
func (c *APIClient) GetStoreID() int64 {
	return c.storeID
}

// GetCookieManager 获取Cookie管理器
func (c *APIClient) GetCookieManager() *CookieManager {
	return c.cookieManager
}
