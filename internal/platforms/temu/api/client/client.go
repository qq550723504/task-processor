// Package client 提供TEMU平台API客户端核心功能
package client

import (
	"fmt"
	"net/http"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/clients/management/api"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

// APIClient TEMU API客户端 - 核心客户端，只负责基础的HTTP通信
type APIClient struct {
	config        *Config
	client        *req.Client
	storeID       int64
	cookies       []*http.Cookie
	cookieManager *CookieManager
	proxyURL      string // 代理地址
	logger        *logrus.Entry
	httpManager   *HTTPManager
	authManager   *AuthManager
}

// NewAPIClient 创建TEMU API客户端
func NewAPIClient(storeID int64, managementClient *management.ClientManager) *APIClient {
	config := DefaultConfig()

	logger := logrus.WithFields(logrus.Fields{
		"component": "TEMUAPIClient",
		"storeID":   storeID,
	})

	apiClient := &APIClient{
		config:        config,
		storeID:       storeID,
		cookieManager: NewCookieManager(storeID, managementClient),
		logger:        logger,
	}

	// 获取店铺配置信息（包括代理设置）
	if managementClient != nil {
		storeClient := managementClient.GetStoreClient()
		if storeClient != nil {
			if storeInfo, err := storeClient.GetStore(storeID); err != nil {
				apiClient.logger.WithError(err).Warn("获取店铺配置失败，将不使用代理")
			} else if storeInfo != nil && storeInfo.Proxy != "" {
				apiClient.proxyURL = storeInfo.Proxy
				apiClient.logger.Infof("店铺 %d 配置了代理地址: %s", storeID, storeInfo.Proxy)
			}
		}
	}

	// 初始化各个管理器
	apiClient.httpManager = NewHTTPManager(apiClient.proxyURL, logger)
	apiClient.authManager = NewAuthManager(logger)

	// 初始化HTTP客户端
	apiClient.client = apiClient.httpManager.CreateClient()

	// 在初始化时测试管理系统连接
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

	// 初始化时处理MallID设置逻辑
	apiClient.initializeMallID(managementClient)

	return apiClient
}

// initializeMallID 初始化时处理MallID设置逻辑
func (c *APIClient) initializeMallID(managementClient *management.ClientManager) {
	if managementClient == nil {
		c.logger.Warn("管理客户端为空，跳过MallID初始化")
		return
	}

	storeClient := managementClient.GetStoreClient()
	if storeClient == nil {
		c.logger.Warn("店铺客户端为空，跳过MallID初始化")
		return
	}

	// 获取店铺信息
	storeInfo, err := storeClient.GetStore(c.storeID)
	if err != nil {
		c.logger.WithError(err).Error("获取店铺信息失败，跳过MallID初始化")
		return
	}

	if storeInfo == nil {
		c.logger.Error("店铺信息为空，跳过MallID初始化")
		return
	}

	// 从Cookie中获取当前的MALL_ID
	cookieMallID := c.GetMallID()
	c.logger.Infof("Cookie中的MALL_ID: %s, 管理系统中的StoreID: %s", cookieMallID, storeInfo.StoreID)

	// 如果管理系统中的StoreID为空，但Cookie中有MALL_ID，则更新管理系统
	if storeInfo.StoreID == "" && cookieMallID != "" {
		c.logger.Infof("管理系统StoreID为空，使用Cookie中的MALL_ID更新: %s", cookieMallID)

		req := &api.StoreIdUpdateReqDTO{
			ID:      storeInfo.ID,
			StoreID: cookieMallID,
		}

		if _, err := storeClient.UpdateStoreId(req); err != nil {
			c.logger.WithError(err).Error("更新管理系统StoreID失败")
		} else {
			c.logger.Infof("成功更新管理系统StoreID为: %s", cookieMallID)
		}
	} else if storeInfo.StoreID != "" && cookieMallID != storeInfo.StoreID {
		// 如果管理系统中有StoreID，且与Cookie中的不一致，则更新Cookie
		c.logger.Infof("Cookie中的MALL_ID与管理系统不一致，更新Cookie: %s -> %s", cookieMallID, storeInfo.StoreID)
		c.SetMallID(storeInfo.StoreID)
		c.logger.Infof("成功更新Cookie中的MALL_ID为: %s", storeInfo.StoreID)
	} else if storeInfo.StoreID != "" && cookieMallID == storeInfo.StoreID {
		c.logger.Infof("MallID验证通过: %s", cookieMallID)
	} else {
		c.logger.Warn("管理系统StoreID和Cookie MALL_ID都为空")
	}
}

// SetCookies 设置Cookie
func (c *APIClient) SetCookies(cookies []*http.Cookie) {
	c.cookies = cookies
	// req包使用SetCommonCookies来设置全局Cookie
	c.client.SetCommonCookies(cookies...)
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

// HasCookies 检查是否有Cookie
func (c *APIClient) HasCookies() bool {
	return len(c.cookies) > 0
}

// GetCookieCount 获取Cookie数量
func (c *APIClient) GetCookieCount() int {
	return len(c.cookies)
}

// GetCookieValue 获取指定名称的Cookie值
func (c *APIClient) GetCookieValue(name string) string {
	for _, cookie := range c.cookies {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

// GetMallID 从Cookie中获取MALL_ID
func (c *APIClient) GetMallID() string {
	return c.GetCookieValue("MALL_ID")
}

// SetCookieValue 设置指定名称的Cookie值
func (c *APIClient) SetCookieValue(name, value string) {
	// 查找并更新现有Cookie
	for _, cookie := range c.cookies {
		if cookie.Name == name {
			cookie.Value = value
			c.logger.Infof("更新Cookie %s 的值为: %s", name, value)
			// 更新req客户端的Cookie
			c.client.SetCommonCookies(c.cookies...)
			return
		}
	}

	// 如果Cookie不存在，创建新的Cookie
	newCookie := &http.Cookie{
		Name:   name,
		Value:  value,
		Domain: ".temu.com",
		Path:   "/",
	}
	c.cookies = append(c.cookies, newCookie)
	c.logger.Infof("创建新Cookie %s 的值为: %s", name, value)
	// 更新req客户端的Cookie
	c.client.SetCommonCookies(c.cookies...)
}

// SetMallID 设置Cookie中的MALL_ID
func (c *APIClient) SetMallID(mallID string) {
	c.SetCookieValue("MALL_ID", mallID)
}

// SendTEMURequest 发送TEMU API请求（带Cookie检查和重试逻辑）
func (c *APIClient) SendTEMURequest(request map[string]any, result any) error {
	return c.authManager.SendRequestWithAuth(c, request, result)
}

// SendHTTPRequest 发送HTTP请求的内部方法
func (c *APIClient) SendHTTPRequest(method, url string, headers map[string]string, body any, formFields map[string]string, fileFields map[string]any) (*req.Response, error) {
	return c.httpManager.SendRequest(c.client, method, url, headers, body, formFields, fileFields)
}

// GetStoreID 获取店铺ID
func (c *APIClient) GetStoreID() int64 {
	return c.storeID
}

// GetBaseURL 获取基础URL
func (c *APIClient) GetBaseURL() string {
	return c.config.BaseURL
}

// GetConfig 获取配置
func (c *APIClient) GetConfig() any {
	return c.config
}

// GetLogger 获取日志记录器
func (c *APIClient) GetLogger() *logrus.Entry {
	return c.logger
}

// GetCookieManager 获取Cookie管理器
func (c *APIClient) GetCookieManager() any {
	return c.cookieManager
}
