package management

import (
	"fmt"
	"sync"
	"task-processor/internal/core/config"
	domainproduct "task-processor/internal/product"
	"time"
)

// ClientManager 管理系统客户端管理器
type ClientManager struct {
	clients map[string]*ManagementAPIClient

	mutex sync.RWMutex
	// 固定的baseURL
	baseURL string

	// 图片下载客户端
	imageDownloader *ImageDownloader
	// 图片下载超时时间
	imageDownloadTimeout time.Duration

	// 品类限制缓存
	categoryRestrictionCache *CategoryRestrictionCache

	// 数据新鲜度天数
	dataFreshnessDays int
}

// NewClientManager 创建新的客户端管理器
func NewClientManager(cfg *config.ManagementConfig) *ClientManager {
	baseURL := "http://gateway.linkcloudai.com"
	if cfg != nil && cfg.BaseURL != "" {
		baseURL = cfg.BaseURL
	}

	cm := &ClientManager{
		clients: make(map[string]*ManagementAPIClient),
		// 设置默认的管理系统的基准URL
		baseURL: baseURL,
		// 设置默认的图片下载超时时间 - 增加到2分钟适应Amazon图片服务器
		imageDownloadTimeout: 120 * time.Second,
		// 默认数据新鲜度7天
		dataFreshnessDays: 7,
	}

	// 初始化品类限制缓存
	cm.categoryRestrictionCache = NewCategoryRestrictionCache(cm)

	return cm
}

// SetDataFreshnessDays 设置数据新鲜度天数
func (cm *ClientManager) SetDataFreshnessDays(days int) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.dataFreshnessDays = days
}

// GetClient 获取或创建管理系统的API客户端
func (cm *ClientManager) GetClient() *ManagementAPIClient {
	cm.mutex.RLock()
	client, exists := cm.clients[cm.baseURL]
	cm.mutex.RUnlock()

	if exists {
		return client
	}

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// 双重检查
	client, exists = cm.clients[cm.baseURL]
	if exists {
		return client
	}

	client = NewManagementAPIClientWithBaseURL(cm.baseURL)
	cm.clients[cm.baseURL] = client
	return client
}

// SetUserToken 设置用户访问令牌（从登录会话获取）
func (cm *ClientManager) SetUserToken(accessToken, tenantID string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// 为所有已创建的客户端设置令牌
	for _, client := range cm.clients {
		client.SetUserToken(accessToken, tenantID)
	}
}

// GetStoreClient 获取店铺API客户端
func (cm *ClientManager) GetStoreClient() *StoreAPIClient {
	// 直接基于基础客户端创建
	baseClient := cm.GetClient()
	return &StoreAPIClient{
		ManagementAPIClient: baseClient,
	}
}

// GetRawJsonDataClient 获取原始JSON数据API客户端
func (cm *ClientManager) GetRawJsonDataClient() *RawJsonDataAPIClient {
	// 直接基于基础客户端创建
	baseClient := cm.GetClient()
	client := &RawJsonDataAPIClient{
		ManagementAPIClient: baseClient,
	}
	// 设置数据新鲜度天数
	cm.mutex.RLock()
	client.SetDataFreshnessDays(cm.dataFreshnessDays)
	cm.mutex.RUnlock()
	return client
}

// GetRawJsonDataAdapter 获取适配为 domain 接口的原始JSON数据客户端
func (cm *ClientManager) GetRawJsonDataAdapter() domainproduct.RawJsonDataClient {
	return NewRawJsonDataAdapter(cm.GetRawJsonDataClient())
}

// GetFilterRuleClient 获取筛选规则API客户端
func (cm *ClientManager) GetFilterRuleClient() *FilterRuleAPIClient {
	// 直接基于基础客户端创建
	baseClient := cm.GetClient()
	return &FilterRuleAPIClient{
		ManagementAPIClient: baseClient,
	}
}

// GetProfitRuleClient 获取利润规则API客户端
func (cm *ClientManager) GetProfitRuleClient() *ProfitRuleAPIClient {
	// 直接基于基础客户端创建
	baseClient := cm.GetClient()
	return &ProfitRuleAPIClient{
		ManagementAPIClient: baseClient,
	}
}

// GetSensitiveWordClient 获取敏感词API客户端
func (cm *ClientManager) GetSensitiveWordClient() *SensitiveWordAPIClient {
	// 直接基于基础客户端创建
	baseClient := cm.GetClient()
	return &SensitiveWordAPIClient{
		ManagementAPIClient: baseClient,
	}
}

