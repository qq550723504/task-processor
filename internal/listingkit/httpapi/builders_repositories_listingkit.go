package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/reviewstore"
	listingkitstore "task-processor/internal/listingkit/store"
)

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

func BuildListingKitStudioBatchTaskLinkRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StudioBatchTaskLinkRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitStudioBatchTaskLinkRepository, func(logger *logrus.Logger) (listingkit.StudioBatchTaskLinkRepository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit studio batch task link repository")
		return listingkit.NewMemStudioBatchTaskLinkRepository(), nil, nil
	})
}

func BuildListingKitSheinSyncRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.SheinSyncRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitSheinSyncRepository, func(logger *logrus.Logger) (listingkit.SheinSyncRepository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit shein sync repository")
		return listingkitstore.NewMemSheinSyncRepository(), nil, nil
	})
}

func BuildListingKitStoreProfileRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StoreProfileRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitStoreProfileRepository, func(logger *logrus.Logger) (listingkit.StoreProfileRepository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit store profile repository")
		return nil, nil, nil
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
