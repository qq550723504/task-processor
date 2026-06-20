package scheduler

import (
	"context"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	platformtask "task-processor/internal/platformtask"
	managementapi "task-processor/internal/ports/managementapi"
	domainproduct "task-processor/internal/product"
	temuclient "task-processor/internal/temu/api/client"
	temupricingruntime "task-processor/internal/temu/pricing"
	schedulerservice "task-processor/internal/temu/sync"
)

type runtime interface {
	temuclient.StoreRuntime
	platformtask.AutoPricingStoreConfigProvider
	GetRawJsonDataAdapter() domainproduct.RawJsonDataClient
	GetStoreAPI() managementapi.StoreAPI
	GetPricingRuleClient() managementapi.PricingRuleAPI
	GetProductImportMappingAPI() managementapi.ProductImportMappingAPI
	GetInventoryRecordAPI() managementapi.InventoryRecordAPI
	GetLocalProductDataRepository() listingadmin.ProductDataRepository
	PricingRuntime() temupricingruntime.ManagementRuntime
	SyncRuntime() schedulerservice.ServiceRuntime
}

type ManagementRuntime = runtime

type runtimeSource interface {
	temuclient.StoreRuntime
	platformtask.AutoPricingStoreConfigProvider
	GetRawJsonDataAdapter() domainproduct.RawJsonDataClient
	GetPricingRuleClient() managementapi.PricingRuleAPI
	GetProductImportMappingAPI() managementapi.ProductImportMappingAPI
	GetInventoryRecordAPI() managementapi.InventoryRecordAPI
	GetProductDataClient(storeID int64) managementapi.ProductDataAPI
	GetLocalStoreRepository() *listingadmin.GormStoreRepository
	GetLocalProductDataRepository() listingadmin.ProductDataRepository
	GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error)
	GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository
	GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository
}

type managementRuntime struct {
	source runtimeSource
}

func NewManagementRuntime(source runtimeSource) ManagementRuntime {
	if source == nil {
		return nil
	}
	return managementRuntime{source: source}
}

func (r managementRuntime) GetAutoPricingStoreConfig(ctx context.Context, storeID int64) (*platformtask.AutoPricingStoreConfig, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.GetAutoPricingStoreConfig(ctx, storeID)
}

func (r managementRuntime) GetRawJsonDataAdapter() domainproduct.RawJsonDataClient {
	if r.source == nil {
		return nil
	}
	return r.source.GetRawJsonDataAdapter()
}

func (r managementRuntime) GetStoreAPI() managementapi.StoreAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetStoreAPI()
}

func (r managementRuntime) GetPricingRuleClient() managementapi.PricingRuleAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetPricingRuleClient()
}

func (r managementRuntime) GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalPricingRuleRepository()
}

func (r managementRuntime) GetProductImportMappingAPI() managementapi.ProductImportMappingAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetProductImportMappingAPI()
}

func (r managementRuntime) GetInventoryRecordAPI() managementapi.InventoryRecordAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetInventoryRecordAPI()
}

func (r managementRuntime) GetLocalProductDataRepository() listingadmin.ProductDataRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalProductDataRepository()
}

func (r managementRuntime) PricingRuntime() temupricingruntime.ManagementRuntime {
	return temupricingruntime.NewManagementRuntime(r.source)
}

func (r managementRuntime) SyncRuntime() schedulerservice.ServiceRuntime {
	return schedulerservice.NewServiceRuntime(r.source)
}
