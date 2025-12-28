// Package client 提供TEMU平台的API客户端核心功能
package client

import (
	"fmt"
	"net/http"
	"strings"
	"task-processor/internal/common/management"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

// APIClient TEMU API客户端 - 使用req库的增强版本
type APIClient struct {
	config            *Config
	client            *req.Client
	tenantID          int64
	storeID           int64
	cookieHandler     *CookieHandler
	requestSender     *RequestSender
	productAPIHandler *ProductAPIHandler
	logger            *logrus.Entry
}

// NewAPIClient 创建TEMU API客户端
func NewAPIClient(tenantID, storeID int64, managementClient *management.ClientManager) *APIClient {
	config := DefaultConfig()

	logger := logrus.WithFields(logrus.Fields{
		"component": "TEMUAPIClient",
		"tenantID":  tenantID,
		"storeID":   storeID,
	})

	// 获取代理配置
	proxyURL := ""
	if managementClient != nil {
		storeClient := managementClient.GetStoreClient()
		if storeClient != nil {
			if storeInfo, err := storeClient.GetStore(storeID); err != nil {
				logger.WithError(err).Warn("获取店铺配置失败，将不使用代理")
			} else if storeInfo != nil && storeInfo.Proxy != "" {
				proxyURL = storeInfo.Proxy
				logger.Infof("店铺 %d 配置了代理地址: %s", storeID, storeInfo.Proxy)
			}
		}
	}

	// 创建HTTP配置管理器并初始化客户端
	httpConfigManager := NewHTTPConfigManager(config, proxyURL)
	client := httpConfigManager.InitHTTPClient()

	// 创建Cookie处理器
	cookieManager := NewCookieManager(storeID, managementClient)
	cookieHandler := NewCookieHandler(storeID, cookieManager, logger)

	// 创建请求发送器
	requestSender := NewRequestSender(client, config, cookieHandler, logger)

	// 创建产品API处理器
	productAPIHandler := NewProductAPIHandler(requestSender)

	apiClient := &APIClient{
		config:            config,
		client:            client,
		tenantID:          tenantID,
		storeID:           storeID,
		cookieHandler:     cookieHandler,
		requestSender:     requestSender,
		productAPIHandler: productAPIHandler,
		logger:            logger,
	}

	// 初始化Cookie
	cookieHandler.InitializeCookies()

	return apiClient
}

// SetCookies 设置Cookie
func (c *APIClient) SetCookies(cookies []*http.Cookie) {
	c.cookieHandler.SetCookies(cookies)
	// 同时设置到req客户端
	c.client.SetCommonCookies(cookies...)
}

// ReloadCookies 重新加载Cookie
func (c *APIClient) ReloadCookies() error {
	err := c.cookieHandler.ReloadCookies()
	if err == nil && c.cookieHandler.HasCookies() {
		// 同步到req客户端
		c.client.SetCommonCookies(c.cookieHandler.GetCookies()...)
	}
	return err
}

// HasCookies 检查是否有Cookie
func (c *APIClient) HasCookies() bool {
	return c.cookieHandler.HasCookies()
}

// GetCookieCount 获取Cookie数量
func (c *APIClient) GetCookieCount() int {
	return c.cookieHandler.GetCookieCount()
}

// SendTEMURequest 发送TEMU API请求（带Cookie检查和重试逻辑）
func (c *APIClient) SendTEMURequest(request map[string]interface{}, result interface{}) error {
	return c.requestSender.SendTEMURequest(request, result)
}

// GetTenantID 获取租户ID
func (c *APIClient) GetTenantID() int64 {
	return c.tenantID
}

// GetStoreID 获取店铺ID
func (c *APIClient) GetStoreID() int64 {
	return c.storeID
}

// GetBaseURL 获取基础URL
func (c *APIClient) GetBaseURL() string {
	return c.config.BaseURL
}

// ==================== 产品API代理方法 ====================

// ListProducts 获取产品列表
func (c *APIClient) ListProducts(pageNo, pageSize int) (*ProductListResponse, error) {
	return c.productAPIHandler.ListProducts(pageNo, pageSize)
}

// ==================== 认证错误处理 ====================

// isAuthenticationError 判断是否为认证相关错误
func (c *APIClient) isAuthenticationError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	// 检查常见的认证错误关键词
	authErrors := []string{
		"401",
		"403",
		"unauthorized",
		"forbidden",
		"登录",
		"认证",
		"权限",
		"cookie",
		"signature",
		"expired",
		"签名",
		"过期",
	}

	for _, keyword := range authErrors {
		if strings.Contains(errStr, keyword) {
			c.logger.Debugf("检测到认证错误关键词: %s", keyword)
			return true
		}
	}

	return false
}

// setPauseKeyForAuthExpired 设置认证过期暂停键
func (c *APIClient) setPauseKeyForAuthExpired(reason string) error {
	if c.cookieHandler.cookieManager == nil || c.cookieHandler.cookieManager.managementClient == nil {
		c.logger.Warn("管理客户端未初始化，无法设置暂停键")
		return fmt.Errorf("管理客户端未初始化")
	}

	storeClient := c.cookieHandler.cookieManager.managementClient.GetStoreClient()
	if storeClient == nil {
		c.logger.Warn("店铺客户端未初始化，无法设置暂停键")
		return fmt.Errorf("店铺客户端未初始化")
	}

	c.logger.Infof("设置店铺 %d 的认证过期暂停键，原因: %s", c.storeID, reason)
	success, err := storeClient.SetStorePauseStatus(c.storeID, true, "auth_expired")
	if err != nil {
		c.logger.Errorf("设置店铺 %d 的暂停状态失败: %v", c.storeID, err)
		return fmt.Errorf("设置暂停状态失败: %w", err)
	}

	if success {
		c.logger.Infof("✓ 成功设置店铺 %d 的认证过期暂停键", c.storeID)
	} else {
		c.logger.Warnf("设置店铺 %d 的暂停状态返回失败", c.storeID)
		return fmt.Errorf("设置暂停状态返回失败")
	}

	return nil
}
