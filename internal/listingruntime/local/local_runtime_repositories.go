package local

import (
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
)

func (r *LocalRuntime) scheduledTaskConfigRepository() *listingadmin.GormScheduledTaskConfigRepository {
	if r == nil {
		return nil
	}
	if r.resources != nil {
		return r.resources.ScheduledTaskConfigRepository()
	}
	if r.provider != nil {
		return r.provider.ScheduledTaskConfigRepository()
	}
	return nil
}

func (r *LocalRuntime) operationStrategyRepository() *listingadmin.GormOperationStrategyRepository {
	if r == nil {
		return nil
	}
	if r.resources != nil {
		return r.resources.OperationStrategyRepository()
	}
	if r.provider != nil {
		return r.provider.OperationStrategyRepository()
	}
	return nil
}

func (r *LocalRuntime) GetLocalProductDataRepository() listingadmin.ProductDataRepository {
	if r == nil {
		return nil
	}
	if r.resources != nil {
		return r.resources.ProductDataRepository()
	}
	if r.provider != nil {
		return r.provider.ProductDataRepository()
	}
	return nil
}

func (r *LocalRuntime) GetLocalSheinSyncRepository() listingkit.SheinSyncRepository {
	if r == nil {
		return nil
	}
	if r.resources != nil {
		return r.resources.SheinSyncRepository()
	}
	if r.provider != nil {
		return r.provider.SheinSyncRepository()
	}
	return nil
}

func (r *LocalRuntime) GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository {
	if r == nil {
		return nil
	}
	if r.resources != nil {
		return r.resources.PricingRuleRepository()
	}
	if r.provider != nil {
		return r.provider.PricingRuleRepository()
	}
	return nil
}

func (r *LocalRuntime) GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository {
	return r.productImportMappingRepository()
}

func (r *LocalRuntime) productImportMappingRepository() *listingadmin.GormProductImportMappingRepository {
	if r == nil {
		return nil
	}
	if r.resources != nil {
		return r.resources.ProductImportMappingRepository()
	}
	if r.provider != nil {
		return r.provider.ProductImportMappingRepository()
	}
	return nil
}

func (r *LocalRuntime) GetLocalStoreRepository() *listingadmin.GormStoreRepository {
	if r == nil {
		return nil
	}
	if r.resources != nil {
		return r.resources.StoreRepository()
	}
	if r.provider != nil {
		return r.provider.StoreRepository()
	}
	return nil
}

func (r *LocalRuntime) GetLocalInventoryRecordRepository() *listingadmin.GormInventoryRecordRepository {
	if r == nil {
		return nil
	}
	if r.resources != nil {
		return r.resources.InventoryRecordRepository()
	}
	if r.provider != nil {
		return r.provider.InventoryRecordRepository()
	}
	return nil
}

func (r *LocalRuntime) GetLocalFilterRuleRepository() *listingadmin.GormFilterRuleRepository {
	if r == nil {
		return nil
	}
	if r.resources != nil {
		return r.resources.FilterRuleRepository()
	}
	if r.provider != nil {
		return r.provider.FilterRuleRepository()
	}
	return nil
}

func (r *LocalRuntime) GetLocalProfitRuleRepository() *listingadmin.GormProfitRuleRepository {
	if r == nil {
		return nil
	}
	if r.resources != nil {
		return r.resources.ProfitRuleRepository()
	}
	if r.provider != nil {
		return r.provider.ProfitRuleRepository()
	}
	return nil
}
