package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/workbenchcontext"
	workbenchcontexthttpapi "task-processor/internal/workbenchcontext/httpapi"
)

const workbenchAuthorizationContractVersion = "zitadel-authorization-v2"

type workbenchContextBuildResult struct {
	module           kernelmodule.Module
	authDependencies *routeAuthDependencies
}

type workbenchContextModuleBuilder func(*config.Config) (workbenchContextBuildResult, error)

func buildDefaultWorkbenchContextModule(cfg *config.Config) (workbenchContextBuildResult, error) {
	return buildWorkbenchContextModule(cfg, defaultWorkbenchContextFactories())
}

type workbenchContextFactories struct {
	newHTTPClient          func() *http.Client
	newVerifier            func(zitadel.Config) zitadel.Verifier
	newAuthorizationClient func(string, *http.Client) workbenchcontext.AuthorizationClient
	newGrantCache          func() *workbenchcontext.GrantCache
	newGrantResolver       func(workbenchcontext.AuthorizationClient, *workbenchcontext.GrantCache) *workbenchcontext.GrantResolver
	newResolver            func(workbenchcontext.GrantLoader, string, string, workbenchcontext.OrganizationBusinessStatusChecker) *workbenchcontext.Resolver
	newHandler             func() *workbenchcontexthttpapi.Handler
	newModule              func(*workbenchcontexthttpapi.Handler) kernelmodule.Module
}

func defaultWorkbenchContextFactories() workbenchContextFactories {
	return workbenchContextFactories{
		newHTTPClient: func() *http.Client {
			return &http.Client{Timeout: 5 * time.Second}
		},
		newVerifier: zitadel.NewVerifier,
		newAuthorizationClient: func(apiURL string, client *http.Client) workbenchcontext.AuthorizationClient {
			return zitadel.NewAuthorizationClient(apiURL, client)
		},
		newGrantCache: func() *workbenchcontext.GrantCache {
			return workbenchcontext.NewGrantCache(nil)
		},
		newGrantResolver: workbenchcontext.NewGrantResolver,
		newResolver: func(loader workbenchcontext.GrantLoader, projectID string, contractVersion string, status workbenchcontext.OrganizationBusinessStatusChecker) *workbenchcontext.Resolver {
			return workbenchcontext.NewResolver(loader, projectID, contractVersion, status)
		},
		newHandler: workbenchcontexthttpapi.NewHandler,
		newModule:  workbenchcontexthttpapi.NewModule,
	}
}

func buildWorkbenchContextModule(cfg *config.Config, factories workbenchContextFactories) (workbenchContextBuildResult, error) {
	if cfg == nil || !cfg.Workbench.Enabled {
		return workbenchContextBuildResult{}, nil
	}
	if validationErrors := config.ValidateWorkbenchConfig(&cfg.Workbench, &cfg.ListingKit.Zitadel); len(validationErrors) > 0 {
		return workbenchContextBuildResult{}, fmt.Errorf("build workbench context: %w", errors.Join(validationErrors...))
	}

	httpClient := factories.newHTTPClient()
	zitadelConfig := cfg.ListingKit.Zitadel
	verifier := factories.newVerifier(zitadel.Config{
		IssuerURL:    zitadelConfig.IssuerURL,
		ClientID:     zitadelConfig.ClientID,
		ClientSecret: zitadelConfig.ClientSecret,
		ProjectID:    zitadelConfig.ProjectID,
		HTTPClient:   httpClient,
	})
	authorizationClient := factories.newAuthorizationClient(zitadelConfig.AuthorizationAPIURL, httpClient)
	cache := factories.newGrantCache()
	grantResolver := factories.newGrantResolver(authorizationClient, cache)
	resolver := factories.newResolver(grantResolver, zitadelConfig.ProjectID, workbenchAuthorizationContractVersion, nil)
	handler := factories.newHandler()
	module := factories.newModule(handler)
	authDependencies := newRouteAuthDependencies()
	authDependencies.workbenchVerifier = verifier
	authDependencies.organizationResolver = resolver
	return workbenchContextBuildResult{module: module, authDependencies: &authDependencies}, nil
}
