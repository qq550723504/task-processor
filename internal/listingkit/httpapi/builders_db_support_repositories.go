package httpapi

import (
	"github.com/sirupsen/logrus"

	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/reviewstore"
	"task-processor/internal/listingkit/studiostore"
	"task-processor/internal/listingsubscription"
	sheinpub "task-processor/internal/publishing/shein"
)

func newDBSheinResolutionCacheStore(cfg *config.DatabaseConfig, logger *logrus.Logger) (sheinpub.ResolutionCacheStore, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return sheinpub.NewGormResolutionCacheStore(db), closer, nil
}

func newDBAssetRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (assetrepo.Repository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return assetrepo.NewGormRepository(db), closer, nil
}

func newDBListingKitReviewRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (reviewstore.Repository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return reviewstore.NewGormRepository(db), closer, nil
}

func newDBListingKitStudioSessionRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.StudioSessionRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return studiostore.NewGormRepository(db), closer, nil
}

func newDBListingSubscriptionRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingsubscription.Repository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingsubscription.NewGormRepository(db), closer, nil
}
