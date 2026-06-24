package httpapi

import (
	"task-processor/internal/listingadmin"
	listingkitapi "task-processor/internal/listingkit/api"
)

type adminModuleInput struct {
	StoreRepository                   listingadmin.StoreRepository
	StoreStatisticsRepository         listingadmin.StoreStatisticsRepository
	DispatchEventRepository           listingadmin.DispatchEventRepository
	ImportTaskRepository              listingadmin.ImportTaskRepository
	FilterRuleRepository              listingadmin.FilterRuleRepository
	ProfitRuleRepository              listingadmin.ProfitRuleRepository
	PricingRuleRepository             listingadmin.PricingRuleRepository
	OperationStrategyRepository       listingadmin.OperationStrategyRepository
	SensitiveWordRepository           listingadmin.SensitiveWordRepository
	GenerationTopicOverrideRepository listingadmin.GenerationTopicOverrideRepository
	GenerationTopicPolicyRepository   listingadmin.GenerationTopicPolicyRepository
	ProductImportMappingRepository    listingadmin.ProductImportMappingRepository
	CategoryRepository                listingadmin.CategoryRepository
	ProductDataRepository             listingadmin.ProductDataRepository
}

type adminModule struct {
	handlerDependencies listingkitapi.AdminHandlerDependencies
}

func newAdminModuleInput(repos *builtRepositories) adminModuleInput {
	return adminModuleInput{
		StoreRepository:                   repos.storeRepository,
		StoreStatisticsRepository:         repos.storeStatisticsRepository,
		DispatchEventRepository:           repos.dispatchEventRepository,
		ImportTaskRepository:              repos.importTaskRepository,
		FilterRuleRepository:              repos.filterRuleRepository,
		ProfitRuleRepository:              repos.profitRuleRepository,
		PricingRuleRepository:             repos.pricingRuleRepository,
		OperationStrategyRepository:       repos.operationStrategyRepository,
		SensitiveWordRepository:           repos.sensitiveWordRepository,
		GenerationTopicOverrideRepository: repos.generationTopicOverrideRepository,
		GenerationTopicPolicyRepository:   repos.generationTopicPolicyRepository,
		ProductImportMappingRepository:    repos.productImportMappingRepository,
		CategoryRepository:                repos.categoryRepository,
		ProductDataRepository:             repos.productDataRepository,
	}
}

func buildAdminModule(in adminModuleInput) adminModule {
	return adminModule{
		handlerDependencies: listingkitapi.AdminHandlerDependencies{
			StoreRepository:                   in.StoreRepository,
			StoreStatisticsRepository:         in.StoreStatisticsRepository,
			DispatchEventRepository:           in.DispatchEventRepository,
			ImportTaskRepository:              in.ImportTaskRepository,
			FilterRuleRepository:              in.FilterRuleRepository,
			ProfitRuleRepository:              in.ProfitRuleRepository,
			PricingRuleRepository:             in.PricingRuleRepository,
			OperationStrategyRepository:       in.OperationStrategyRepository,
			SensitiveWordRepository:           in.SensitiveWordRepository,
			GenerationTopicOverrideRepository: in.GenerationTopicOverrideRepository,
			GenerationTopicPolicyRepository:   in.GenerationTopicPolicyRepository,
			ProductImportMappingRepository:    in.ProductImportMappingRepository,
			CategoryRepository:                in.CategoryRepository,
			ProductDataRepository:             in.ProductDataRepository,
		},
	}
}
