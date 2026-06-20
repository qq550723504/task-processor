package sync

import (
	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	"task-processor/internal/product"
)

type ServiceRuntime interface {
	productSyncRuntime
	inventoryServiceFactoryRuntime
}

type runtimeSource interface {
	GetProductDataClient(storeID int64) *management.ProductDataAPIClient
	GetLocalStoreRepository() *listingadmin.GormStoreRepository
	GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository
	GetLocalProductDataRepository() listingadmin.ProductDataRepository
	GetStoreClient() *management.StoreAPIClient
	GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error)
	GetRawJsonDataAdapter() product.RawJsonDataClient
	GetInventoryRecordClient() *management.InventoryRecordAPIClient
}

type managementRuntime struct {
	source runtimeSource
}

func NewServiceRuntime(source runtimeSource) ServiceRuntime {
	if source == nil {
		return nil
	}
	return managementRuntime{source: source}
}

func (r managementRuntime) GetProductDataClient(storeID int64) managementapi.ProductDataAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetProductDataClient(storeID)
}

func (r managementRuntime) GetLocalStoreRepository() *listingadmin.GormStoreRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalStoreRepository()
}

func (r managementRuntime) GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalProductImportMappingRepository()
}

func (r managementRuntime) GetLocalProductDataRepository() listingadmin.ProductDataRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalProductDataRepository()
}

func (r managementRuntime) GetStoreAPI() managementapi.StoreAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetStoreClient()
}

func (r managementRuntime) GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.GetRuntimeOperationStrategy(storeID)
}

func (r managementRuntime) GetRawJsonDataAdapter() product.RawJsonDataClient {
	if r.source == nil {
		return nil
	}
	return r.source.GetRawJsonDataAdapter()
}

func (r managementRuntime) GetInventoryRecordAPI() managementapi.InventoryRecordAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetInventoryRecordClient()
}
