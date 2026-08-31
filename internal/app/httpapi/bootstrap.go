package httpapi

import (
	"strings"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/productenrich"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

type bootstrapBuildDependencies struct {
	buildRuntimeDeps   func(*logrus.Logger, string) (*runtimeDeps, error)
	buildComposition   func(*logrus.Logger, *runtimeDeps) (httpFeatureComposition, error)
	buildRuntimeBundle func(httpFeatureComposition, *config.Config) (runtimeBundle, error)
}

func buildBootstrap(logger *logrus.Logger, options Options) (*appBootstrap, error) {
	return buildBootstrapWithDependencies(logger, options, bootstrapBuildDependencies{
		buildRuntimeDeps: buildRuntimeDeps,
		buildComposition: func(logger *logrus.Logger, deps *runtimeDeps) (httpFeatureComposition, error) {
			return newHTTPFeatureCompositionBuilder().build(logger, deps)
		},
		buildRuntimeBundle: func(composition httpFeatureComposition, cfg *config.Config) (runtimeBundle, error) {
			return composition.buildRuntimeBundle(cfg)
		},
	})
}

func buildBootstrapWithDependencies(logger *logrus.Logger, options Options, builders bootstrapBuildDependencies) (*appBootstrap, error) {
	timer := newStartupTimer(logger)

	done := timer.phase("buildRuntimeDeps")
	deps, err := builders.buildRuntimeDeps(logger, options.ConfigPath)
	done()
	if err != nil {
		return nil, err
	}
	completed := false
	defer func() {
		cleanupOwnedRuntimeResources(completed, deps.constructionClosers)
	}()
	deps.shared.sourceImageFetcher = options.SourceImageFetcher

	done = timer.phase("configureSheinLoginAccount")
	configureSheinLoginAccount(deps)
	done()

	done = timer.phase("buildHTTPFeatureComposition")
	composition, err := builders.buildComposition(logger, deps)
	done()
	if err != nil {
		return nil, err
	}

	done = timer.phase("buildRuntimeBundle")
	runtimeBundle, err := builders.buildRuntimeBundle(composition, deps.shared.cfg)
	done()
	if err != nil {
		return nil, err
	}

	done = timer.phase("buildHTTPServerBundle")
	server, routes := runtimeBundle.buildServerBundle(options.Port)
	if bindAddress := strings.TrimSpace(options.BindAddress); bindAddress != "" {
		server.Addr = serverAddress(bindAddress, options.Port)
	}
	done()
	timer.total("buildBootstrap")
	closers := append([]func() error(nil), deps.shared.closers...)
	if deps.featureFlagsCloser != nil {
		closers = append(closers, deps.featureFlagsCloser)
	}
	bootstrap := &appBootstrap{
		productHandler: composition.productHandler(),
		imageHandler:   composition.imageHandler(),
		server:         server,
		routes:         routes,
		pools:          runtimeBundle.pools(),
		closers:        closers,
	}
	completed = true
	return bootstrap, nil
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
