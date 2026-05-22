// Package client 提供SHEIN平台的API客户端
package client

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"time"

	"task-processor/internal/core/logger"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

// APIClient SHEIN API客户端（参考TEMU设计）
type APIClient struct {
	storeID          int64
	tenantID         int64
	baseURL          string
	proxyURL         string
	managementClient *management.ClientManager
	httpClient       *req.Client
	logger           *logrus.Entry
	cookieManager    *CookieManager
	cookies          []*http.Cookie
}

// NewAPIClient 创建SHEIN API客户端
func NewAPIClient(storeID int64, managementClient *management.ClientManager) *APIClient {
	return newAPIClient(storeID, managementClient, nil, nil)
}

// NewAPIClientWithStoreInfo 使用已加载的店铺配置创建SHEIN API客户端
func NewAPIClientWithStoreInfo(storeID int64, managementClient *management.ClientManager, storeInfo *managementapi.StoreRespDTO) *APIClient {
	return newAPIClient(storeID, managementClient, toStoreConfig(storeInfo), nil)
}

func NewAPIClientWithStoreConfig(storeID int64, storeInfo *StoreConfig, cookieProvider CookieProvider) *APIClient {
	return newAPIClient(storeID, nil, storeInfo, cookieProvider)
}

func newAPIClient(storeID int64, managementClient *management.ClientManager, storeInfo *StoreConfig, cookieProvider CookieProvider) *APIClient {
	logger := logger.GetGlobalLogger("SheinAPIClient").WithField("storeID", storeID)

	// 创建HTTP客户端
	httpClient := req.C().
		SetTimeout(30*time.Second).
		SetCommonRetryCount(3).
		SetCommonRetryBackoffInterval(1*time.Second, 5*time.Second)

	apiClient := &APIClient{
		storeID:          storeID,
		baseURL:          "https://sellerhub.shein.com",
		managementClient: managementClient,
		httpClient:       httpClient,
		logger:           logger,
		cookieManager:    NewCookieManagerWithProvider(storeID, managementClient, cookieProvider),
	}

	// 获取店铺配置信息（包括代理设置和端点设置）
	if storeInfo == nil && managementClient != nil {
		storeClient := managementClient.GetStoreClient()
		if storeClient != nil {
			if info, err := storeClient.GetStore(storeID); err != nil {
				apiClient.logger.WithError(err).Warn("获取店铺配置失败，将使用默认配置")
			} else {
				storeInfo = toStoreConfig(info)
			}
		}
	}
	apiClient.applyStoreConfig(storeInfo)

	// 直接加载 Cookie，避免初始化时额外执行一次 GetStore 探活。
	// 店铺配置接口不可用时，调用方会在构建期校验里进入 blocked，而不是长时间卡住。
	if cookies, err := apiClient.cookieManager.LoadCookies(); err != nil {
		apiClient.logger.WithError(err).Error("初始化时加载Cookie失败")
	} else if cookies != nil {

		apiClient.SetCookies(cookies)
		apiClient.logger.Info("成功在初始化时加载Cookie")
	} else {
		apiClient.logger.Info("初始化时未找到Cookie数据")
	}

	return apiClient
}

func toStoreConfig(storeInfo *managementapi.StoreRespDTO) *StoreConfig {
	if storeInfo == nil {
		return nil
	}
	return &StoreConfig{
		ID:       storeInfo.ID,
		TenantID: storeInfo.TenantID,
		StoreID:  strings.TrimSpace(storeInfo.StoreID),
		Name:     strings.TrimSpace(storeInfo.Name),
		Platform: strings.TrimSpace(storeInfo.Platform),
		Region:   strings.TrimSpace(storeInfo.Region),
		LoginURL: strings.TrimSpace(storeInfo.LoginUrl),
		Proxy:    strings.TrimSpace(storeInfo.Proxy),
	}
}

