package local

import (
	"fmt"

	api "task-processor/internal/listingadmin"
)

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
