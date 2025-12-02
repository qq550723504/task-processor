package impl

import (
	"fmt"
	"net/http"
	"task-processor/common/management/api"
)

// InventoryRecordAPIClientImpl 库存记录API客户端实现
type InventoryRecordAPIClientImpl struct {
	*ManagementAPIClientImpl
}

// CreateInventoryRecord 创建库存记录
func (m *InventoryRecordAPIClientImpl) CreateInventoryRecord(req *api.InventoryRecordCreateReqDTO) (int64, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/inventory-record/create", m.baseURL)

	var result APIResponse
	var recordID int64
	result.Data = &recordID

	// 使用POST请求并将参数作为请求体传递
	err := m.apiRequest(http.MethodPost, url, req, &result)
	if err != nil {
		return 0, fmt.Errorf("创建库存记录失败: %w", err)
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return 0, fmt.Errorf("处理API响应失败: %w", err)
	}

	// 安全的类型断言
	if idPtr, ok := result.Data.(*int64); ok {
		return *idPtr, nil
	}

	return 0, fmt.Errorf("无法解析返回的记录ID")
}

// GetLatestInventoryRecord 获取最新的库存记录
func (m *InventoryRecordAPIClientImpl) GetLatestInventoryRecord(platform, productId, region string) (*api.InventoryRecordRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/inventory-record/latest", m.baseURL)

	// 构建查询参数
	params := map[string]interface{}{
		"platform":  platform,
		"productId": productId,
		"region":    region,
	}

	var result APIResponse
	result.Data = &api.InventoryRecordRespDTO{}

	err := m.apiRequest(http.MethodGet, url, params, &result)
	if err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	if result.Data == nil {
		return nil, nil // 没有找到记录
	}

	// 安全的类型断言
	record, ok := result.Data.(*api.InventoryRecordRespDTO)
	if !ok {
		return nil, fmt.Errorf("库存记录数据类型转换失败")
	}

	return record, nil
}
