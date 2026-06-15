package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/listingadmin"
)

func newDBListingAdminStoreRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.StoreRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingadmin.NewGormStoreRepository(db), closer, nil
}

func newDBListingAdminStoreStatisticsRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.StoreStatisticsRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingadmin.NewGormStoreStatisticsRepository(db), closer, nil
}

func newDBListingAdminImportTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.ImportTaskRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingadmin.NewGormImportTaskRepository(db), closer, nil
}

func newDBListingAdminFilterRuleRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.FilterRuleRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingadmin.NewGormFilterRuleRepository(db), closer, nil
}

func newDBListingAdminProfitRuleRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.ProfitRuleRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingadmin.NewGormProfitRuleRepository(db), closer, nil
}

func newDBListingAdminPricingRuleRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.PricingRuleRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingadmin.NewGormPricingRuleRepository(db), closer, nil
}

func newDBListingAdminOperationStrategyRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.OperationStrategyRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingadmin.NewGormOperationStrategyRepository(db), closer, nil
}

func newDBListingAdminSensitiveWordRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.SensitiveWordRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingadmin.NewGormSensitiveWordRepository(db), closer, nil
}

func newDBListingAdminGenerationTopicPolicyRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.GenerationTopicPolicyRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingadmin.NewGormGenerationTopicPolicyRepository(db), closer, nil
}

func newDBListingAdminGenerationTopicOverrideRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.GenerationTopicOverrideRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingadmin.NewGormGenerationTopicOverrideRepository(db), closer, nil
}

func newDBListingAdminProductImportMappingRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.ProductImportMappingRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingadmin.NewGormProductImportMappingRepository(db), closer, nil
}

func newDBListingAdminCategoryRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.CategoryRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingadmin.NewGormCategoryRepository(db), closer, nil
}

func newDBListingAdminProductDataRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.ProductDataRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingadmin.NewGormProductDataRepository(db), closer, nil
}
