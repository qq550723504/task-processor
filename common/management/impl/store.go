package impl

import (
	"encoding/json"
	"fmt"
	"task-processor/common/management/api"
)

// StoreClient 店铺API客户端实现
type StoreClient struct {
	*BaseManagementClient
}

// NewStoreClient 创建店铺API客户端
func NewStoreClient(baseURL string) *StoreClient {
	return &StoreClient{
		BaseManagementClient: NewBaseManagementClient(baseURL),
	}
}

// GetStore 通过店铺ID获取店铺信息
func (c *StoreClient) GetStore(id int64) (*api.StoreRespDTO, error) {
	url := fmt.Sprintf("/rpc-api/listing/store/get?id=%d", id)

	body, err := c.makeAPIRequestWithURL("GET", url)
	if err != nil {
		return nil, err
	}

	var result APIResponse
	result.Data = &api.StoreRespDTO{}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil {
		return nil, fmt.Errorf("店铺信息数据为空: 店铺可能已被删除")
	}

	// 安全的类型断言
	store, ok := result.Data.(*api.StoreRespDTO)
	if !ok {
		return nil, fmt.Errorf("店铺信息数据类型转换失败")
	}

	return store, nil
}

// GetStoreCookie 通过店铺ID获取用户Cookie
func (c *StoreClient) GetStoreCookie(id int64) (string, error) {
	url := fmt.Sprintf("/rpc-api/listing/store/get-cookie?id=%d", id)

	body, err := c.makeAPIRequestWithURL("GET", url)
	if err != nil {
		return "", err
	}

	var result APIResponse
	result.Data = new(string)

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		return "", err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil {
		return "", fmt.Errorf("Cookie数据为空")
	}

	// 安全的类型断言
	cookieData, ok := result.Data.(*string)
	if !ok {
		return "", fmt.Errorf("Cookie数据类型转换失败")
	}

	return *cookieData, nil
}

// UpdateStoreId 修改店铺的StoreID
func (c *StoreClient) UpdateStoreId(req *api.StoreIdUpdateReqDTO) (bool, error) {
	params := map[string]any{
		"id":      req.ID,
		"storeId": req.StoreID,
	}

	body, err := c.makeAPIRequest("PUT", "/rpc-api/listing/store/update-store-id", params)
	if err != nil {
		return false, err
	}

	var result APIResponse
	result.Data = new(bool)

	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("解析响应失败: %w", err)
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	// 安全的类型断言
	success, ok := result.Data.(*bool)
	if !ok {
		return false, fmt.Errorf("更新店铺ID响应数据类型转换失败")
	}

	return *success, nil
}

// UpdateStoreStatus 更新店铺状态
func (c *StoreClient) UpdateStoreStatus(req *api.StoreStatusUpdateReqDTO) (bool, error) {
	params := map[string]any{
		"id":     req.ID,
		"status": req.Status,
	}

	body, err := c.makeAPIRequest("PUT", "/rpc-api/listing/store/update-status", params)
	if err != nil {
		return false, err
	}

	var result APIResponse
	result.Data = new(bool)

	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("解析响应失败: %w", err)
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	// 安全的类型断言
	success, ok := result.Data.(*bool)
	if !ok {
		return false, fmt.Errorf("更新店铺状态响应数据类型转换失败")
	}

	return *success, nil
}

