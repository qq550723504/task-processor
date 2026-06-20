package pricing

import (
	"context"

	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
)

type runtime interface {
	GetStoreAPI() managementapi.StoreAPI
	GetPricingRuleClient() managementapi.PricingRuleAPI
	GetProductImportMappingAPI() managementapi.ProductImportMappingAPI
	GetLocalStoreRepository() *listingadmin.GormStoreRepository
	GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository
	GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository
	GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error)
}

type runtimeSource interface {
	GetStoreClient() *management.StoreAPIClient
	GetPricingRuleClient() *management.PricingRuleAPIClient
	GetProductImportMappingClient() *management.ProductImportMappingAPIClient
	GetLocalStoreRepository() *listingadmin.GormStoreRepository
	GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository
	GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository
	GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error)
}

type ManagementRuntime struct {
	source runtimeSource
}

func NewManagementRuntime(source runtimeSource) ManagementRuntime {
	return ManagementRuntime{source: source}
}

func (r ManagementRuntime) GetStoreAPI() managementapi.StoreAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetStoreClient()
}

func (r ManagementRuntime) GetPricingRuleClient() managementapi.PricingRuleAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetPricingRuleClient()
}

func (r ManagementRuntime) GetProductImportMappingAPI() managementapi.ProductImportMappingAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetProductImportMappingClient()
}

func (r ManagementRuntime) GetLocalStoreRepository() *listingadmin.GormStoreRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalStoreRepository()
}

func (r ManagementRuntime) GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalPricingRuleRepository()
}

func (r ManagementRuntime) GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalProductImportMappingRepository()
}

func (r ManagementRuntime) GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.GetRuntimeOperationStrategy(storeID)
}

type storeRepository interface {
	FindStoreByID(ctx context.Context, id int64) (*listingadmin.Store, error)
}
