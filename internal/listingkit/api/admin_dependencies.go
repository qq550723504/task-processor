package api

import "task-processor/internal/listingadmin"

func withStoreAdminDependencies(deps AdminHandlerDependencies) HandlerOption {
	options := []HandlerOption{
		WithStoreRepository(deps.StoreRepository),
		WithStoreStatisticsRepository(deps.StoreStatisticsRepository),
		WithDispatchEventRepository(deps.DispatchEventRepository),
		WithImportTaskRepository(deps.ImportTaskRepository),
	}
	return func(h *handler) {
		for _, option := range options {
			if option != nil {
				option(h)
			}
		}
	}
}

func withCatalogAdminDependencies(deps AdminHandlerDependencies) HandlerOption {
	options := []HandlerOption{
		WithFilterRuleRepository(deps.FilterRuleRepository),
		WithProfitRuleRepository(deps.ProfitRuleRepository),
		WithPricingRuleRepository(deps.PricingRuleRepository),
		WithOperationStrategyRepository(deps.OperationStrategyRepository),
		WithSensitiveWordRepository(deps.SensitiveWordRepository),
		WithGenerationTopicOverrideRepository(deps.GenerationTopicOverrideRepository),
		WithGenerationTopicPolicyRepository(deps.GenerationTopicPolicyRepository),
		WithProductImportMappingRepository(deps.ProductImportMappingRepository),
		WithCategoryRepository(deps.CategoryRepository),
		WithProductDataRepository(deps.ProductDataRepository),
	}
	return func(h *handler) {
		for _, option := range options {
			if option != nil {
				option(h)
			}
		}
	}
}

func WithStoreRepository(repo listingadmin.StoreRepository) HandlerOption {
	return withHandlerState(func(h *handler) {
		if repo == nil {
			return
		}
		h.storeRepository = repo
		h.adminHandlers.storeHandler = listingadmin.NewStoreHandler(repo)
	})
}

func WithStoreStatisticsRepository(repo listingadmin.StoreStatisticsRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.StoreStatisticsRepository, admin *adminHandlers) {
		admin.storeStatisticsHandler = listingadmin.NewStoreStatisticsHandler(repo)
	})
}

func WithDispatchEventRepository(repo listingadmin.DispatchEventRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.DispatchEventRepository, admin *adminHandlers) {
		admin.dispatchEventHandler = listingadmin.NewDispatchEventHandler(repo)
	})
}

func WithImportTaskRepository(repo listingadmin.ImportTaskRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.ImportTaskRepository, admin *adminHandlers) {
		admin.importTaskHandler = listingadmin.NewImportTaskHandler(repo)
	})
}

func WithFilterRuleRepository(repo listingadmin.FilterRuleRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.FilterRuleRepository, admin *adminHandlers) {
		admin.filterRuleHandler = listingadmin.NewFilterRuleHandler(repo)
	})
}

func WithProfitRuleRepository(repo listingadmin.ProfitRuleRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.ProfitRuleRepository, admin *adminHandlers) {
		admin.profitRuleHandler = listingadmin.NewProfitRuleHandler(repo)
	})
}

func WithPricingRuleRepository(repo listingadmin.PricingRuleRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.PricingRuleRepository, admin *adminHandlers) {
		admin.pricingRuleHandler = listingadmin.NewPricingRuleHandler(repo)
	})
}

func WithOperationStrategyRepository(repo listingadmin.OperationStrategyRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.OperationStrategyRepository, admin *adminHandlers) {
		admin.operationStrategyHandler = listingadmin.NewOperationStrategyHandler(repo)
	})
}

func WithSensitiveWordRepository(repo listingadmin.SensitiveWordRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.SensitiveWordRepository, admin *adminHandlers) {
		admin.sensitiveWordHandler = listingadmin.NewSensitiveWordHandler(repo)
	})
}

func WithGenerationTopicOverrideRepository(repo listingadmin.GenerationTopicOverrideRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.GenerationTopicOverrideRepository, admin *adminHandlers) {
		admin.generationTopicOverrideHandler = listingadmin.NewGenerationTopicOverrideHandler(repo)
		admin.generationTopicCatalogHandler = listingadmin.NewGenerationTopicCatalogHandler(repo)
	})
}

func WithGenerationTopicPolicyRepository(repo listingadmin.GenerationTopicPolicyRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.GenerationTopicPolicyRepository, admin *adminHandlers) {
		admin.generationTopicPolicyHandler = listingadmin.NewGenerationTopicPolicyHandler(repo)
	})
}

func WithProductImportMappingRepository(repo listingadmin.ProductImportMappingRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.ProductImportMappingRepository, admin *adminHandlers) {
		admin.productImportMappingHandler = listingadmin.NewProductImportMappingHandler(repo)
	})
}

func WithCategoryRepository(repo listingadmin.CategoryRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.CategoryRepository, admin *adminHandlers) {
		admin.categoryHandler = listingadmin.NewCategoryHandler(repo)
	})
}

func WithProductDataRepository(repo listingadmin.ProductDataRepository) HandlerOption {
	return withAdminDependency(repo, func(repo listingadmin.ProductDataRepository, admin *adminHandlers) {
		admin.productDataHandler = listingadmin.NewProductDataHandler(repo)
	})
}
