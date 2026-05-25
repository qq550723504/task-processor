package httpapi

import (
	"task-processor/internal/listingadmin"
	listingkitapi "task-processor/internal/listingkit/api"
)

type adminModuleInput struct {
	StoreRepository                listingadmin.StoreRepository
	StoreStatisticsRepository      listingadmin.StoreStatisticsRepository
	ImportTaskRepository           listingadmin.ImportTaskRepository
	FilterRuleRepository           listingadmin.FilterRuleRepository
	ProfitRuleRepository           listingadmin.ProfitRuleRepository
	PricingRuleRepository          listingadmin.PricingRuleRepository
	OperationStrategyRepository    listingadmin.OperationStrategyRepository
	SensitiveWordRepository        listingadmin.SensitiveWordRepository
	ProductImportMappingRepository listingadmin.ProductImportMappingRepository
	CategoryRepository             listingadmin.CategoryRepository
	ProductDataRepository          listingadmin.ProductDataRepository
}

type adminModule struct {
	handlerDependencies listingkitapi.AdminHandlerDependencies
}

func newAdminModuleInput(repos *builtRepositories) adminModuleInput {
	return adminModuleInput{
		StoreRepository:                repos.storeRepository,
		StoreStatisticsRepository:      repos.storeStatisticsRepository,
		ImportTaskRepository:           repos.importTaskRepository,
		FilterRuleRepository:           repos.filterRuleRepository,
		ProfitRuleRepository:           repos.profitRuleRepository,
		PricingRuleRepository:          repos.pricingRuleRepository,
		OperationStrategyRepository:    repos.operationStrategyRepository,
		SensitiveWordRepository:        repos.sensitiveWordRepository,
		ProductImportMappingRepository: repos.productImportMappingRepository,
		CategoryRepository:             repos.categoryRepository,
		ProductDataRepository:          repos.productDataRepository,
	}
}

func buildAdminModule(in adminModuleInput) adminModule {
	return adminModule{
		handlerDependencies: listingkitapi.AdminHandlerDependencies{
			StoreRepository:                in.StoreRepository,
			StoreStatisticsRepository:      in.StoreStatisticsRepository,
			ImportTaskRepository:           in.ImportTaskRepository,
			FilterRuleRepository:           in.FilterRuleRepository,
			ProfitRuleRepository:           in.ProfitRuleRepository,
			PricingRuleRepository:          in.PricingRuleRepository,
			OperationStrategyRepository:    in.OperationStrategyRepository,
			SensitiveWordRepository:        in.SensitiveWordRepository,
			ProductImportMappingRepository: in.ProductImportMappingRepository,
			CategoryRepository:             in.CategoryRepository,
			ProductDataRepository:          in.ProductDataRepository,
		},
	}
}
