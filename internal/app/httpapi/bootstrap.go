package httpapi

import (
	"strings"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
)

const productListingTraceOperation = "product-listing-api"

type bootstrapBuildDependencies struct {
	buildRuntimeDeps        func(*logrus.Logger, string) (*runtimeDeps, error)
	buildRouteAuthorization func(*config.Config) (routeAuthorization, error)
	buildComposition        func(*logrus.Logger, *runtimeDeps) (httpFeatureComposition, error)
	buildRuntimeBundle      func(httpFeatureComposition, *config.Config) (runtimeBundle, error)
}

func buildBootstrap(logger *logrus.Logger, options Options) (*appBootstrap, error) {
	return buildBootstrapWithDependencies(logger, options, bootstrapBuildDependencies{
		buildRuntimeDeps:        buildRuntimeDeps,
		buildRouteAuthorization: buildRouteAuthorization,
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
	routeAuthorizationBuilder := builders.buildRouteAuthorization
	if routeAuthorizationBuilder == nil {
		routeAuthorizationBuilder = buildRouteAuthorization
	}
	authorization, err := routeAuthorizationBuilder(deps.shared.cfg)
	if err != nil {
		return nil, err
	}

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
	server, routes := runtimeBundle.buildServerBundle(options.Port, authorization)
	if bindAddress := strings.TrimSpace(options.BindAddress); bindAddress != "" {
		server.Addr = serverAddress(bindAddress, options.Port)
	}
	if deps.shared.traceRuntime != nil {
		server.Handler = deps.shared.traceRuntime.WrapHTTPHandler(server.Handler, productListingTraceOperation)
	}
	done()
	timer.total("buildBootstrap")
	closers := append([]func() error(nil), deps.shared.closers...)
	if deps.featureFlagsCloser != nil {
		closers = append(closers, deps.featureFlagsCloser)
	}
	if deps.traceCloser != nil {
		closers = append(closers, deps.traceCloser)
	}
	bootstrap := &appBootstrap{
		server:  server,
		routes:  routes,
		pools:   runtimeBundle.pools(),
		closers: closers,
	}
	completed = true
	return bootstrap, nil
}