// GetCategoryRestrictionCollectionsClient 获取品类限制集合API客户端
func (cm *ClientManager) GetCategoryRestrictionCollectionsClient() *CategoryRestrictionCollectionsAPIClient {
	// 直接基于基础客户端创建
	baseClient := cm.GetClient()
	return &CategoryRestrictionCollectionsAPIClient{
		ManagementAPIClient: baseClient,
	}
}

// GetPricingRuleClient 获取自动核价规则API客户端
func (cm *ClientManager) GetPricingRuleClient() *PricingRuleAPIClient {
	// 直接基于基础客户端创建
	baseClient := cm.GetClient()
	return &PricingRuleAPIClient{
		ManagementAPIClient: baseClient,
	}
}

// GetImportTaskClient 获取导入任务API客户端
func (cm *ClientManager) GetImportTaskClient() *ImportTaskAPIClient {
	// 直接基于基础客户端创建
	baseClient := cm.GetClient()
	return &ImportTaskAPIClient{
		ManagementAPIClient: baseClient,
	}
}

// GetDailyListingCountClient 获取每日上架数量API客户端
func (cm *ClientManager) GetDailyListingCountClient() *DailyListingCountAPIClient {
	// 直接基于基础客户端创建
	baseClient := cm.GetClient()
	return &DailyListingCountAPIClient{
		ManagementAPIClient: baseClient,
	}
}

// GetProductImportMappingClient 获取产品导入映射API客户端
func (cm *ClientManager) GetProductImportMappingClient() *ProductImportMappingAPIClient {
	// 直接基于基础客户端创建
	baseClient := cm.GetClient()
	return &ProductImportMappingAPIClient{
		ManagementAPIClient: baseClient,
	}
}

// GetProductDataClient 获取产品数据API客户端
func (cm *ClientManager) GetProductDataClient(storeID int64) *ProductDataAPIClient {
	// 直接基于基础客户端创建
	baseClient := cm.GetClient()
	return &ProductDataAPIClient{
		ManagementAPIClient: baseClient,
		StoreID:             storeID,
	}
}

// GetProductDataClientWithTenant 获取产品数据API客户端（指定租户ID）
func (cm *ClientManager) GetProductDataClientWithTenant(storeID, tenantID int64) *ProductDataAPIClient {
	// 为每个店铺创建独立的客户端
	baseClient := NewManagementAPIClientWithBaseURL(cm.baseURL)

	// 从共享客户端获取访问令牌
	sharedClient := cm.GetClient()
	accessToken, _ := sharedClient.GetAccessToken()

	// 设置访问令牌和租户ID
	baseClient.SetUserToken(accessToken, fmt.Sprintf("%d", tenantID))

	return &ProductDataAPIClient{
		ManagementAPIClient: baseClient,
		StoreID:             storeID,
	}
}

// GetCategoryRestrictionCache 获取品类限制缓存
func (cm *ClientManager) GetCategoryRestrictionCache() *CategoryRestrictionCache {
	return cm.categoryRestrictionCache
}

// GetInventoryRecordClient 获取库存记录API客户端
func (cm *ClientManager) GetInventoryRecordClient() *InventoryRecordAPIClient {
	// 直接基于基础客户端创建
	baseClient := cm.GetClient()
	return &InventoryRecordAPIClient{
		ManagementAPIClient: baseClient,
	}
}

// GetOperationStrategyClient 获取运营策略API客户端
func (cm *ClientManager) GetOperationStrategyClient() *OperationStrategyClient {
	// 直接基于基础客户端创建
	baseClient := cm.GetClient()
	return &OperationStrategyClient{
		ManagementAPIClient: baseClient,
	}
}

// GetImageDownloader 获取图片下载客户端
func (cm *ClientManager) GetImageDownloader() *ImageDownloader {
	cm.mutex.RLock()

	if cm.imageDownloader != nil {
		defer cm.mutex.RUnlock()
		return cm.imageDownloader
	}

	// 如果还没有创建图片下载客户端，则创建一个新的
	cm.mutex.RUnlock()
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// 双重检查
	if cm.imageDownloader != nil {
		return cm.imageDownloader
	}

	cm.imageDownloader = NewImageDownloader(cm.imageDownloadTimeout)
	return cm.imageDownloader
}

// RemoveClient 移除客户端
func (cm *ClientManager) RemoveClient() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	delete(cm.clients, cm.baseURL)
}

// SetBaseURL 设置管理系统的基准URL
func (cm *ClientManager) SetBaseURL(baseURL string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.baseURL = baseURL
}

// SetImageDownloadTimeout 设置图片下载超时时间
func (cm *ClientManager) SetImageDownloadTimeout(timeout time.Duration) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.imageDownloadTimeout = timeout
	// 如果已经存在图片下载客户端，需要重新创建
	if cm.imageDownloader != nil {
		cm.imageDownloader = NewImageDownloader(timeout)
	}
}
