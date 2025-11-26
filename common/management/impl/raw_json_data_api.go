package impl

import (
	"fmt"
	"net/http"
	"task-processor/common/management/api"
)

// RawJsonDataAPIClientImpl 原始JSON数据API客户端实现
type RawJsonDataAPIClientImpl struct {
	*ManagementAPIClientImpl
}

// GetRawJsonData 获取原始JSON数据
func (m *RawJsonDataAPIClientImpl) GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/raw-json-data/get", m.baseURL)

	var result APIResponse
	result.Data = &api.RawJsonDataRespDTO{}

	// 使用POST请求并将参数作为请求体传递
	err := m.apiRequest(http.MethodPost, url, req, &result)
	if err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
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
func (m *RawJsonDataAPIClientImpl) ConfirmProductVariants(req *api.ProductVariantConfirmationReqDTO) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/import-task/confirm-variants", m.baseURL)

	var result APIResponse
	var confirmed bool
	result.Data = &confirmed

	// 使用POST请求并将参数作为请求体传递
	err := m.apiRequest(http.MethodPost, url, req, &result)
	if err != nil {
		return false, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	// 安全的类型断言
	if boolPtr, ok := result.Data.(*bool); ok {
		return *boolPtr, nil
	}

	return false, nil
}

// CreateRawJsonData 创建原始JSON数据（提交到服务器缓存）
func (m *RawJsonDataAPIClientImpl) CreateRawJsonData(req *api.RawJsonDataCreateReqDTO) (int64, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/raw-json-data/create", m.baseURL)

	var result APIResponse
	var recordID int64
	result.Data = &recordID

	// 使用POST请求并将参数作为请求体传递
	err := m.apiRequest(http.MethodPost, url, req, &result)
	if err != nil {
		return 0, fmt.Errorf("创建原始JSON数据失败: %w", err)
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
