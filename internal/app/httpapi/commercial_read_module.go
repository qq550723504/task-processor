package httpapi

import (
	"errors"

	"github.com/sirupsen/logrus"
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
	reader := listingsubscription.NewCommercialReadService(listingsubscription.NewGormRepository(db), authorizer)
	return commercialReadBuildResult{module: commercialhttpapi.NewModule(commercialhttpapi.NewHandler(reader)), closer: func() error { return platformdatabase.CloseShared(configadapter.Database(cfg.Database), db) }}, nil
}
