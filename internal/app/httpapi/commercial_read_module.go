package httpapi

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"task-processor/internal/app/configadapter"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingsubscription"
	commercialhttpapi "task-processor/internal/listingsubscription/httpapi"
	platformdatabase "task-processor/internal/platform/database"
)

type commercialReadBuildResult struct {
	module kernelmodule.Module
	closer func() error
}
type commercialReadModuleBuilder func(*config.Config, *logrus.Logger) (commercialReadBuildResult, error)

func buildCommercialReadModule(cfg *config.Config, _ *logrus.Logger) (commercialReadBuildResult, error) {
	if cfg == nil || !cfg.Workbench.Enabled {
		return commercialReadBuildResult{}, nil
	}
	if cfg.Database == nil {
		return commercialReadBuildResult{}, errors.New("commercial read requires durable database configuration")
	}
	authorizer, err := authz.NewListingKitAuthorizer(cfg.ListingKit.PlatformAdminUsers, cfg.ListingKit.PlatformAdminRoles)
	if err != nil {
		return commercialReadBuildResult{}, errors.New("commercial read authorizer unavailable")
	}
	db, err := platformdatabase.OpenShared(configadapter.Database(cfg.Database))
	if err != nil {
		return commercialReadBuildResult{}, errors.New("commercial read database unavailable")
	}
	// Repository construction is side-effect-free. No AutoMigrate, NewService,
	// default plans, grants, counter repairs or resource commands are called.
	module := newCommercialReadModule(db, authorizer)
	return commercialReadBuildResult{module: module, closer: func() error { return platformdatabase.CloseShared(configadapter.Database(cfg.Database), db) }}, nil
}

func buildCommercialReadModuleFromDatabase(ctx context.Context, db *gorm.DB, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
	if db == nil || authorizer == nil {
		return nil, errors.New("commercial read dependencies unavailable")
	}
	if err := listingsubscription.VerifyCommercialReadSchema(ctx, db); err != nil {
		return nil, err
	}
	return newCommercialReadModule(db, authorizer), nil
}

func newCommercialReadModule(db *gorm.DB, authorizer *authz.ListingKitAuthorizer) kernelmodule.Module {
	reader := listingsubscription.NewCommercialReadService(listingsubscription.NewGormRepository(db), authorizer)
	return commercialhttpapi.NewModule(commercialhttpapi.NewHandler(reader))
}
