package impl

import (
	"encoding/json"
	"fmt"
	"task-processor/common/management/api"
)

// RawJsonDataClient 原始JSON数据API客户端实现
type RawJsonDataClient struct {
	*BaseManagementClient
}

// NewRawJsonDataClient 创建原始JSON数据API客户端
func NewRawJsonDataClient(baseURL string) *RawJsonDataClient {
	return &RawJsonDataClient{
		BaseManagementClient: NewBaseManagementClient(baseURL),
	}
}

// GetRawJsonData 获取原始JSON数据
func (c *RawJsonDataClient) GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error) {
	params := map[string]any{
		"tenantId":   req.TenantID,
		"platform":   req.Platform,
		"productId":  req.ProductID,
		"region":     req.Region,
		"storeId":    req.StoreID,
		"categoryId": req.CategoryID,
		"creator":    req.Creator,
	}

	body, err := c.makeAPIRequest("POST", "/rpc-api/listing/raw-json-data/get", params)
	if err != nil {
		return nil, err
	}

	var result APIResponse
	result.Data = &api.RawJsonDataRespDTO{}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	if result.Data == nil {
		return nil, fmt.Errorf("原始JSON数据为空")
	}

	// 安全的类型断言
	rawData, ok := result.Data.(*api.RawJsonDataRespDTO)
	if !ok {
		return nil, fmt.Errorf("原始JSON数据类型转换失败")
	}

	return rawData, nil
}

// ConfirmProductVariants 确认产品变体数据
func (c *RawJsonDataClient) ConfirmProductVariants(req *api.ProductVariantConfirmationReqDTO) (bool, error) {
	params := map[string]any{
		"productId":  req.ProductID,
		"platform":   req.Platform,
		"region":     req.Region,
		"variantIds": req.VariantIds,
	}

	body, err := c.makeAPIRequest("POST", "/rpc-api/listing/import-task/confirm-variants", params)
	if err != nil {
		return false, err
	}

	var result APIResponse
	var confirmed bool
	result.Data = &confirmed

	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("解析响应失败: %w", err)
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	// 安全的类型断言
	if boolPtr, ok := result.Data.(*bool); ok {
		return *boolPtr, nil
	}

	return false, nil
}

// CreateRawJsonData 创建原始JSON数据（提交到服务器缓存）
func (c *RawJsonDataClient) CreateRawJsonData(req *api.RawJsonDataCreateReqDTO) (int64, error) {
	params := map[string]any{
		"tenantId":     req.TenantID,
		"storeId":      req.StoreID,
		"importTaskId": req.ImportTaskID,
		"platform":     req.Platform,
		"region":       req.Region,
		"categoryId":   req.CategoryID,
		"productId":    req.ProductID,
		"rawJsonData":  req.RawJsonData,
		"creator":      req.Creator,
	}

	body, err := c.makeAPIRequest("POST", "/rpc-api/listing/raw-json-data/create", params)
	if err != nil {
		return 0, fmt.Errorf("创建原始JSON数据失败: %w", err)
	}

	var result APIResponse
	var recordID int64
	result.Data = &recordID

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("解析响应失败: %w", err)
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		return 0, fmt.Errorf("处理API响应失败: %w", err)
	}

	// 安全的类型断言
	if idPtr, ok := result.Data.(*int64); ok {
		return *idPtr, nil
	}

	return 0, fmt.Errorf("无法解析返回的记录ID")
}
