package impl

import (
	"fmt"
	"net/http"
	"task-processor/internal/common/management/api"
)

type StoreAPIClientImpl struct {
	*ManagementAPIClientImpl
}

// GetStore 通过店铺ID获取店铺信息
func (m *StoreAPIClientImpl) GetStore(id int64) (*api.StoreRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/get?id=%d", m.baseURL, id)

	var result APIResponse
	result.Data = &api.StoreRespDTO{}

	err := m.apiRequest(http.MethodGet, url, nil, &result)
	if err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil {
		return nil, NewNonRetryableError("店铺信息数据为空: 店铺可能已被删除", nil)
	}

	// 安全的类型断言
	store, ok := result.Data.(*api.StoreRespDTO)
	if !ok {
		return nil, fmt.Errorf("店铺信息数据类型转换失败")
	}

	return store, nil
}

// GetStoreCookie 通过店铺ID获取用户Cookie
func (m *StoreAPIClientImpl) GetStoreCookie(id int64) (string, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/get-cookie?id=%d", m.baseURL, id)

	var result APIResponse
	result.Data = new(string)

	err := m.apiRequest(http.MethodGet, url, nil, &result)
	if err != nil {
		return "", err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
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

	// 直接返回 JSON 格式的 cookie，不进行转换
	// ClientManager 会在创建客户端时进行解析和转换
	return *cookieData, nil
}

// UpdateStoreId 修改店铺的StoreID
func (m *StoreAPIClientImpl) UpdateStoreId(req *api.StoreIdUpdateReqDTO) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/update-store-id", m.baseURL)

	var result APIResponse
	result.Data = new(bool)

	err := m.apiRequest(http.MethodPut, url, req, &result)
	if err != nil {
		return false, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil {
		return false, fmt.Errorf("更新店铺ID响应数据为空")
	}

	// 安全的类型断言
	success, ok := result.Data.(*bool)
	if !ok {
		return false, fmt.Errorf("更新店铺ID响应数据类型转换失败")
	}

	return *success, nil
}

// UpdateStoreStatus 更新店铺状态
func (m *StoreAPIClientImpl) UpdateStoreStatus(req *api.StoreStatusUpdateReqDTO) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/update-status", m.baseURL)

	var result APIResponse
	result.Data = new(bool)

	err := m.apiRequest(http.MethodPut, url, req, &result)
	if err != nil {
		return false, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil {
		return false, fmt.Errorf("更新店铺状态响应数据为空")
	}

	// 安全的类型断言
	success, ok := result.Data.(*bool)
	if !ok {
		return false, fmt.Errorf("更新店铺状态响应数据类型转换失败")
	}

	return *success, nil
}

// DeleteStoreCookie 通过店铺ID删除用户Cookie
func (m *StoreAPIClientImpl) DeleteStoreCookie(id int64) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/delete-cookie?id=%d", m.baseURL, id)

	var result APIResponse
	result.Data = new(bool)

	err := m.apiRequest(http.MethodPut, url, nil, &result)
	if err != nil {
		return false, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil {
		return false, fmt.Errorf("删除Cookie响应数据为空")
	}

	// 安全的类型断言
	success, ok := result.Data.(*bool)
	if !ok {
		return false, fmt.Errorf("删除Cookie响应数据类型转换失败")
	}

	return *success, nil
}

// SetStorePauseStatus 设置店铺任务暂停状态
// pauseType: auth_expired(认证过期) 或 quota_limit(配额限制)，空字符串时使用默认值 quota_limit
func (m *StoreAPIClientImpl) SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/set-pause-status?id=%d&pause=%t", m.baseURL, id, pause)

	// 如果提供了 pauseType，添加到 URL 参数中
	if pauseType != "" {
		url = fmt.Sprintf("%s&pauseType=%s", url, pauseType)
	}

	var result APIResponse
	result.Data = new(bool)

	err := m.apiRequest(http.MethodPut, url, nil, &result)
	if err != nil {
		return false, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil {
		return false, fmt.Errorf("设置店铺暂停状态响应数据为空")
	}

	// 安全的类型断言
	success, ok := result.Data.(*bool)
	if !ok {
		return false, fmt.Errorf("设置店铺暂停状态响应数据类型转换失败")
	}

	return *success, nil
}

// GetStorePauseStatus 获取店铺任务暂停状态
func (m *StoreAPIClientImpl) GetStorePauseStatus(id int64) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/get-pause-status?id=%d", m.baseURL, id)

	var result APIResponse
	result.Data = new(bool)

	err := m.apiRequest(http.MethodGet, url, nil, &result)
	if err != nil {
		return false, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil {
		return false, fmt.Errorf("获取店铺暂停状态响应数据为空")
	}

	// 安全的类型断言
	isPaused, ok := result.Data.(*bool)
	if !ok {
		return false, fmt.Errorf("获取店铺暂停状态响应数据类型转换失败")
	}

	return *isPaused, nil
}
