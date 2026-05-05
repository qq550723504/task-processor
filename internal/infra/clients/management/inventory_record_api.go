package management

import (
	"fmt"
	"net/http"
	"task-processor/internal/infra/clients/management/api"
)

// InventoryRecordAPIClient 库存记录API客户端实现
type InventoryRecordAPIClient struct {
	*ManagementAPIClient
	localDataProvider *LocalDataProvider
}

// CreateInventoryRecord 创建库存记录
func (m *InventoryRecordAPIClient) CreateInventoryRecord(req *api.InventoryRecordCreateReqDTO) (int64, error) {
	if m.localDataProvider != nil && m.localDataProvider.HasDB() {
		id, err := m.localDataProvider.CreateInventoryRecord(req)
		if err != nil || id > 0 {
			return id, err
		}
	}
	url := fmt.Sprintf("%s/rpc-api/listing/inventory-record/create", m.baseURL)
	id, err := getTypedResult[int64](m.ManagementAPIClient, http.MethodPost, url, req)
	if err != nil {
		return 0, fmt.Errorf("创建库存记录失败: %w", err)
	}
	return id, nil
}

// GetLatestInventoryRecord 获取最新的库存记录
func (m *InventoryRecordAPIClient) GetLatestInventoryRecord(platform, productId, region string) (*api.InventoryRecordRespDTO, error) {
	if m.localDataProvider != nil && m.localDataProvider.HasDB() {
		if record, found, err := m.localDataProvider.GetLatestInventoryRecord(platform, productId, region); err != nil || found {
			return record, err
		}
	}
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
