package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/listingkit"
	listingkitstore "task-processor/internal/listingkit/store"
)

func newDBListingKitTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.Repository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingkitstore.NewTaskRepository(db), closer, nil
}

func newDBListingKitStudioAsyncJobRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.StudioAsyncJobRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingkit.NewGormStudioAsyncJobRepository(db), closer, nil
}

func newDBListingKitStudioBatchRunRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.StudioBatchRunRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingkit.NewGormStudioBatchRunRepository(db), closer, nil
}

func newDBListingKitStudioBatchRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.StudioBatchRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingkit.NewGormStudioBatchRepository(db), closer, nil
}

func newDBListingKitStudioBatchTaskLinkRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.StudioBatchTaskLinkRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingkit.NewGormStudioBatchTaskLinkRepository(db), closer, nil
}

func newDBListingKitSheinSyncRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.SheinSyncRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingkitstore.NewSheinSyncRepository(db), closer, nil
}

func newDBListingKitUploadedImageRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.UploadedImageRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingkit.NewGormUploadedImageRepository(db), closer, nil
}

func newDBListingKitStoreProfileRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.StoreProfileRepository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return listingkit.NewGormStoreProfileRepository(db), closer, nil
}
