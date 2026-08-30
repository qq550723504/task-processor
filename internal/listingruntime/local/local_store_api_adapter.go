package local

import (
	"context"
	"fmt"

	api "task-processor/internal/listingadmin"
)

type localStoreAPI struct {
	storeAPI       api.StoreAPI
	storeState     *localStoreRuntimeState
	provider       *LocalDataProvider
	cookieProvider SheinCookieProvider
}

func (a localStoreAPI) GetStore(id int64) (*api.StoreRespDTO, error) {
	if a.storeAPI != nil {
		return a.storeAPI.GetStore(id)
	}
	if a.provider == nil {
		return nil, fmt.Errorf("store local data provider is not configured")
	}
	return a.provider.GetStore(id)
}

func (a localStoreAPI) PageStores(req *api.StorePageReqDTO) (*api.PageResult[*api.StoreRespDTO], error) {
	if a.storeAPI != nil {
		return a.storeAPI.PageStores(req)
	}
	if a.provider == nil {
		return nil, fmt.Errorf("store local data provider is not configured")
	}
	return a.provider.PageStores(req)
}

func (a localStoreAPI) GetStoreCookie(id int64) (string, error) {
	if a.cookieProvider == nil {
		return "", fmt.Errorf("shein cookie provider is not configured")
	}
	result, err := a.cookieProvider.GetCookie(context.Background(), id)
	if err != nil || result == nil || result.CookieJSON == "" {
		return "", err
	}
	return result.CookieJSON, nil
}

func (a localStoreAPI) UpdateStoreId(req *api.StoreIdUpdateReqDTO) (bool, error) {
	if a.storeAPI != nil {
		return a.storeAPI.UpdateStoreId(req)
	}
	if a.provider == nil || req == nil {
		return false, fmt.Errorf("store local data provider is not configured")
	}
	return a.provider.UpdateStoreID(req.ID, req.StoreID)
}

func (a localStoreAPI) UpdateStoreStatus(req *api.StoreStatusUpdateReqDTO) (bool, error) {
	if a.storeAPI != nil {
		return a.storeAPI.UpdateStoreStatus(req)
	}
	if a.provider == nil || req == nil {
		return false, fmt.Errorf("store local data provider is not configured")
	}
	return a.provider.UpdateStoreStatus(req.ID, req.Status, req.Remark)
}

func (a localStoreAPI) DeleteStoreCookie(id int64) (bool, error) {
	if a.storeState != nil {
		return a.storeState.DeleteStoreCookie(id)
	}
	if a.provider != nil {
		return a.provider.DeleteStoreCookie(id)
	}
	if a.cookieProvider != nil {
		return a.cookieProvider.DeleteCookie(context.Background(), id)
	}
	return false, fmt.Errorf("store local cookie provider is not configured")
}

func (a localStoreAPI) SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error) {
	if a.storeState != nil {
		return a.storeState.SetStorePauseStatus(id, pause, pauseType)
	}
	if a.provider == nil {
		return false, fmt.Errorf("store local data provider is not configured")
	}
	return a.provider.SetStorePauseStatus(id, pause, pauseType)
}

func (a localStoreAPI) GetStorePauseStatus(id int64) (bool, error) {
	if a.storeState != nil {
		return a.storeState.GetStorePauseStatus(id)
	}
	if a.provider == nil {
		return false, fmt.Errorf("store local data provider is not configured")
	}
	return a.provider.GetStorePauseStatus(id)
}

func (a localStoreAPI) GetStorePauseStatusDetail(id int64) (*api.StorePauseStatusRespDTO, error) {
	if a.storeState != nil {
		return a.storeState.GetStorePauseStatusDetail(id)
	}
	if a.provider == nil {
		return nil, fmt.Errorf("store local data provider is not configured")
	}
	return a.provider.GetStorePauseStatusDetail(id)
}
