package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/infra/worker"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/productenrich"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
	sdsbootstrap "task-processor/internal/sds/httpbootstrap"
	sdsloginbootstrap "task-processor/internal/sdslogin/bootstrap"
	sheinclient "task-processor/internal/shein/client"
	sheinloginbootstrap "task-processor/internal/sheinlogin/bootstrap"
)

var newSDSSyncServiceForHTTPAPI = sdsbootstrap.NewSyncService

func buildBootstrap(logger *logrus.Logger, options Options) (*appBootstrap, error) {
	timer := newStartupTimer(logger)

	done := timer.phase("buildRuntimeDeps")
	deps, err := buildRuntimeDeps(logger, options.ConfigPath)
	done()
	if err != nil {
		return nil, err
	}

	done = timer.phase("configureSheinLoginAccount")
	sheinclient.ConfigureLoginAccountFromConfig(deps.shared.cfg)
	done()

	done = timer.phase("buildHTTPFeatureComposition")
	composition, err := newHTTPFeatureCompositionBuilder().build(logger, deps)
	done()
	if err != nil {
		return nil, err
	}

	done = timer.phase("buildRuntimeBundle")
	runtimeBundle, err := composition.buildRuntimeBundle(deps.shared.cfg)
	done()
	if err != nil {
		return nil, err
	}

	done = timer.phase("buildHTTPServerBundle")
	server, routes := runtimeBundle.buildServerBundle(options.Port)
	done()
	timer.total("buildBootstrap")
	return &appBootstrap{
		productHandler: composition.productHandler(),
		imageHandler:   composition.imageHandler(),
		server:         server,
		routes:         routes,
		pools:          runtimeBundle.pools(),
		closers:        deps.shared.closers,
	}, nil
}

func buildSheinLoginModuleResult(deps *runtimeDeps) (*sheinloginbootstrap.BuildResult, func() error, error) {
	if deps == nil {
		return nil, nil, nil
	}

	result, err := sheinloginbootstrap.BuildHandler(sheinloginbootstrap.BuildInput{
		Config:                   deps.shared.cfg,
		ManagementClient:         deps.managementClient(),
		AccountRepositoryBuilder: listingkithttpapi.BuildListingAdminStoreRepository,
	})
	if err != nil {
		return nil, nil, err
	}
	if result == nil {
		return nil, nil, nil
	}
	return result, result.Close, nil
}

func buildSDSLoginModuleResult(deps *runtimeDeps) (*sdsloginbootstrap.BuildResult, func() error, error) {
	if deps == nil {
		return nil, nil, nil
	}
	result, err := sdsloginbootstrap.BuildHandler(deps.shared.cfg)
	if err != nil {
		return nil, nil, err
	}
	if result == nil || result.StatusProvider == nil {
		return nil, nil, nil
	}
	return result, nil, nil
}

func BuildHandlers(logger *logrus.Logger, options Options) (productenrich.ProductHandler, productimagehttpapi.RouteHandler, []worker.WorkerPool, []func() error, error) {
	bootstrap, err := buildBootstrap(logger, options)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return bootstrap.productHandler, bootstrap.imageHandler, bootstrap.pools, bootstrap.closers, nil
}
