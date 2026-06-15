package httpapi

import (
	"github.com/sirupsen/logrus"

	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
	"task-processor/internal/listingsubscription"
	sheinpub "task-processor/internal/publishing/shein"
)

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

func BuildSheinResolutionCacheStore(cfg *config.Config, logger *logrus.Logger) (sheinpub.ResolutionCacheStore, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBSheinResolutionCacheStore, func(logger *logrus.Logger) (sheinpub.ResolutionCacheStore, []func() error, error) {
		logger.Warn("database not configured, using in-memory SHEIN resolution cache fallback")
		return nil, nil, nil
	})
}
