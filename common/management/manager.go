package management

import (
	"task-processor/common/management/api"
	"task-processor/common/management/impl"
)

// ManagementManager 管理系统API管理器 - 包含所有API客户端
type ManagementManager struct {
	ImportTask  api.ImportTaskAPI
	Store       api.StoreAPI
	RawJsonData api.RawJsonDataAPI
	PricingRule api.PricingRuleAPI
	ProfitRule  api.ProfitRuleAPI
	FilterRule  api.FilterRuleAPI

	// 内部客户端实例
	importTaskClient  *impl.ImportTaskClient
	storeClient       *impl.StoreClient
	rawJsonDataClient *impl.RawJsonDataClient
	pricingRuleClient *impl.PricingRuleClient
	profitRuleClient  *impl.ProfitRuleClient
	filterRuleClient  *impl.FilterRuleClient
}

// NewManagementManager 创建管理系统API管理器
func NewManagementManager(baseURL string) *ManagementManager {
	// 创建所有客户端实例
	importTaskClient := impl.NewImportTaskClient(baseURL)
	storeClient := impl.NewStoreClient(baseURL)
	rawJsonDataClient := impl.NewRawJsonDataClient(baseURL)
	pricingRuleClient := impl.NewPricingRuleClient(baseURL)
	profitRuleClient := impl.NewProfitRuleClient(baseURL)
	filterRuleClient := impl.NewFilterRuleClient(baseURL)

	return &ManagementManager{
		// 接口实现
		ImportTask:  importTaskClient,
		Store:       storeClient,
		RawJsonData: rawJsonDataClient,
		PricingRule: pricingRuleClient,
		ProfitRule:  profitRuleClient,
		FilterRule:  filterRuleClient,

		// 内部客户端实例
		importTaskClient:  importTaskClient,
		storeClient:       storeClient,
		rawJsonDataClient: rawJsonDataClient,
		pricingRuleClient: pricingRuleClient,
		profitRuleClient:  profitRuleClient,
		filterRuleClient:  filterRuleClient,
	}
}

// SetUserToken 设置用户访问令牌到所有客户端
func (m *ManagementManager) SetUserToken(accessToken, tenantID string) {
	m.importTaskClient.SetUserToken(accessToken, tenantID)
	m.storeClient.SetUserToken(accessToken, tenantID)
	m.rawJsonDataClient.SetUserToken(accessToken, tenantID)
	m.pricingRuleClient.SetUserToken(accessToken, tenantID)
	m.profitRuleClient.SetUserToken(accessToken, tenantID)
	m.filterRuleClient.SetUserToken(accessToken, tenantID)
}

// GetUserToken 获取用户访问令牌
func (m *ManagementManager) GetUserToken() (string, string) {
	return m.importTaskClient.GetUserToken()
}

// HasValidToken 检查是否有有效的令牌
func (m *ManagementManager) HasValidToken() bool {
	return m.importTaskClient.HasValidToken()
}

// GetToken 获取当前访问令牌
func (m *ManagementManager) GetToken() (string, error) {
	return m.importTaskClient.GetToken()
}

// IsAuthenticated 检查是否已认证
func (m *ManagementManager) IsAuthenticated() bool {
	return m.importTaskClient.IsAuthenticated()
}
