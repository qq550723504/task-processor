package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	assetpersistence "task-processor/internal/integration/persistence/product/asset"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/memberinvite"
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

func newDBApprovedAssetInventoryReader(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.ApprovedAssetInventoryReader, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	repository, err := assetpersistence.NewRepository(db)
	if err != nil {
		_ = closer()
		return nil, nil, err
	}
	return repository, closer, nil
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

func newDBMemberInvitationAuditRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (memberinvite.AuditRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return memberinvite.NewGormAuditRepository(db), closer, nil
}
