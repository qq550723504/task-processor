package management

import (
	"fmt"
	"net/http"
	"task-processor/internal/infra/clients/management/api"
)

// StoreAPIClient 店铺API客户端实现
type StoreAPIClient struct {
	*ManagementAPIClient
}

// GetStore 通过店铺ID获取店铺信息
func (m *StoreAPIClient) GetStore(id int64) (*api.StoreRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/get?id=%d", m.baseURL, id)

	var result APIResponse
	result.Data = &api.StoreRespDTO{}

	if err := m.apiRequest(http.MethodGet, url, nil, &result); err != nil {
		return nil, err
	}
	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}
	if result.Data == nil {
		return nil, NewNonRetryableError("店铺信息数据为空: 店铺可能已被删除", nil)
	}
	store, ok := result.Data.(*api.StoreRespDTO)
	if !ok {
		return nil, fmt.Errorf("店铺信息数据类型转换失败")
	}
	return store, nil
}

// GetStoreCookie 通过店铺ID获取用户Cookie
func (m *StoreAPIClient) GetStoreCookie(id int64) (string, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/get-cookie?id=%d", m.baseURL, id)
	cookie, err := getTypedResult[string](m.ManagementAPIClient, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("Cookie数据为空或类型错误: %w", err)
	}
	return cookie, nil
}

// UpdateStoreId 修改店铺的StoreID
func (m *StoreAPIClient) UpdateStoreId(req *api.StoreIdUpdateReqDTO) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/update-store-id", m.baseURL)
	return getTypedResult[bool](m.ManagementAPIClient, http.MethodPut, url, req)
}

// UpdateStoreStatus 更新店铺状态
func (m *StoreAPIClient) UpdateStoreStatus(req *api.StoreStatusUpdateReqDTO) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/update-status", m.baseURL)
	return getTypedResult[bool](m.ManagementAPIClient, http.MethodPut, url, req)
}

// DeleteStoreCookie 通过店铺ID删除用户Cookie
func (m *StoreAPIClient) DeleteStoreCookie(id int64) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/delete-cookie?id=%d", m.baseURL, id)
	return getTypedResult[bool](m.ManagementAPIClient, http.MethodPut, url, nil)
}

// SetStorePauseStatus 设置店铺任务暂停状态
func (m *StoreAPIClient) SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/set-pause-status?id=%d&pause=%t", m.baseURL, id, pause)
	if pauseType != "" {
		url = fmt.Sprintf("%s&pauseType=%s", url, pauseType)
	}
	return getTypedResult[bool](m.ManagementAPIClient, http.MethodPut, url, nil)
}

// GetStorePauseStatus 获取店铺任务暂停状态
func (m *StoreAPIClient) GetStorePauseStatus(id int64) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/get-pause-status?id=%d", m.baseURL, id)
	return getTypedResult[bool](m.ManagementAPIClient, http.MethodGet, url, nil)
}