func (c *APIClient) applyStoreConfig(storeInfo *StoreConfig) {
	if storeInfo == nil {
		return
	}

	c.tenantID = storeInfo.TenantID
	if c.cookieManager != nil && storeInfo.TenantID > 0 {
		c.cookieManager.resolvedTenantID = storeInfo.TenantID
	}

	// 本地联调可显式禁用店铺代理，避免本机依赖代理可达性。
	if storeInfo.Proxy != "" && !ignoreStoreProxy() {
		c.proxyURL = storeInfo.Proxy
		c.httpClient.SetProxyURL(storeInfo.Proxy)
		c.logger.Infof("店铺 %d 配置了代理地址: %s", c.storeID, storeInfo.Proxy)
	}

	// 根据店铺的 loginUrl 来设置客户端端点；本地调试可传完整 URL。
	switch {
	case strings.HasPrefix(storeInfo.LoginURL, "http://") || strings.HasPrefix(storeInfo.LoginURL, "https://"):
		c.baseURL = strings.TrimRight(storeInfo.LoginURL, "/")
		c.logger.Infof("店铺 %d 使用显式端点: %s", c.storeID, c.baseURL)
	case storeInfo.LoginURL == "sso.geiwohuo.com":
		c.baseURL = "https://sso.geiwohuo.com"
		c.logger.Infof("店铺 %d 使用第三方端点: %s", c.storeID, c.baseURL)
	default:
		c.logger.Infof("店铺 %d 使用自营端点: %s", c.storeID, c.baseURL)
	}
}

func ignoreStoreProxy() bool {
	value := strings.TrimSpace(os.Getenv("TASK_PROCESSOR_SHEIN_IGNORE_STORE_PROXY"))
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// SetCookies 设置Cookie（参考TEMU实现）
func (c *APIClient) SetCookies(cookies []*http.Cookie) {
	c.cookies = cookies
	c.httpClient.ClearCookies()
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

func (c *APIClient) ForceRefreshCookies() error {
	cookies, err := c.cookieManager.ForceRefreshCookies()
	if err != nil {
		c.logger.WithError(err).Error("强制刷新Cookie失败")
		return fmt.Errorf("强制刷新Cookie失败: %w", err)
	}
	if len(cookies) == 0 {
		return fmt.Errorf("强制刷新Cookie后仍未找到Cookie数据")
	}
	c.SetCookies(cookies)
	c.logger.Info("成功强制刷新Cookie")
	return nil
}

// HasCookies 检查是否有Cookie（改进版本）
func (c *APIClient) HasCookies() bool {
	return len(c.cookies) > 0
}

// GetCookieCount 获取Cookie数量（改进版本）
func (c *APIClient) GetCookieCount() int {
	return len(c.cookies)
}

// GetStoreID 获取店铺ID
func (c *APIClient) GetStoreID() int64 {
	return c.storeID
}

// GetCookieManager 获取Cookie管理器
func (c *APIClient) GetCookieManager() *CookieManager {
	return c.cookieManager
}

// GetHTTPClient 获取HTTP客户端
func (c *APIClient) GetHTTPClient() *req.Client {
	return c.httpClient
}

// GetProxyURL 获取当前客户端配置的代理地址
func (c *APIClient) GetProxyURL() string {
	return c.proxyURL
}

// GetManagementClient 获取管理客户端
func (c *APIClient) GetManagementClient() *management.ClientManager {
	return c.managementClient
}

// GetBaseURL 获取基础URL
func (c *APIClient) GetBaseURL() string {
	if c.baseURL != "" {
		return c.baseURL
	}
	return "https://sellerhub.shein.com"
}

// GetTenantID 获取租户ID
func (c *APIClient) GetTenantID() int64 {
	if c.cookieManager != nil {
		if tenantID := c.cookieManager.GetResolvedTenantID(); tenantID > 0 {
			return tenantID
		}
	}
	if c.tenantID > 0 {
		return c.tenantID
	}
	return 0
}
