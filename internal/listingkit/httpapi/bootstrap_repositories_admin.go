package httpapi

func buildAdminRepositories(input BuildServiceInput, closers *closerStack) (*builtAdminRepositories, error) {
	catalog, err := buildAdminCatalogRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	rules, err := buildAdminRuleRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	return &builtAdminRepositories{
		storeRepository:                   catalog.storeRepository,
		storeStatisticsRepository:         catalog.storeStatisticsRepository,
		dispatchEventRepository:           catalog.dispatchEventRepository,
		importTaskRepository:              catalog.importTaskRepository,
		filterRuleRepository:              rules.filterRuleRepository,
		profitRuleRepository:              rules.profitRuleRepository,
		pricingRuleRepository:             rules.pricingRuleRepository,
		operationStrategyRepository:       rules.operationStrategyRepository,
		sensitiveWordRepository:           rules.sensitiveWordRepository,
		generationTopicOverrideRepository: rules.generationTopicOverrideRepository,
		generationTopicPolicyRepository:   rules.generationTopicPolicyRepository,
		productImportMappingRepository:    catalog.productImportMappingRepository,
		categoryRepository:                catalog.categoryRepository,
		productDataRepository:             catalog.productDataRepository,
	}, nil
}

func buildAdminCatalogRepositories(input BuildServiceInput, closers *closerStack) (*adminCatalogRepositories, error) {
	repoBuilders := input.Repositories.Admin

	storeRepository, err := buildNamedWithClosers("admin.store", repoBuilders.Store, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	storeStatisticsRepository, err := buildNamedWithClosers("admin.store_statistics", repoBuilders.StoreStatistics, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	dispatchEventRepository, err := buildNamedWithClosers("admin.dispatch_event", repoBuilders.DispatchEvent, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	importTaskRepository, err := buildNamedWithClosers("admin.import_task", repoBuilders.ImportTask, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	productImportMappingRepository, err := buildNamedWithClosers("admin.product_import_mapping", repoBuilders.ProductImportMapping, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	categoryRepository, err := buildNamedWithClosers("admin.category", repoBuilders.Category, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	productDataRepository, err := buildNamedWithClosers("admin.product_data", repoBuilders.ProductData, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}

	return &adminCatalogRepositories{
		storeRepository:                storeRepository,
		storeStatisticsRepository:      storeStatisticsRepository,
		dispatchEventRepository:        dispatchEventRepository,
		importTaskRepository:           importTaskRepository,
		productImportMappingRepository: productImportMappingRepository,
		categoryRepository:             categoryRepository,
		productDataRepository:          productDataRepository,
	}, nil
}

func buildAdminRuleRepositories(input BuildServiceInput, closers *closerStack) (*adminRuleRepositories, error) {
	repoBuilders := input.Repositories.Admin

	filterRuleRepository, err := buildNamedWithClosers("admin.filter_rule", repoBuilders.FilterRule, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	profitRuleRepository, err := buildNamedWithClosers("admin.profit_rule", repoBuilders.ProfitRule, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	pricingRuleRepository, err := buildNamedWithClosers("admin.pricing_rule", repoBuilders.PricingRule, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	operationStrategyRepository, err := buildNamedWithClosers("admin.operation_strategy", repoBuilders.OperationStrategy, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	sensitiveWordRepository, err := buildNamedWithClosers("admin.sensitive_word", repoBuilders.SensitiveWord, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	generationTopicOverrideRepository, err := buildNamedWithClosers("admin.generation_topic_override", repoBuilders.GenerationTopicOverride, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	generationTopicPolicyRepository, err := buildNamedWithClosers("admin.generation_topic_policy", repoBuilders.GenerationTopicPolicy, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}

	return &adminRuleRepositories{
		filterRuleRepository:              filterRuleRepository,
		profitRuleRepository:              profitRuleRepository,
		pricingRuleRepository:             pricingRuleRepository,
		operationStrategyRepository:       operationStrategyRepository,
		sensitiveWordRepository:           sensitiveWordRepository,
		generationTopicOverrideRepository: generationTopicOverrideRepository,
		generationTopicPolicyRepository:   generationTopicPolicyRepository,
	}, nil
}
