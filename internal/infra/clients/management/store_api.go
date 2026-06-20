package management

import (
	"context"
	"fmt"
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
		return m.localDataProvider.GetStore(id)
	}
	return nil, fmt.Errorf("store local data provider is not configured")
}

// PageStores 分页查询店铺列表
func (m *StoreAPIClient) PageStores(req *api.StorePageReqDTO) (*api.PageResult[*api.StoreRespDTO], error) {
	if m.localDataProvider != nil {
		return m.localDataProvider.PageStores(req)
	}
	return nil, fmt.Errorf("store local data provider is not configured")
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
	return "", fmt.Errorf("shein cookie provider is not configured")
}

// UpdateStoreId 修改店铺的StoreID
func (m *StoreAPIClient) UpdateStoreId(req *api.StoreIdUpdateReqDTO) (bool, error) {
	if m.localDataProvider != nil {
		return m.localDataProvider.UpdateStoreID(req.ID, req.StoreID)
	}
	return false, fmt.Errorf("store local data provider is not configured")
}

// UpdateStoreStatus 更新店铺状态
func (m *StoreAPIClient) UpdateStoreStatus(req *api.StoreStatusUpdateReqDTO) (bool, error) {
	if m.localDataProvider != nil {
		return m.localDataProvider.UpdateStoreStatus(req.ID, req.Status, req.Remark)
	}
	return false, fmt.Errorf("store local data provider is not configured")
}

// DeleteStoreCookie 通过店铺ID删除用户Cookie
func (m *StoreAPIClient) DeleteStoreCookie(id int64) (bool, error) {
	if m.localDataProvider != nil {
		return m.localDataProvider.DeleteStoreCookie(id)
	}
	if m.sheinCookieProvider != nil {
		return m.sheinCookieProvider.DeleteCookie(context.Background(), id)
	}
	return false, fmt.Errorf("store local cookie provider is not configured")
}

// SetStorePauseStatus 设置店铺任务暂停状态
func (m *StoreAPIClient) SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error) {
	if m.localDataProvider != nil {
		return m.localDataProvider.SetStorePauseStatus(id, pause, pauseType)
	}
	return false, fmt.Errorf("store local data provider is not configured")
}

// GetStorePauseStatus 获取店铺任务暂停状态
func (m *StoreAPIClient) GetStorePauseStatus(id int64) (bool, error) {
	if m.localDataProvider != nil {
		return m.localDataProvider.GetStorePauseStatus(id)
	}
	return false, fmt.Errorf("store local data provider is not configured")
}

// GetStorePauseStatusDetail 获取店铺任务暂停状态详情
func (m *StoreAPIClient) GetStorePauseStatusDetail(id int64) (*api.StorePauseStatusRespDTO, error) {
	if m.localDataProvider != nil {
		return m.localDataProvider.GetStorePauseStatusDetail(id)
	}
	return nil, fmt.Errorf("store local data provider is not configured")
}
