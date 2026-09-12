package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	sourceaccountstore "task-processor/internal/integration/persistence/sourceaccountregistry"
	kernelmodule "task-processor/internal/kernel/module"
)

type currentApplicationRoute struct {
	Method string
	Path   string
}

var currentWorkbenchApplicationRoutes = []currentApplicationRoute{
	{Method: http.MethodGet, Path: "/api/v1/workbench/context"},
	{Method: http.MethodPut, Path: "/api/v1/workbench/context/effective-organization"},
	{Method: http.MethodGet, Path: "/api/v1/account/profile"},
	{Method: http.MethodGet, Path: "/api/v1/account/organization"},
	{Method: http.MethodGet, Path: "/api/v1/workbench/commercial/overview"},
	{Method: http.MethodPost, Path: "/api/v1/workbench/source-accounts"},
	{Method: http.MethodGet, Path: "/api/v1/workbench/source-accounts"},
	{Method: http.MethodGet, Path: "/api/v1/workbench/source-accounts/:source_account_id"},
	{Method: http.MethodPost, Path: "/api/v1/workbench/source-accounts/:source_account_id/disable"},
	{Method: http.MethodPost, Path: "/api/v1/workbench/source-accounts/:source_account_id/enable"},
}

type currentApplicationFactories struct {
	buildWorkbench      workbenchContextModuleBuilder
	buildSourceAccount  func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error)
	buildCommercial     func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error)
	buildAcquisition    func(*authz.ListingKitAuthorizer, routeAuthDependencies) (kernelmodule.Module, error)
	buildBrowserCapture func(*authz.ListingKitAuthorizer, routeAuthDependencies) (kernelmodule.Module, error)
}

func defaultCurrentApplicationFactories(ctx context.Context) currentApplicationFactories {
	return currentApplicationFactories{
		buildWorkbench: buildDefaultWorkbenchContextModule,
		buildSourceAccount: func(db *gorm.DB, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
			if err := sourceaccountstore.VerifyRuntimePermissions(ctx, db); err != nil {
				return nil, err
			}
			return buildSourceAccountModule(ctx, db, authorizer)
		},
		buildCommercial: func(db *gorm.DB, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
			return buildCommercialReadModuleFromDatabase(ctx, db, authorizer)
		},
	}
}

// NewCurrentApplication assembles the admitted RUN-1 application shell. The
// caller owns both existing database pools, the listener and server lifecycle.
// Construction does not migrate, seed, repair or invoke default legacy feature
// composition.
func NewCurrentApplication(ctx context.Context, sourceAccountDB, commercialDB *gorm.DB, cfg *config.Config, logger *logrus.Logger) (*http.Server, error) {
	if ctx == nil {
		return nil, errors.New("current application startup context unavailable")
	}
	return buildCurrentApplication(ctx, sourceAccountDB, commercialDB, cfg, logger, defaultCurrentApplicationFactories(ctx))
}

