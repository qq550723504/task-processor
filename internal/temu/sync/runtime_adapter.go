package sync

import (
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	"task-processor/internal/product"
)

type ServiceRuntime interface {
	productSyncRuntime
	inventoryServiceFactoryRuntime
}

type runtimeSource interface {
	GetProductDataClient(storeID int64) listingadmin.ProductDataAPI
	GetLocalStoreRepository() *listingadmin.GormStoreRepository
	GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository
	GetLocalProductDataRepository() listingadmin.ProductDataRepository
	GetStoreAPI() listingadmin.StoreAPI
	GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error)
	GetRawJsonDataAdapter() product.RawJsonDataClient
	GetInventoryRecordAPI() listingadmin.InventoryRecordAPI
}

type serviceRuntime struct {
	source runtimeSource
}

func NewServiceRuntime(source runtimeSource) ServiceRuntime {
	if source == nil {
		return nil
	}
	return serviceRuntime{source: source}
}

func (r serviceRuntime) GetProductDataClient(storeID int64) listingadmin.ProductDataAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetProductDataClient(storeID)
}

func (r serviceRuntime) GetLocalStoreRepository() *listingadmin.GormStoreRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalStoreRepository()
}

func (r serviceRuntime) GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalProductImportMappingRepository()
}

func (r serviceRuntime) GetLocalProductDataRepository() listingadmin.ProductDataRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalProductDataRepository()
}

func (r serviceRuntime) GetStoreAPI() listingadmin.StoreAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetStoreAPI()
}

func (r serviceRuntime) GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.GetRuntimeOperationStrategy(storeID)
}

func (r serviceRuntime) GetRawJsonDataAdapter() product.RawJsonDataClient {
	if r.source == nil {
		return nil
	}
	return r.source.GetRawJsonDataAdapter()
}

func (r serviceRuntime) GetInventoryRecordAPI() listingadmin.InventoryRecordAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetInventoryRecordAPI()
}
