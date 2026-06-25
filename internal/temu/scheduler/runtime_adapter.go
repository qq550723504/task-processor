package scheduler

import (
	"context"

	"task-processor/internal/listingadmin"
	managementapi "task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	platformtask "task-processor/internal/platformtask"
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
	PricingRuntime() temupricingruntime.PricingRuntime
	SyncRuntime() schedulerservice.ServiceRuntime
}

type SchedulerRuntime = runtime

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

type schedulerRuntime struct {
	source runtimeSource
}

func NewSchedulerRuntime(source runtimeSource) SchedulerRuntime {
	if source == nil {
		return nil
	}
	return schedulerRuntime{source: source}
}

func (r schedulerRuntime) GetAutoPricingStoreConfig(ctx context.Context, storeID int64) (*platformtask.AutoPricingStoreConfig, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.GetAutoPricingStoreConfig(ctx, storeID)
}

func (r schedulerRuntime) GetRawJsonDataAdapter() domainproduct.RawJsonDataClient {
	if r.source == nil {
		return nil
	}
	return r.source.GetRawJsonDataAdapter()
}

func (r schedulerRuntime) GetStoreAPI() managementapi.StoreAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetStoreAPI()
}

func (r schedulerRuntime) GetPricingRuleClient() managementapi.PricingRuleAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetPricingRuleClient()
}

func (r schedulerRuntime) GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalPricingRuleRepository()
}

func (r schedulerRuntime) GetProductImportMappingAPI() managementapi.ProductImportMappingAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetProductImportMappingAPI()
}

func (r schedulerRuntime) GetInventoryRecordAPI() managementapi.InventoryRecordAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetInventoryRecordAPI()
}

func (r schedulerRuntime) GetLocalProductDataRepository() listingadmin.ProductDataRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalProductDataRepository()
}

func (r schedulerRuntime) PricingRuntime() temupricingruntime.PricingRuntime {
	return temupricingruntime.NewPricingRuntime(r.source)
}

func (r schedulerRuntime) SyncRuntime() schedulerservice.ServiceRuntime {
	return schedulerservice.NewServiceRuntime(r.source)
}
