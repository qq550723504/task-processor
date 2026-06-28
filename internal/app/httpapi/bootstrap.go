package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/productenrich"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

func buildBootstrap(logger *logrus.Logger, options Options) (*appBootstrap, error) {
	timer := newStartupTimer(logger)

	done := timer.phase("buildRuntimeDeps")
	deps, err := buildRuntimeDeps(logger, options.ConfigPath)
	done()
	if err != nil {
		return nil, err
	}

	done = timer.phase("configureSheinLoginAccount")
	configureSheinLoginAccount(deps)
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

func (c httpFeatureComposition) productHandler() productenrich.ProductHandler {
	if c.productModule == nil {
		return nil
	}
	return c.productModule.Handler
}

func (c httpFeatureComposition) imageHandler() productimagehttpapi.RouteHandler {
	if c.imageModule == nil {
		return nil
	}
	return c.imageModule.Handler
}
