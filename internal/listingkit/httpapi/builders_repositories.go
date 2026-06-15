package httpapi

import (
	"github.com/sirupsen/logrus"

	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/reviewstore"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/listingsubscription"
	sheinpub "task-processor/internal/publishing/shein"
)

func buildRepositoryWithFallback[T any](
	cfg *config.Config,
	logger *logrus.Logger,
	buildDB func(*config.DatabaseConfig, *logrus.Logger) (T, func() error, error),
	buildFallback func(*logrus.Logger) (T, []func() error, error),
) (T, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := buildDB(cfg.Database, logger)
		if err != nil {
			var zero T
			return zero, nil, err
		}
		return repo, []func() error{closer}, nil
	}
	return buildFallback(logger)
}

func BuildListingKitTaskRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.Repository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitTaskRepository, func(logger *logrus.Logger) (listingkit.Repository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit repository")
		return listingkitstore.NewMemTaskRepository(), nil, nil
	})
}

func BuildListingKitStudioAsyncJobRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StudioAsyncJobRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitStudioAsyncJobRepository, func(logger *logrus.Logger) (listingkit.StudioAsyncJobRepository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit studio async job repository")
		return listingkit.NewMemStudioAsyncJobRepository(), nil, nil
	})
}

func BuildListingKitStudioBatchRunRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StudioBatchRunRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitStudioBatchRunRepository, func(logger *logrus.Logger) (listingkit.StudioBatchRunRepository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit studio batch run repository for studio batch run APIs")
		return listingkit.NewMemStudioBatchRunRepository(), nil, nil
	})
}

func BuildListingKitStudioBatchRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StudioBatchRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitStudioBatchRepository, func(logger *logrus.Logger) (listingkit.StudioBatchRepository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit studio batch repository")
		return listingkit.NewMemStudioBatchRepository(), nil, nil
	})
}

func BuildListingKitSheinSyncRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.SheinSyncRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitSheinSyncRepository, func(logger *logrus.Logger) (listingkit.SheinSyncRepository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit shein sync repository")
		return listingkitstore.NewMemSheinSyncRepository(), nil, nil
	})
}

func BuildListingAdminStoreRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminStoreRepository, func(logger *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit store admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingKitStoreProfileRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StoreProfileRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitStoreProfileRepository, func(logger *logrus.Logger) (listingkit.StoreProfileRepository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit store profile repository")
		return nil, nil, nil
	})
}

func BuildListingAdminStoreStatisticsRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminStoreStatisticsRepository, func(logger *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit store statistics admin API disabled")
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

func BuildListingSubscriptionRepository(cfg *config.Config, logger *logrus.Logger) (listingsubscription.Repository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingSubscriptionRepository, func(logger *logrus.Logger) (listingsubscription.Repository, []func() error, error) {
		logger.Warn("database not configured, using in-memory ListingKit subscription repository")
		return listingsubscription.NewMemRepository(), nil, nil
	})
}

func BuildAssetRepository(cfg *config.Config, logger *logrus.Logger) (assetrepo.Repository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBAssetRepository, func(logger *logrus.Logger) (assetrepo.Repository, []func() error, error) {
		logger.Warn("database not configured, using in-memory asset repository")
		return assetrepo.NewMemRepository(), nil, nil
	})
}

func BuildListingKitReviewRepository(cfg *config.Config, logger *logrus.Logger) (reviewstore.Repository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitReviewRepository, func(logger *logrus.Logger) (reviewstore.Repository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit review repository")
		return reviewstore.NewMemRepository(), nil, nil
	})
}

func BuildListingKitStudioSessionRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StudioSessionRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitStudioSessionRepository, func(logger *logrus.Logger) (listingkit.StudioSessionRepository, []func() error, error) {
		logger.Warn("database not configured, SHEIN studio session repository disabled")
		return nil, nil, nil
	})
}

func BuildListingKitUploadedImageRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.UploadedImageRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitUploadedImageRepository, func(logger *logrus.Logger) (listingkit.UploadedImageRepository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit uploaded image repository")
		return listingkit.NewMemUploadedImageRepository(), nil, nil
	})
}

func BuildSheinResolutionCacheStore(cfg *config.Config, logger *logrus.Logger) (sheinpub.ResolutionCacheStore, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBSheinResolutionCacheStore, func(logger *logrus.Logger) (sheinpub.ResolutionCacheStore, []func() error, error) {
		logger.Warn("database not configured, using in-memory SHEIN resolution cache fallback")
		return nil, nil, nil
	})
}
