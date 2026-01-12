// Package client 提供TEMU平台API客户端核心功能
package client

import (
	"fmt"
	"net/http"
	"task-processor/internal/pkg/management"
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

// ConfigProvider 配置提供者接口（避免循环导入）
type ConfigProvider interface {
	GetAmazonConfig() interface{}
	GetAmazonProcessor() interface{}
	GetPlatformConfig() interface{}
}

// AutoPricingService 自动核价服务接口（避免循环导入）
type AutoPricingService interface {
	AutoProcessPendingPricesWithRules(managementClient *management.ClientManager) (*models.PricingStatistics, error)
	AutoProcessPendingPricesWithRulesAndAmazon(managementClient *management.ClientManager, configProvider ConfigProvider) (*models.PricingStatistics, error)
}

// PricingAPI 定价API管理器（避免循环导入）
type PricingAPI struct {
	client APIClientInterface
	logger *logrus.Entry
}

// NewPricingAPI 创建新的定价API管理器
func NewPricingAPI(client APIClientInterface, logger *logrus.Entry) *PricingAPI {
	return &PricingAPI{
		client: client,
		logger: logger,
	}
}

// APIClient TEMU API客户端 - 使用req库的增强版本
type APIClient struct {
	config        *Config
	client        *req.Client
	tenantID      int64
	storeID       int64
	cookies       []*http.Cookie
	cookieManager *CookieManager
	proxyURL      string // 代理地址
	logger        *logrus.Entry
	httpManager   *HTTPManager
	authManager   *AuthManager
}

// NewAPIClient 创建TEMU API客户端
func NewAPIClient(tenantID, storeID int64, managementClient *management.ClientManager) *APIClient {
	config := DefaultConfig()

	logger := logrus.WithFields(logrus.Fields{
		"component": "TEMUAPIClient",
		"tenantID":  tenantID,
		"storeID":   storeID,
	})

	apiClient := &APIClient{
		config:        config,
		tenantID:      tenantID,
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
	apiClient.authManager = NewAuthManager(apiClient.cookieManager, logger)

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

// GetConfig 获取配置
func (c *APIClient) GetConfig() interface{} {
	return c.config
}

// GetLogger 获取日志记录器
func (c *APIClient) GetLogger() *logrus.Entry {
	return c.logger
}

// GetCookieManager 获取Cookie管理器
func (c *APIClient) GetCookieManager() interface{} {
	return c.cookieManager
}

// SendHTTPRequestInterface 发送HTTP请求（接口适配器）
func (c *APIClient) SendHTTPRequestInterface(method, url string, headers map[string]string, body any, formFields map[string]string, fileFields map[string]any) (interface{}, error) {
	return c.SendHTTPRequest(method, url, headers, body, formFields, fileFields)
}

// AutoProcessPendingPricesWithRules 根据利润率规则智能处理待核价商品
func (c *APIClient) AutoProcessPendingPricesWithRules(managementClient *management.ClientManager) (*models.PricingStatistics, error) {
	c.logger.Info("开始智能核价处理")

	// 参数校验
	if managementClient == nil {
		return nil, fmt.Errorf("managementClient不能为空")
	}

	// 处理待核价商品
	return c.processWithBasicRules(managementClient)
}

// AutoProcessPendingPricesWithRulesAndAmazon 根据利润率规则智能处理待核价商品（支持Amazon数据）
func (c *APIClient) AutoProcessPendingPricesWithRulesAndAmazon(managementClient *management.ClientManager, configProvider interface{}) (*models.PricingStatistics, error) {
	c.logger.Info("开始智能核价处理（Amazon增强版）")

	// 参数校验
	if managementClient == nil {
		return nil, fmt.Errorf("managementClient不能为空")
	}

	if configProvider == nil {
		c.logger.Warn("配置提供者为空，使用基础决策服务")
		return c.AutoProcessPendingPricesWithRules(managementClient)
	}

	// 处理待核价商品（Amazon增强版）
	return c.processWithAmazonRules(managementClient, configProvider)
}

// processWithBasicRules 使用基础规则处理待核价商品
func (c *APIClient) processWithBasicRules(managementClient *management.ClientManager) (*models.PricingStatistics, error) {
	stats := &models.PricingStatistics{}
	pageNo := 1
	pageSize := 25

	for {
		// 获取待核价列表
		resp, err := c.getPendingPriceList(pageNo, pageSize)
		if err != nil {
			return stats, fmt.Errorf("获取待核价列表失败: %w", err)
		}

		if resp == nil || len(resp.Result.SalesBoostGoodsList) == 0 {
			c.logger.Info("没有更多待核价商品")
			break
		}

		// 遍历商品列表
		for _, goods := range resp.Result.SalesBoostGoodsList {
			for _, sku := range goods.SalesBoostSkuList {
				stats.TotalProcessed++

				// 简单的决策逻辑：接受所有价格
				decision := &models.PricingDecision{
					Action: models.DecisionAccept,
					Reason: "基础规则：接受平台报价",
				}

				// 执行决策
				if err := c.executeDecisionForSalesBoost(decision, &goods, &sku); err != nil {
					c.logger.WithError(err).Warnf("商品 %s SKU %s 执行决策失败",
						goods.SalesBoostGoodsBasicInfo.GoodsName, sku.SkuID)
					stats.FailCount++
				} else {
					stats.SuccessCount++
					stats.AcceptCount++
				}
			}
		}

		// 检查是否处理完所有商品
		if stats.TotalProcessed >= resp.Result.Total {
			break
		}

		pageNo++
	}

	c.logger.Infof("📊 智能核价完成: 总数=%d, 接受=%d, 成功=%d, 失败=%d",
		stats.TotalProcessed, stats.AcceptCount, stats.SuccessCount, stats.FailCount)

	return stats, nil
}

// processWithAmazonRules 使用Amazon增强规则处理待核价商品
func (c *APIClient) processWithAmazonRules(managementClient *management.ClientManager, configProvider interface{}) (*models.PricingStatistics, error) {
	// 目前先使用基础规则，后续可以扩展Amazon逻辑
	c.logger.Info("Amazon增强功能暂未实现，使用基础规则")
	return c.processWithBasicRules(managementClient)
}

// getPendingPriceList 获取待核价列表
func (c *APIClient) getPendingPriceList(pageNo, pageSize int) (*models.PendingPriceListResponse, error) {
	c.logger.Infof("获取待核价列表: pageNo=%d, pageSize=%d", pageNo, pageSize)

	req := &models.PendingPriceListRequest{
		PageSize: pageSize,
		PageNo:   pageNo,
		Scene:    "PRICING_HEALTH_SALES_BOOST", // 价格健康-销量提升场景
	}

	headers := GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/"

	request := map[string]interface{}{
		"method":  "POST",
		"url":     "/mms/marigold/price/v2/search_sales_boost",
		"headers": headers,
		"body":    req,
	}

	var result models.PendingPriceListResponse
	if err := c.SendTEMURequest(request, &result); err != nil {
		c.logger.WithError(err).Error("获取待核价列表失败")
		return nil, fmt.Errorf("获取待核价列表失败: %w", err)
	}

	if !result.Success {
		c.logger.Errorf("获取待核价列表失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("获取待核价列表失败: errorCode=%d", result.ErrorCode)
	}

	c.logger.Infof("成功获取待核价列表: 总数=%d, 当前页商品数=%d",
		result.Result.Total, len(result.Result.SalesBoostGoodsList))

	return &result, nil
}

// executeDecisionForSalesBoost 执行核价决策
func (c *APIClient) executeDecisionForSalesBoost(decision *models.PricingDecision, goods *models.SalesBoostGoods, sku *models.SalesBoostSku) error {
	switch decision.Action {
	case models.DecisionAccept:
		// 接受平台报价
		if sku.TargetSupplierPrice.Amount == "" || sku.TargetSupplierPrice.Currency == "" {
			return fmt.Errorf("目标价格信息不完整")
		}
		return c.acceptPrice(goods.SalesBoostGoodsBasicInfo.GoodsID, sku)

	case models.DecisionReject:
		// 拒绝报价
		return c.rejectPrice(goods.SalesBoostGoodsBasicInfo.GoodsID, []string{sku.SkuID})

	case models.DecisionSkip:
		// 跳过，不做任何操作
		c.logger.Info("跳过")
		return nil

	default:
		return fmt.Errorf("未知的决策动作: %s", decision.Action)
	}
}

// acceptPrice 接受平台报价
func (c *APIClient) acceptPrice(goodsID string, sku *models.SalesBoostSku) error {
	c.logger.Infof("接受平台报价: goodsID=%s, skuID=%s", goodsID, sku.SkuID)

	skuList := []models.AcceptPriceSkuInfo{
		{
			SkuID:                  sku.SkuID,
			Currency:               sku.TargetSupplierPrice.Currency,
			TargetSupplierPriceStr: sku.TargetSupplierPrice.Amount,
		},
	}

	req := &models.AcceptPriceRequest{
		Scene:   2,
		GoodsID: goodsID,
		SkuList: skuList,
	}

	headers := GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/products.html"

	request := map[string]interface{}{
		"method":  "POST",
		"url":     "/mms/marigold/price/goods/change",
		"headers": headers,
		"body":    req,
	}

	var result models.AcceptPriceResponse
	if err := c.SendTEMURequest(request, &result); err != nil {
		c.logger.WithError(err).Error("接受平台报价失败")
		return fmt.Errorf("接受平台报价失败: %w", err)
	}

	if !result.Success {
		c.logger.Errorf("接受平台报价失败: errorCode=%d", result.ErrorCode)
		return fmt.Errorf("接受平台报价失败: errorCode=%d", result.ErrorCode)
	}

	c.logger.Info("成功接受平台报价")
	return nil
}

// rejectPrice 拒绝平台报价
func (c *APIClient) rejectPrice(goodsID string, skuIDs []string) error {
	c.logger.Infof("拒绝平台报价: goodsID=%s, skuIDs=%v", goodsID, skuIDs)

	req := &models.RejectPriceRequest{
		GoodsID:         goodsID,
		SkuIDs:          skuIDs,
		OperationSource: 1005,
	}

	headers := GetDefaultHeaders()
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["x-document-referer"] = "https://seller.temu.com/products.html"

	request := map[string]interface{}{
		"method":  "POST",
		"url":     "/mms/marigold/sku/offline",
		"headers": headers,
		"body":    req,
	}

	var result models.RejectPriceResponse
	if err := c.SendTEMURequest(request, &result); err != nil {
		c.logger.WithError(err).Error("拒绝平台报价失败")
		return fmt.Errorf("拒绝平台报价失败: %w", err)
	}

	if !result.Success {
		c.logger.Errorf("拒绝平台报价失败: errorCode=%d", result.ErrorCode)
		return fmt.Errorf("拒绝平台报价失败: errorCode=%d", result.ErrorCode)
	}

	c.logger.Info("成功拒绝平台报价")
	return nil
}
