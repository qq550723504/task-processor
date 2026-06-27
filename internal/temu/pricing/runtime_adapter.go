package pricing

import (
	"context"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
)

type runtime interface {
	GetStoreAPI() listingadmin.StoreAPI
	GetPricingRuleClient() listingadmin.PricingRuleAPI
	GetProductImportMappingAPI() listingadmin.ProductImportMappingAPI
	GetLocalStoreRepository() *listingadmin.GormStoreRepository
	GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository
	GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository
	GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error)
}

type runtimeSource interface {
	GetStoreAPI() listingadmin.StoreAPI
	GetPricingRuleClient() listingadmin.PricingRuleAPI
	GetProductImportMappingAPI() listingadmin.ProductImportMappingAPI
	GetLocalStoreRepository() *listingadmin.GormStoreRepository
	GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository
	GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository
	GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error)
}

type PricingRuntime struct {
	source runtimeSource
}

func NewPricingRuntime(source runtimeSource) PricingRuntime {
	return PricingRuntime{source: source}
}

func (r PricingRuntime) GetStoreAPI() listingadmin.StoreAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetStoreAPI()
}

func (r PricingRuntime) GetPricingRuleClient() listingadmin.PricingRuleAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetPricingRuleClient()
}

func (r PricingRuntime) GetProductImportMappingAPI() listingadmin.ProductImportMappingAPI {
	if r.source == nil {
		return nil
	}
	return r.source.GetProductImportMappingAPI()
}

func (r PricingRuntime) GetLocalStoreRepository() *listingadmin.GormStoreRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalStoreRepository()
}

func (r PricingRuntime) GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalPricingRuleRepository()
}

func (r PricingRuntime) GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository {
	if r.source == nil {
		return nil
	}
	return r.source.GetLocalProductImportMappingRepository()
}

func (r PricingRuntime) GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error) {
	if r.source == nil {
		return nil, nil
	}
	return r.source.GetRuntimeOperationStrategy(storeID)
}

type storeRepository interface {
	FindStoreByID(ctx context.Context, id int64) (*listingadmin.Store, error)
}
