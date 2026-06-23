package management

import (
	"fmt"
	"net/http"
	"task-processor/internal/infra/clients/management/api"
)

// ProfitRuleAPIClient 利润规则API客户端实现
type ProfitRuleAPIClient struct {
	*ManagementAPIClient
	localDataProvider *LocalDataProvider
}

// GetProfitRule 获取利润规则
func (m *ProfitRuleAPIClient) GetProfitRule(req *api.ProfitRuleReqDTO) (*api.ProfitRuleRespDTO, error) {
	if m.localDataProvider != nil && m.localDataProvider.HasDB() {
		return m.localDataProvider.GetProfitRule(req)
	}
	url := fmt.Sprintf("%s/rpc-api/listing/profit-rule/get?tenantId=%d&storeId=%d", m.baseURL, req.TenantID, req.StoreID)

	var result APIResponse
	result.Data = &api.ProfitRuleRespDTO{}

	if err := m.apiRequest(http.MethodGet, url, nil, &result); err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	if result.Data == nil {
		return nil, fmt.Errorf("利润规则数据为空")
	}

	profitRule, ok := result.Data.(*api.ProfitRuleRespDTO)
	if !ok {
		return nil, fmt.Errorf("利润规则数据类型转换失败")
	}

	return profitRule, nil
}