// CreateProductImportMapping 创建产品导入映射关系
func (c *StoreClient) CreateProductImportMapping(createReqDTO *api.ProductImportMappingCreateReqDTO) (int64, error) {
	params := map[string]any{
		"tenantId":     createReqDTO.TenantID,
		"importTaskId": createReqDTO.ImportTaskId,
		"storeId":      createReqDTO.StoreId,
		"platform":     createReqDTO.Platform,
		"region":       createReqDTO.Region,
		"productId":    createReqDTO.ProductId,
	}

	// 添加可选字段
	if createReqDTO.Sku != nil {
		params["sku"] = *createReqDTO.Sku
	}
	if createReqDTO.CostPrice != nil {
		params["costPrice"] = *createReqDTO.CostPrice
	}
	if createReqDTO.PlatformProductId != nil {
		params["platformProductId"] = *createReqDTO.PlatformProductId
	}
	if createReqDTO.ProfitRuleId != nil {
		params["profitRuleId"] = *createReqDTO.ProfitRuleId
	}
	if createReqDTO.SalePriceMultiplier != nil {
		params["salePriceMultiplier"] = *createReqDTO.SalePriceMultiplier
	}
	if createReqDTO.DiscountPriceMultiplier != nil {
		params["discountPriceMultiplier"] = *createReqDTO.DiscountPriceMultiplier
	}
	if createReqDTO.Status != nil {
		params["status"] = *createReqDTO.Status
	}
	if createReqDTO.Remark != nil {
		params["remark"] = *createReqDTO.Remark
	}
	if createReqDTO.ParentProductId != nil {
		params["parentProductId"] = *createReqDTO.ParentProductId
	}
	if createReqDTO.PlatformParentProductId != nil {
		params["platformParentProductId"] = *createReqDTO.PlatformParentProductId
	}
	if createReqDTO.FilterRuleId != nil {
		params["filterRuleId"] = *createReqDTO.FilterRuleId
	}
	if createReqDTO.FilterRuleRange != nil {
		params["filterRuleRange"] = *createReqDTO.FilterRuleRange
	}

	body, err := c.makeAPIRequest("POST", "/rpc-api/listing/product-import-mapping/create", params)
	if err != nil {
		return 0, err
	}

	var result APIResponse
	result.Data = new(int64)

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("解析响应失败: %w", err)
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		return 0, err
	}

	// 安全的类型断言
	id, ok := result.Data.(*int64)
	if !ok {
		return 0, fmt.Errorf("创建产品导入映射关系响应数据类型转换失败")
	}

	return *id, nil
}

// GetProductImportMappingByPlatformProductId 通过平台产品ID获取产品导入映射关系
func (c *StoreClient) GetProductImportMappingByPlatformProductId(req *api.ProductImportMappingGetReqDTO) (*api.ProductImportMappingRespDTO, error) {
	url := fmt.Sprintf("/rpc-api/listing/product-import-mapping/get?platformProductId=%s", req.PlatformProductId)

	body, err := c.makeAPIRequestWithURL("GET", url)
	if err != nil {
		return nil, err
	}

	var result APIResponse
	result.Data = &api.ProductImportMappingRespDTO{}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil {
		return nil, fmt.Errorf("产品导入映射关系数据为空: 可能不存在对应的映射关系")
	}

	// 安全的类型断言
	mapping, ok := result.Data.(*api.ProductImportMappingRespDTO)
	if !ok {
		return nil, fmt.Errorf("产品导入映射关系数据类型转换失败")
	}

	return mapping, nil
}

// DeleteStoreCookie 通过店铺ID删除用户Cookie
func (c *StoreClient) DeleteStoreCookie(id int64) (bool, error) {
	url := fmt.Sprintf("/rpc-api/listing/store/delete-cookie?id=%d", id)

	body, err := c.makeAPIRequestWithURL("PUT", url)
	if err != nil {
		return false, err
	}

	var result APIResponse
	result.Data = new(bool)

	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("解析响应失败: %w", err)
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	// 安全的类型断言
	success, ok := result.Data.(*bool)
	if !ok {
		return false, fmt.Errorf("删除Cookie响应数据类型转换失败")
	}

	return *success, nil
}

// SetStorePauseStatus 设置店铺任务暂停状态
func (c *StoreClient) SetStorePauseStatus(id int64, pause bool) (bool, error) {
	url := fmt.Sprintf("/rpc-api/listing/store/set-pause-status?id=%d&pause=%t", id, pause)

	body, err := c.makeAPIRequestWithURL("PUT", url)
	if err != nil {
		return false, err
	}

	var result APIResponse
	result.Data = new(bool)

	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("解析响应失败: %w", err)
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	// 安全的类型断言
	success, ok := result.Data.(*bool)
	if !ok {
		return false, fmt.Errorf("设置店铺暂停状态响应数据类型转换失败")
	}

	return *success, nil
}
