package management

import (
	"fmt"
	"net/http"
	"task-processor/internal/infra/clients/management/api"
)

// InventoryRecordAPIClient 库存记录API客户端实现
type InventoryRecordAPIClient struct {
	*ManagementAPIClient
}

// CreateInventoryRecord 创建库存记录
func (m *InventoryRecordAPIClient) CreateInventoryRecord(req *api.InventoryRecordCreateReqDTO) (int64, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/inventory-record/create", m.baseURL)

	var result APIResponse
	var recordID int64
	result.Data = &recordID

	if err := m.apiRequest(http.MethodPost, url, req, &result); err != nil {
		return 0, fmt.Errorf("创建库存记录失败: %w", err)
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return 0, fmt.Errorf("处理API响应失败: %w", err)
	}

	if idPtr, ok := result.Data.(*int64); ok {
		return *idPtr, nil
	}

	return 0, fmt.Errorf("无法解析返回的记录ID")
}

// GetLatestInventoryRecord 获取最新的库存记录
func (m *InventoryRecordAPIClient) GetLatestInventoryRecord(platform, productId, region string) (*api.InventoryRecordRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/inventory-record/get-latest", m.baseURL)

	params := map[string]any{
		"platform":  platform,
		"productId": productId,
		"region":    region,
	}

	var result APIResponse
	result.Data = &api.InventoryRecordRespDTO{}

	if err := m.apiRequest(http.MethodGet, url, params, &result); err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	if result.Data == nil {
		return nil, nil
	}

	record, ok := result.Data.(*api.InventoryRecordRespDTO)
	if !ok {
		return nil, fmt.Errorf("库存记录数据类型转换失败")
	}

	return record, nil
}