func buildCurrentApplication(ctx context.Context, sourceAccountDB, commercialDB *gorm.DB, cfg *config.Config, logger *logrus.Logger, factories currentApplicationFactories) (*http.Server, error) {
	if sourceAccountDB == nil || commercialDB == nil || cfg == nil || logger == nil || !cfg.Workbench.Enabled {
		return nil, errors.New("current application dependencies unavailable")
	}
	if factories.buildWorkbench == nil || factories.buildSourceAccount == nil || factories.buildCommercial == nil {
		return nil, errors.New("current application factories unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("current application startup canceled: %w", err)
	}
	authorizer, err := authz.NewListingKitAuthorizer(cfg.ListingKit.PlatformAdminUsers, cfg.ListingKit.PlatformAdminRoles)
	if err != nil {
		return nil, fmt.Errorf("build current application authorizer: %w", err)
	}
	workbench, err := factories.buildWorkbench(cfg, logger)
	if err != nil {
		return nil, err
	}
	if workbench.module == nil || workbench.authDependencies == nil {
		return nil, errors.New("current application workbench dependencies unavailable")
	}
	workbench.authDependencies.authorizer = authorizer
	sourceAccount, err := factories.buildSourceAccount(sourceAccountDB, authorizer)
	if err != nil {
		return nil, fmt.Errorf("build current source account module: %w", err)
	}
	commercial, err := factories.buildCommercial(commercialDB, authorizer)
	if err != nil {
		return nil, fmt.Errorf("build current commercial module: %w", err)
	}
	modules := []kernelmodule.Module{workbench.module, commercial, sourceAccount}
	if factories.buildAcquisition != nil {
		acquisition, err := factories.buildAcquisition(authorizer, *workbench.authDependencies)
		if err != nil {
			return nil, fmt.Errorf("build current product acquisition module: %w", err)
		}
		if acquisition == nil {
			return nil, errors.New("current product acquisition module unavailable")
		}
		modules = append(modules, acquisition)
	}
	if factories.buildBrowserCapture != nil {
		browser, err := factories.buildBrowserCapture(authorizer, *workbench.authDependencies)
		if err != nil {
			return nil, fmt.Errorf("build current Browser capture module: %w", err)
		}
		if browser == nil {
			return nil, errors.New("current Browser capture module unavailable")
		}
		modules = append(modules, browser)
	}
	bundle, err := buildRuntimeBundleFromModules(cfg, modules)
	if err != nil {
		return nil, err
	}
	if err := validateCurrentApplicationRoutesForSourcing(bundle.routes, factories.buildAcquisition != nil, factories.buildBrowserCapture != nil); err != nil {
		return nil, err
	}
	return buildCurrentApplicationHTTPServer(bundle.routes, *workbench.authDependencies), nil
}

func validateCurrentApplicationRoutes(routes []httproute.Descriptor) error {
	return validateCurrentApplicationRoutesForAcquisition(routes, false)
}

func validateCurrentApplicationRoutesForAcquisition(routes []httproute.Descriptor, acquisition bool) error {
	return validateCurrentApplicationRoutesForSourcing(routes, acquisition, false)
}

func validateCurrentApplicationRoutesForSourcing(routes []httproute.Descriptor, acquisition, browser bool) error {
	admitted := append([]currentApplicationRoute(nil), currentWorkbenchApplicationRoutes...)
	if acquisition {
		admitted = append(admitted,
			currentApplicationRoute{Method: http.MethodPost, Path: productAcquisitionBase},
			currentApplicationRoute{Method: http.MethodPost, Path: productAcquisitionBase + "/verify"},
			currentApplicationRoute{Method: http.MethodGet, Path: productAcquisitionBase + "/:operation_id"},
		)
	}
	if browser {
		admitted = append(admitted,
			currentApplicationRoute{Method: http.MethodPost, Path: browserCaptureBase},
			currentApplicationRoute{Method: http.MethodPost, Path: browserCaptureBase + "/verify"},
			currentApplicationRoute{Method: http.MethodGet, Path: browserCaptureBase + "/by-key/:key"},
			currentApplicationRoute{Method: http.MethodGet, Path: browserCaptureBase + "/:operation_id"},
		)
	}
	expected := make(map[currentApplicationRoute]struct{}, len(admitted))
	for _, route := range admitted {
		expected[route] = struct{}{}
	}
	if len(routes) != len(expected) {
		return fmt.Errorf("current application route contract mismatch: got %d routes, want %d", len(routes), len(expected))
	}
	seen := make(map[currentApplicationRoute]struct{}, len(routes))
	for _, descriptor := range routes {
		route := currentApplicationRoute{Method: descriptor.Method, Path: descriptor.Path}
		if _, ok := expected[route]; !ok {
			return fmt.Errorf("current application route contract contains unadmitted route %s %s", descriptor.Method, descriptor.Path)
		}
		if _, duplicate := seen[route]; duplicate {
			return fmt.Errorf("current application route contract contains duplicate route %s %s", descriptor.Method, descriptor.Path)
		}
		seen[route] = struct{}{}
	}
	return nil
}

func buildCurrentApplicationHTTPServer(routes []httproute.Descriptor, dependencies routeAuthDependencies) *http.Server {
	server := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, routes, dependencies)
	server.ReadTimeout = 32 * time.Second
	server.WriteTimeout = 32 * time.Second
	server.IdleTimeout = 60 * time.Second
	inner := server.Handler
	server.Handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		inner.ServeHTTP(writer, request)
	})
	return server
}
