package impl

import (
	"fmt"
	"net/http"
	"task-processor/common/management/api"
)

// PricingRuleAPIClientImpl 自动核价规则API客户端实现
type PricingRuleAPIClientImpl struct {
	*ManagementAPIClientImpl
}

// GetPricingRule 获取自动核价规则
func (m *PricingRuleAPIClientImpl) GetPricingRule(req *api.PricingRuleReqDTO) (*api.PricingRuleRespDTO, error) {
	// 构建URL，根据参数情况添加查询参数
	url := fmt.Sprintf("%s/rpc-api/listing/pricing-rule/get?tenantId=%d", m.baseURL, req.TenantID)

	if req.StoreID != nil {
		url = fmt.Sprintf("%s&storeId=%d", url, *req.StoreID)
	}

	if req.CategoryID != nil {
		url = fmt.Sprintf("%s&categoryId=%d", url, *req.CategoryID)
	}

	var result APIResponse
	result.Data = &api.PricingRuleRespDTO{}

	err := m.apiRequest(http.MethodGet, url, nil, &result)
	if err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil {
		return nil, fmt.Errorf("自动核价规则数据为空")
	}

	// 安全的类型断言
	pricingRule, ok := result.Data.(*api.PricingRuleRespDTO)
	if !ok {
		return nil, fmt.Errorf("自动核价规则数据类型转换失败")
	}

	return pricingRule, nil
}
