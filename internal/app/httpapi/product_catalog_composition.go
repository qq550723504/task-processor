package httpapi

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/sirupsen/logrus"

	"task-processor/internal/app/configadapter"
	"task-processor/internal/core/config"
	catalogpersistence "task-processor/internal/integration/persistence/product/catalog"
	platformdatabase "task-processor/internal/platform/database"
	productcatalog "task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

type productCatalogDatabaseBuilder func(*config.DatabaseConfig, *logrus.Logger) (*gorm.DB, func() error, error)

// attachProductSnapshotRepository is the typed composition boundary for the
// production Catalog repository. The persistence adapter receives only a
// database handle; it never receives or loads legacy application config.
func attachProductSnapshotRepository(deps *runtimeDeps, db *gorm.DB) error {
	if deps == nil || db == nil {
		return productcatalog.ErrRepositoryUnavailable
	}
	repository, err := catalogpersistence.NewRepository(db)
	if err != nil {
		return err
	}
	catalogPublisher, err := productcatalog.NewPublisher(repository)
	if err != nil {
		return err
	}
	sourcePublisher, err := sourcing.NewPublisher(catalogPublisher)
	if err != nil {
		return err
	}
	if deps.features == nil {
		deps.features = &featureRuntimeState{}
	}
	deps.features.productSnapshotReader = newListingKitProductSnapshotReader(repository)
	deps.features.productSnapshotPublisher = sourcePublisher
	return nil
}

func initializeProductSnapshotReader(deps *runtimeDeps) error {
	if deps == nil || deps.shared == nil || deps.shared.productCatalogDB == nil {
		return nil
	}
	return attachProductSnapshotRepository(deps, deps.shared.productCatalogDB)
}

func openProductCatalogDatabase(cfg *config.DatabaseConfig, logger *logrus.Logger) (*gorm.DB, func() error, error) {
	if cfg == nil {
		return nil, nil, nil
	}
	databaseConfig := configadapter.Database(cfg)
	db, err := platformdatabase.OpenShared(databaseConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	if logger != nil {
		logger.Infof("product catalog database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	}
	return db, func() error { return platformdatabase.CloseShared(databaseConfig, db) }, nil
}
