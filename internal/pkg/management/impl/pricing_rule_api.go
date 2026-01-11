package impl

import (
	"encoding/json"
	"fmt"
	"net/http"
	"task-processor/internal/pkg/management/api"
)

// PricingRuleAPIClientImpl 自动核价规则API客户端实现
type PricingRuleAPIClientImpl struct {
	*ManagementAPIClientImpl
}

// GetPricingRule 获取自动核价规则（返回数组）
func (m *PricingRuleAPIClientImpl) GetPricingRule(req *api.PricingRuleReqDTO) ([]api.PricingRuleRespDTO, error) {
	// 构建URL，根据参数情况添加查询参数
	url := fmt.Sprintf("%s/rpc-api/listing/pricing-rule/get?tenantId=%d", m.baseURL, req.TenantID)

	if req.StoreID != nil {
		url = fmt.Sprintf("%s&storeId=%d", url, *req.StoreID)
	}

	if req.CategoryID != nil {
		url = fmt.Sprintf("%s&categoryId=%d", url, *req.CategoryID)
	}

	// 使用 json.RawMessage 来处理不确定的数据结构
	var result APIResponse
	var rawData json.RawMessage
	result.Data = &rawData

	err := m.apiRequest(http.MethodGet, url, nil, &result)
	if err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil || len(rawData) == 0 {
		return []api.PricingRuleRespDTO{}, nil
	}

	// 先尝试解析为数组
	var rulesArray []api.PricingRuleRespDTO
	if err := json.Unmarshal(rawData, &rulesArray); err == nil {
		return rulesArray, nil
	}

	// 如果数组解析失败，尝试解析为单个对象
	var singleRule api.PricingRuleRespDTO
	if err := json.Unmarshal(rawData, &singleRule); err == nil {
		return []api.PricingRuleRespDTO{singleRule}, nil
	}

	return nil, fmt.Errorf("无法解析自动核价规则数据")
}
