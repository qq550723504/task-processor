package local

import (
	"task-processor/internal/listingadmin"
	"task-processor/internal/product"
)

func NewLocalStoreAPI(provider *LocalDataProvider, cookieProvider SheinCookieProvider) listingadmin.StoreAPI {
	if provider == nil {
		return nil
	}
	return localStoreAPI{provider: provider, cookieProvider: cookieProvider}
}

func NewLocalStoreAPIFromResources(resources *RuntimeResources, provider *LocalDataProvider, cookieProvider SheinCookieProvider) listingadmin.StoreAPI {
	if resources == nil {
		return NewLocalStoreAPI(provider, cookieProvider)
	}
	storeAPI := listingadmin.NewGormStoreAPI(resources.StoreRepository())
	if storeAPI == nil {
		return NewLocalStoreAPI(provider, cookieProvider)
	}
	return localStoreAPI{
		storeAPI:       storeAPI,
		storeState:     newLocalStoreRuntimeState(resources, storeAPI),
		provider:       provider,
		cookieProvider: cookieProvider,
	}
}

func NewLocalProductDataAPI(provider *LocalDataProvider, storeID int64) listingadmin.ProductDataAPI {
	if provider == nil {
		return nil
	}
	return localProductDataAPI{provider: provider, storeID: storeID}
}

func NewLocalProductImportMappingAPI(provider *LocalDataProvider) listingadmin.ProductImportMappingAPI {
	if provider == nil {
		return nil
	}
	return localProductImportMappingAPI{provider: provider}
}

func NewLocalInventoryRecordAPI(provider *LocalDataProvider) listingadmin.InventoryRecordAPI {
	if provider == nil {
		return nil
	}
	return localInventoryRecordAPI{provider: provider}
}

func (r *LocalRuntime) GetStoreAPI() listingadmin.StoreAPI {
	if r == nil {
		return nil
	}
	return NewLocalStoreAPIFromResources(r.resources, r.provider, r.cookieProvider)
}

func (r *LocalRuntime) GetRawJsonDataAdapter() product.RawJsonDataClient {
	if r == nil || r.resources == nil {
		return nil
	}
	api := listingadmin.NewGormRawJsonDataAPI(r.resources.RawJSONDataRepository())
	if api == nil {
		return nil
	}
	return NewRawJsonDataAdapter(api)
}

func (r *LocalRuntime) GetPricingRuleClient() listingadmin.PricingRuleAPI {
	if r == nil || r.resources == nil {
		return nil
	}
	return listingadmin.NewGormPricingRuleAPI(r.resources.PricingRuleRepository())
}

func (r *LocalRuntime) GetProductImportMappingAPI() listingadmin.ProductImportMappingAPI {
	if r == nil {
		return nil
	}
	if r.resources == nil {
		return NewLocalProductImportMappingAPI(r.provider)
	}
	return listingadmin.NewGormProductImportMappingAPI(r.resources.ProductImportMappingRepository())
}

func (r *LocalRuntime) GetInventoryRecordAPI() listingadmin.InventoryRecordAPI {
	if r == nil || r.resources == nil {
		return nil
	}
	return listingadmin.NewGormInventoryRecordAPI(r.resources.InventoryRecordRepository())
}

func (r *LocalRuntime) GetProductDataClient(storeID int64) listingadmin.ProductDataAPI {
	if r == nil {
		return nil
	}
	if r.resources == nil {
		return NewLocalProductDataAPI(r.provider, storeID)
	}
	return listingadmin.NewGormProductDataAPI(r.resources.ProductDataRepository(), storeID)
}

func (r *LocalRuntime) GetFilterRuleClient() listingadmin.FilterRuleAPI {
	if r == nil || r.resources == nil {
		return nil
	}
	return listingadmin.NewGormFilterRuleAPI(r.resources.FilterRuleRepository())
}

func (r *LocalRuntime) GetDailyListingCountClient() listingadmin.DailyListingCountAPI {
	if r == nil {
		return nil
	}
	if r.resources != nil {
		return r.resources.DailyListingCountAPI()
	}
	if r.provider != nil {
		return r.provider
	}
	return nil
}

func (r *LocalRuntime) GetProfitRuleClient() listingadmin.ProfitRuleAPI {
	if r == nil || r.resources == nil {
		return nil
	}
	return listingadmin.NewGormProfitRuleAPI(r.resources.ProfitRuleRepository())
}
