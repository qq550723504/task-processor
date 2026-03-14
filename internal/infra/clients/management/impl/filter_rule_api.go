package impl

import (
	"fmt"
	"net/http"
	"task-processor/internal/infra/clients/management/api"
)

// FilterRuleAPIClientImpl 筛选规则API客户端实现
type FilterRuleAPIClientImpl struct {
	*ManagementAPIClientImpl
}

// GetFilterRule 获取筛选规则
func (m *FilterRuleAPIClientImpl) GetFilterRule(req *api.FilterRuleReqDTO) (*[]api.FilterRuleRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/filter-rule/list?tenantId=%d&storeId=%d", m.baseURL, req.TenantID, req.StoreID)

	var result APIResponse
	result.Data = &[]api.FilterRuleRespDTO{}

	if err := m.apiRequest(http.MethodGet, url, nil, &result); err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	if result.Data == nil {
		return nil, fmt.Errorf("筛选规则数据为空")
	}

	filterRules, ok := result.Data.(*[]api.FilterRuleRespDTO)
	if !ok {
		return nil, fmt.Errorf("筛选规则数据类型转换失败")
	}

	return filterRules, nil
}
