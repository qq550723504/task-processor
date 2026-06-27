package local

import (
	"context"
	"fmt"

	api "task-processor/internal/listingadmin"
)

type localStoreAPI struct {
	provider       *LocalDataProvider
	cookieProvider SheinCookieProvider
}

func (a localStoreAPI) GetStore(id int64) (*api.StoreRespDTO, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("store local data provider is not configured")
	}
	return a.provider.GetStore(id)
}

func (a localStoreAPI) PageStores(req *api.StorePageReqDTO) (*api.PageResult[*api.StoreRespDTO], error) {
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
	if a.provider == nil || req == nil {
		return false, fmt.Errorf("store local data provider is not configured")
	}
	return a.provider.UpdateStoreID(req.ID, req.StoreID)
}

func (a localStoreAPI) UpdateStoreStatus(req *api.StoreStatusUpdateReqDTO) (bool, error) {
	if a.provider == nil || req == nil {
		return false, fmt.Errorf("store local data provider is not configured")
	}
	return a.provider.UpdateStoreStatus(req.ID, req.Status, req.Remark)
}

func (a localStoreAPI) DeleteStoreCookie(id int64) (bool, error) {
	if a.provider != nil {
		return a.provider.DeleteStoreCookie(id)
	}
	if a.cookieProvider != nil {
		return a.cookieProvider.DeleteCookie(context.Background(), id)
	}
	return false, fmt.Errorf("store local cookie provider is not configured")
}

func (a localStoreAPI) SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error) {
	if a.provider == nil {
		return false, fmt.Errorf("store local data provider is not configured")
	}
	return a.provider.SetStorePauseStatus(id, pause, pauseType)
}

func (a localStoreAPI) GetStorePauseStatus(id int64) (bool, error) {
	if a.provider == nil {
		return false, fmt.Errorf("store local data provider is not configured")
	}
	return a.provider.GetStorePauseStatus(id)
}

func (a localStoreAPI) GetStorePauseStatusDetail(id int64) (*api.StorePauseStatusRespDTO, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("store local data provider is not configured")
	}
	return a.provider.GetStorePauseStatusDetail(id)
}

type localProductDataAPI struct {
	provider *LocalDataProvider
	storeID  int64
}

func (a localProductDataAPI) BatchCreateOrUpdate(req *api.ProductDataBatchSaveReqDTO) (int, error) {
	if a.provider == nil {
		return 0, fmt.Errorf("product data local provider is not configured")
	}
	return a.provider.BatchCreateOrUpdateProductData(req)
}

func (a localProductDataAPI) ListByStore(platform string, tenantID, storeID int64, shelfStatus *int) ([]*api.ProductDataDTO, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("product data local provider is not configured")
	}
	if storeID == 0 {
		storeID = a.storeID
	}
	return a.provider.ListProductDataByStore(platform, tenantID, storeID, shelfStatus)
}

func (a localProductDataAPI) BatchUpdateAttributes(req *api.ProductDataBatchUpdateAttributesReqDTO) (int, error) {
	if a.provider == nil {
		return 0, fmt.Errorf("product data local provider is not configured")
	}
	return a.provider.BatchUpdateProductAttributes(req)
}

func (a localProductDataAPI) PageProductDataByStore(req *api.ProductDataListByStorePageReqDTO) (*api.PageResult[*api.ProductDataRespDTO], error) {
	if a.provider == nil {
		return nil, fmt.Errorf("product data local provider is not configured")
	}
	return a.provider.PageProductDataByStore(req)
}

type localProductImportMappingAPI struct {
	provider *LocalDataProvider
}

func (a localProductImportMappingAPI) CreateProductImportMapping(req *api.ProductImportMappingCreateReqDTO) (int64, error) {
	if a.provider == nil {
		return 0, fmt.Errorf("product import mapping local provider is not configured")
	}
	return a.provider.CreateProductImportMapping(req)
}

func (a localProductImportMappingAPI) GetProductImportMappingByPlatformProductId(req *api.ProductImportMappingGetReqDTO) (*api.ProductImportMappingRespDTO, error) {
	if a.provider == nil || req == nil {
		return nil, fmt.Errorf("product import mapping local provider is not configured")
	}
	mapping, _, err := a.provider.GetProductImportMappingByPlatformProductID(req.PlatformProductId)
	return mapping, err
}

func (a localProductImportMappingAPI) CheckProductExists(req *api.ProductImportMappingCheckReqDTO) (bool, error) {
	if a.provider == nil {
		return false, fmt.Errorf("product import mapping local provider is not configured")
	}
	exists, _, err := a.provider.CheckProductExists(req)
	return exists, err
}

func (a localProductImportMappingAPI) GetProductImportMappingBySku(req *api.ProductImportMappingGetBySkuReqDTO) (*api.ProductImportMappingRespDTO, error) {
	if a.provider == nil || req == nil {
		return nil, fmt.Errorf("product import mapping local provider is not configured")
	}
	mapping, _, err := a.provider.GetProductImportMappingBySKU(req.Sku, req.StoreId)
	return mapping, err
}

func (a localProductImportMappingAPI) GetProductImportMappingByTaskAndSku(importTaskID int64, sku string) (*api.ProductImportMappingRespDTO, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("product import mapping local provider is not configured")
	}
	mapping, _, err := a.provider.GetProductImportMappingByTaskAndSKU(importTaskID, sku)
	return mapping, err
}

func (a localProductImportMappingAPI) GetProductImportMappingByPlatformProductIdAndStore(req *api.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO) (*api.ProductImportMappingRespDTO, error) {
	if a.provider == nil || req == nil {
		return nil, fmt.Errorf("product import mapping local provider is not configured")
	}
	mapping, _, err := a.provider.GetProductImportMappingByPlatformProductIDAndStore(req.PlatformProductId, req.StoreId)
	return mapping, err
}

func (a localProductImportMappingAPI) UpdateProductImportMapping(req *api.ProductImportMappingCreateReqDTO) error {
	if a.provider == nil {
		return fmt.Errorf("product import mapping local provider is not configured")
	}
	_, err := a.provider.UpdateProductImportMapping(req)
	return err
}

type localInventoryRecordAPI struct {
	provider *LocalDataProvider
}

func (a localInventoryRecordAPI) CreateInventoryRecord(req *api.InventoryRecordCreateReqDTO) (int64, error) {
	if a.provider == nil {
		return 0, fmt.Errorf("inventory record local provider is not configured")
	}
	return a.provider.CreateInventoryRecord(req)
}

func (a localInventoryRecordAPI) GetLatestInventoryRecord(platform, productID, region string) (*api.InventoryRecordRespDTO, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("inventory record local provider is not configured")
	}
	record, _, err := a.provider.GetLatestInventoryRecord(platform, productID, region)
	return record, err
}
