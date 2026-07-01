package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/listingadmin"
)

func BuildListingAdminStoreRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminStoreRepository, func(logger *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit store admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminStoreStatisticsRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminStoreStatisticsRepository, func(logger *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit store statistics admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminDispatchEventRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.DispatchEventRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminDispatchEventRepository, func(logger *logrus.Logger) (listingadmin.DispatchEventRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit dispatch event admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminImportTaskRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ImportTaskRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminImportTaskRepository, func(logger *logrus.Logger) (listingadmin.ImportTaskRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit import task admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminFilterRuleRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.FilterRuleRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminFilterRuleRepository, func(logger *logrus.Logger) (listingadmin.FilterRuleRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit filter rule admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminProfitRuleRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ProfitRuleRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminProfitRuleRepository, func(logger *logrus.Logger) (listingadmin.ProfitRuleRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit profit rule admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminPricingRuleRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.PricingRuleRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminPricingRuleRepository, func(logger *logrus.Logger) (listingadmin.PricingRuleRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit pricing rule admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminOperationStrategyRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.OperationStrategyRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminOperationStrategyRepository, func(logger *logrus.Logger) (listingadmin.OperationStrategyRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit operation strategy admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminScheduledTaskConfigRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ScheduledTaskConfigRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminScheduledTaskConfigRepository, func(logger *logrus.Logger) (listingadmin.ScheduledTaskConfigRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit scheduled task config admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminSensitiveWordRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.SensitiveWordRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminSensitiveWordRepository, func(logger *logrus.Logger) (listingadmin.SensitiveWordRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit sensitive word admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminGenerationTopicPolicyRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.GenerationTopicPolicyRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminGenerationTopicPolicyRepository, func(logger *logrus.Logger) (listingadmin.GenerationTopicPolicyRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit generation topic policy admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminGenerationTopicOverrideRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.GenerationTopicOverrideRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminGenerationTopicOverrideRepository, func(logger *logrus.Logger) (listingadmin.GenerationTopicOverrideRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit generation topic override admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminProductImportMappingRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ProductImportMappingRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminProductImportMappingRepository, func(logger *logrus.Logger) (listingadmin.ProductImportMappingRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit product import mapping admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminCategoryRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.CategoryRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminCategoryRepository, func(logger *logrus.Logger) (listingadmin.CategoryRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit category admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminProductDataRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ProductDataRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminProductDataRepository, func(logger *logrus.Logger) (listingadmin.ProductDataRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit product data admin API disabled")
		return nil, nil, nil
	})
}
