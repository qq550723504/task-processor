package management

import (
	"context"
	"fmt"
	"net/http"
	"task-processor/internal/infra/clients/management/api"
)

// StoreAPIClient 店铺API客户端实现
type StoreAPIClient struct {
	*ManagementAPIClient
	localDataProvider   *LocalDataProvider
	sheinCookieProvider SheinCookieProvider
}

// GetStore 通过店铺ID获取店铺信息
func (m *StoreAPIClient) GetStore(id int64) (*api.StoreRespDTO, error) {
	if m.localDataProvider != nil {
		if store, err := m.localDataProvider.GetStore(id); err != nil || store != nil {
			return store, err
		}
	}
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

// PageStores 分页查询店铺列表
func (m *StoreAPIClient) PageStores(req *api.StorePageReqDTO) (*api.PageResult[*api.StoreRespDTO], error) {
	if m.localDataProvider != nil {
		if page, err := m.localDataProvider.PageStores(req); err != nil || page != nil {
			return page, err
		}
	}
	url := fmt.Sprintf("%s/rpc-api/listing/store/page", m.baseURL)

	reqBody := map[string]any{
		"pageNo":   req.PageNo,
		"pageSize": req.PageSize,
	}
	if req.Platform != "" {
		reqBody["platform"] = req.Platform
	}
	if req.TenantID > 0 {
		reqBody["tenantId"] = req.TenantID
	}
	if req.EnableAutoPrice != nil {
		reqBody["enableAutoPrice"] = *req.EnableAutoPrice
	}

	var result api.CommonResult[api.PageResult[*api.StoreRespDTO]]
	if err := m.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, fmt.Errorf("分页查询店铺失败: %w", err)
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("分页查询店铺失败: %s", result.Msg)
	}

	return &result.Data, nil
}

// GetStoreCookie 通过店铺ID获取用户Cookie
func (m *StoreAPIClient) GetStoreCookie(id int64) (string, error) {
	if m.sheinCookieProvider != nil {
		result, err := m.sheinCookieProvider.GetCookie(context.Background(), id)
		if err != nil {
			return "", err
		}
		if result != nil {
			cookie := result.CookieJSON
			if cookie != "" {
				return cookie, nil
			}
		}
	}
	url := fmt.Sprintf("%s/rpc-api/listing/store/get-cookie?id=%d", m.baseURL, id)
	cookie, err := getTypedResult[string](m.ManagementAPIClient, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("Cookie数据为空或类型错误: %w", err)
	}
	return cookie, nil
}

// UpdateStoreId 修改店铺的StoreID
func (m *StoreAPIClient) UpdateStoreId(req *api.StoreIdUpdateReqDTO) (bool, error) {
	if m.localDataProvider != nil {
		if ok, err := m.localDataProvider.UpdateStoreID(req.ID, req.StoreID); err != nil || ok {
			return ok, err
		}
	}
	url := fmt.Sprintf("%s/rpc-api/listing/store/update-store-id", m.baseURL)
	return getTypedResult[bool](m.ManagementAPIClient, http.MethodPut, url, req)
}

// UpdateStoreStatus 更新店铺状态
func (m *StoreAPIClient) UpdateStoreStatus(req *api.StoreStatusUpdateReqDTO) (bool, error) {
	if m.localDataProvider != nil {
		if ok, err := m.localDataProvider.UpdateStoreStatus(req.ID, req.Status, ""); err != nil || ok {
			return ok, err
		}
	}
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
	if m.localDataProvider != nil {
		if ok, err := m.localDataProvider.SetStorePauseStatus(id, pause, pauseType); err != nil || ok {
			return ok, err
		}
	}
	url := fmt.Sprintf("%s/rpc-api/listing/store/set-pause-status?id=%d&pause=%t", m.baseURL, id, pause)
	if pauseType != "" {
		url = fmt.Sprintf("%s&pauseType=%s", url, pauseType)
	}
	return getTypedResult[bool](m.ManagementAPIClient, http.MethodPut, url, nil)
}

// GetStorePauseStatus 获取店铺任务暂停状态
func (m *StoreAPIClient) GetStorePauseStatus(id int64) (bool, error) {
	if m.localDataProvider != nil {
		if paused, err := m.localDataProvider.GetStorePauseStatus(id); err != nil || paused {
			return paused, err
		}
	}
	url := fmt.Sprintf("%s/rpc-api/listing/store/get-pause-status?id=%d", m.baseURL, id)
	return getTypedResult[bool](m.ManagementAPIClient, http.MethodGet, url, nil)
}

// GetStorePauseStatusDetail 获取店铺任务暂停状态详情
func (m *StoreAPIClient) GetStorePauseStatusDetail(id int64) (*api.StorePauseStatusRespDTO, error) {
	if m.localDataProvider != nil {
		if detail, err := m.localDataProvider.GetStorePauseStatusDetail(id); err != nil || detail != nil {
			return detail, err
		}
	}
	url := fmt.Sprintf("%s/rpc-api/listing/store/get-pause-status-detail?id=%d", m.baseURL, id)
	return getTypedResult[*api.StorePauseStatusRespDTO](m.ManagementAPIClient, http.MethodGet, url, nil)
}
