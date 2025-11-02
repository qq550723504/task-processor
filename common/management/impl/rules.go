package impl

import (
	"encoding/json"
	"fmt"
	"task-processor/common/management/api"
)

// PricingRuleClient 自动核价规则API客户端实现
type PricingRuleClient struct {
	*BaseManagementClient
}

// NewPricingRuleClient 创建自动核价规则API客户端
func NewPricingRuleClient(baseURL string) *PricingRuleClient {
	return &PricingRuleClient{
		BaseManagementClient: NewBaseManagementClient(baseURL),
	}
}

// GetPricingRule 获取自动核价规则
func (c *PricingRuleClient) GetPricingRule(req *api.PricingRuleReqDTO) (*api.PricingRuleRespDTO, error) {
	url := fmt.Sprintf("/rpc-api/listing/pricing-rule/get?tenantId=%d", req.TenantID)

	// 添加可选参数
	if req.StoreID != nil {
		url = fmt.Sprintf("%s&storeId=%d", url, *req.StoreID)
	}
	if req.CategoryID != nil {
		url = fmt.Sprintf("%s&categoryId=%d", url, *req.CategoryID)
	}

	body, err := c.makeAPIRequestWithURL("GET", url)
	if err != nil {
		return nil, err
	}

	var result APIResponse
	result.Data = &api.PricingRuleRespDTO{}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	if result.Data == nil {
		return nil, fmt.Errorf("自动核价规则数据为空")
	}

	// 安全的类型断言
	rule, ok := result.Data.(*api.PricingRuleRespDTO)
	if !ok {
		return nil, fmt.Errorf("自动核价规则数据类型转换失败")
	}

	return rule, nil
}

// ProfitRuleClient 利润规则API客户端实现
type ProfitRuleClient struct {
	*BaseManagementClient
}

// NewProfitRuleClient 创建利润规则API客户端
func NewProfitRuleClient(baseURL string) *ProfitRuleClient {
	return &ProfitRuleClient{
		BaseManagementClient: NewBaseManagementClient(baseURL),
	}
}

// GetProfitRule 获取利润规则
func (c *ProfitRuleClient) GetProfitRule(req *api.ProfitRuleReqDTO) (*api.ProfitRuleRespDTO, error) {
	url := fmt.Sprintf("/rpc-api/listing/profit-rule/get?tenantId=%d&storeId=%d", req.TenantID, req.StoreID)

	body, err := c.makeAPIRequestWithURL("GET", url)
	if err != nil {
		return nil, err
	}

	var result APIResponse
	result.Data = &api.ProfitRuleRespDTO{}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	if result.Data == nil {
		return nil, fmt.Errorf("利润规则数据为空")
	}

	// 安全的类型断言
	rule, ok := result.Data.(*api.ProfitRuleRespDTO)
	if !ok {
		return nil, fmt.Errorf("利润规则数据类型转换失败")
	}

	return rule, nil
}

// FilterRuleClient 过滤规则API客户端实现
type FilterRuleClient struct {
	*BaseManagementClient
}

// NewFilterRuleClient 创建过滤规则API客户端
func NewFilterRuleClient(baseURL string) *FilterRuleClient {
	return &FilterRuleClient{
		BaseManagementClient: NewBaseManagementClient(baseURL),
	}
}

// GetFilterRule 获取过滤规则
func (c *FilterRuleClient) GetFilterRule(req *api.FilterRuleReqDTO) (*[]api.FilterRuleRespDTO, error) {
	url := fmt.Sprintf("/rpc-api/listing/filter-rule/get?storeId=%d", req.StoreID)

	// 添加可选参数
	if req.TenantID > 0 {
		url = fmt.Sprintf("%s&tenantId=%d", url, req.TenantID)
	}
	if req.CategoryID > 0 {
		url = fmt.Sprintf("%s&categoryId=%d", url, req.CategoryID)
	}

	body, err := c.makeAPIRequestWithURL("GET", url)
	if err != nil {
		return nil, err
	}

	var result APIResponse
	var rules []api.FilterRuleRespDTO
	result.Data = &rules

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	// 安全的类型断言
	if rulesPtr, ok := result.Data.(*[]api.FilterRuleRespDTO); ok {
		return rulesPtr, nil
	}

	return nil, fmt.Errorf("过滤规则数据类型转换失败")
}
