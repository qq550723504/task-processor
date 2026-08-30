package local

import (
	"fmt"

	api "task-processor/internal/listingadmin"
)

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
