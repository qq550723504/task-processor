package httpapi

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/app/configadapter"
	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
	platformdatabase "task-processor/internal/platform/database"
	"task-processor/internal/storecenter"
	storecenterhttpapi "task-processor/internal/storecenter/httpapi"
)

type storeCenterBuildResult struct {
	module kernelmodule.Module
	closer func() error
}

type storeCenterModuleBuilder func(*config.Config, *logrus.Logger) (storeCenterBuildResult, error)

type storeCenterFactories struct {
	openDatabase          func(*config.DatabaseConfig) (*gorm.DB, error)
	closeDatabase         func(*config.DatabaseConfig, *gorm.DB) error
	newStoreRepository    func(*gorm.DB) (storecenter.Repository, error)
	newAuditRepository    func(*gorm.DB) (storecenter.AuditRepository, error)
	newConnectionProvider func() storecenter.ConnectionStatusProvider
	newService            func(storecenter.Repository, storecenter.AuditRepository, storecenter.ConnectionStatusProvider, func() time.Time) (storecenterhttpapi.StoreService, error)
	newHandler            func(storecenterhttpapi.StoreService) (*storecenterhttpapi.Handler, error)
	newModule             func(*storecenterhttpapi.Handler) (kernelmodule.Module, error)
	now                   func() time.Time
}

func defaultStoreCenterFactories() storeCenterFactories {
	return storeCenterFactories{
		openDatabase: func(cfg *config.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.OpenShared(configadapter.Database(cfg))
		},
		closeDatabase: func(cfg *config.DatabaseConfig, db *gorm.DB) error {
			return platformdatabase.CloseShared(configadapter.Database(cfg), db)
		},
		newStoreRepository: func(db *gorm.DB) (storecenter.Repository, error) {
			return storecenter.NewGormStoreRepository(db)
		},
		newAuditRepository: func(db *gorm.DB) (storecenter.AuditRepository, error) {
			return storecenter.NewGormAuditRepository(db)
		},
		newConnectionProvider: func() storecenter.ConnectionStatusProvider {
			return unavailableConnectionStatusProvider{}
		},
		newService: func(repository storecenter.Repository, audit storecenter.AuditRepository, provider storecenter.ConnectionStatusProvider, now func() time.Time) (storecenterhttpapi.StoreService, error) {
			return storecenter.NewService(repository, audit, provider, now)
		},
		newHandler: storecenterhttpapi.NewHandler,
		newModule: func(handler *storecenterhttpapi.Handler) (kernelmodule.Module, error) {
			return storecenterhttpapi.NewModule(handler), nil
		},
		now: time.Now,
	}
}

func buildDefaultStoreCenterModule(cfg *config.Config, logger *logrus.Logger) (storeCenterBuildResult, error) {
	return buildStoreCenterModule(cfg, logger, defaultStoreCenterFactories())
}

func buildStoreCenterModule(cfg *config.Config, logger *logrus.Logger, factories storeCenterFactories) (result storeCenterBuildResult, err error) {
	_ = logger
	if cfg == nil || !cfg.Workbench.Enabled {
		return storeCenterBuildResult{}, nil
	}
	if cfg.Database == nil {
		return storeCenterBuildResult{}, errors.New("build Store Center: durable database configuration is required")
	}

	db, openErr := factories.openDatabase(cfg.Database)
	if openErr != nil {
		return storeCenterBuildResult{}, newStoreCenterStartupError("open database", openErr)
	}
	if db == nil {
		return storeCenterBuildResult{}, errors.New("build Store Center: durable database is unavailable")
	}

	release := true
	defer func() {
		if release {
			_ = factories.closeDatabase(cfg.Database, db)
		}
	}()

	repository, constructorErr := factories.newStoreRepository(db)
	if constructorErr != nil {
		return storeCenterBuildResult{}, newStoreCenterStartupError("construct Store repository", constructorErr)
	}
	audit, constructorErr := factories.newAuditRepository(db)
	if constructorErr != nil {
		return storeCenterBuildResult{}, newStoreCenterStartupError("construct audit repository", constructorErr)
	}
	provider := factories.newConnectionProvider()
	service, constructorErr := factories.newService(repository, audit, provider, factories.now)
	if constructorErr != nil {
		return storeCenterBuildResult{}, newStoreCenterStartupError("construct service", constructorErr)
	}
	handler, constructorErr := factories.newHandler(service)
	if constructorErr != nil {
		return storeCenterBuildResult{}, newStoreCenterStartupError("construct handler", constructorErr)
	}
	module, constructorErr := factories.newModule(handler)
	if constructorErr != nil {
		return storeCenterBuildResult{}, newStoreCenterStartupError("construct module", constructorErr)
	}
	if module == nil {
		return storeCenterBuildResult{}, errors.New("build Store Center: construct module failed")
	}

	release = false
	return storeCenterBuildResult{
		module: module,
		closer: func() error {
			return factories.closeDatabase(cfg.Database, db)
		},
	}, nil
}

type storeCenterStartupError struct {
	stage string
	cause error
}

func newStoreCenterStartupError(stage string, cause error) error {
	return &storeCenterStartupError{stage: stage, cause: cause}
}

func (e *storeCenterStartupError) Error() string {
	return fmt.Sprintf("build Store Center: %s failed", e.stage)
}

func (e *storeCenterStartupError) Unwrap() error { return e.cause }

// unavailableConnectionStatusProvider is the honest runtime boundary until a
// provider implementation for opaque Workbench connection references exists.
type unavailableConnectionStatusProvider struct{}

func (unavailableConnectionStatusProvider) Status(context.Context, storecenter.ConnectionStatusInput) (storecenter.ConnectionStatus, error) {
	return storecenter.ConnectionStatusUnavailable, nil
}
