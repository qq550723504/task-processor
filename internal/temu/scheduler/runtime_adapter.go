package scheduler

import (
	"context"

	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/listingadmin"
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
	PricingRuntime() temupricingruntime.ManagementRuntime
	SyncRuntime() schedulerservice.ServiceRuntime
}

type ManagementRuntime = runtime

type managementRuntime struct {
	client *management.ClientManager
}

func NewManagementRuntime(client *management.ClientManager) ManagementRuntime {
	if client == nil {
		return nil
	}
	return managementRuntime{client: client}
}

func (r managementRuntime) GetStoreClient() *management.StoreAPIClient {
	if r.client == nil {
		return nil
	}
	return r.client.GetStoreClient()
}

func (r managementRuntime) GetAutoPricingStoreConfig(ctx context.Context, storeID int64) (*platformtask.AutoPricingStoreConfig, error) {
	if r.client == nil {
		return nil, nil
	}
	return r.client.GetAutoPricingStoreConfig(ctx, storeID)
}

func (r managementRuntime) GetRawJsonDataAdapter() domainproduct.RawJsonDataClient {
	if r.client == nil {
		return nil
	}
	return r.client.GetRawJsonDataAdapter()
}

func (r managementRuntime) GetStoreAPI() managementapi.StoreAPI {
	if r.client == nil {
		return nil
	}
	return r.client.GetStoreClient()
}

func (r managementRuntime) GetPricingRuleClient() managementapi.PricingRuleAPI {
	if r.client == nil {
		return nil
	}
	return r.client.GetPricingRuleClient()
}

func (r managementRuntime) GetProductImportMappingClient() *management.ProductImportMappingAPIClient {
	if r.client == nil {
		return nil
	}
	return r.client.GetProductImportMappingClient()
}

func (r managementRuntime) GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository {
	if r.client == nil {
		return nil
	}
	return r.client.GetLocalPricingRuleRepository()
}

func (r managementRuntime) GetProductImportMappingAPI() managementapi.ProductImportMappingAPI {
	if r.client == nil {
		return nil
	}
	return r.client.GetProductImportMappingClient()
}

func (r managementRuntime) GetInventoryRecordAPI() managementapi.InventoryRecordAPI {
	if r.client == nil {
		return nil
	}
	return r.client.GetInventoryRecordClient()
}

func (r managementRuntime) GetLocalProductDataRepository() listingadmin.ProductDataRepository {
	if r.client == nil {
		return nil
	}
	return r.client.GetLocalProductDataRepository()
}

func (r managementRuntime) PricingRuntime() temupricingruntime.ManagementRuntime {
	return temupricingruntime.NewManagementRuntime(r.client)
}

func (r managementRuntime) SyncRuntime() schedulerservice.ServiceRuntime {
	return schedulerservice.NewServiceRuntime(r.client)
}
