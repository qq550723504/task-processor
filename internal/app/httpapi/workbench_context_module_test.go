package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/workbenchcontext"
	workbenchcontexthttpapi "task-processor/internal/workbenchcontext/httpapi"
)

type workbenchAuthorizationClientStub struct{}

func (workbenchAuthorizationClientStub) ListOwnProjectAuthorizations(context.Context, string, string, string) ([]authidentity.OrganizationGrant, error) {
	return nil, nil
}

func TestWorkbenchModuleBuildDisabledIsInert(t *testing.T) {
	called := 0
	factories := workbenchContextFactories{
		newHTTPClient: func() *http.Client { called++; return &http.Client{} },
		newVerifier:   func(zitadelruntime.Config) zitadelruntime.Verifier { called++; return mountedVerifierStub{} },
		newAuthorizationClient: func(string, *http.Client) workbenchcontext.AuthorizationClient {
			called++
			return workbenchAuthorizationClientStub{}
		},
		newGrantCache: func() *workbenchcontext.GrantCache { called++; return workbenchcontext.NewGrantCache(nil) },
		newGrantResolver: func(workbenchcontext.AuthorizationClient, *workbenchcontext.GrantCache) *workbenchcontext.GrantResolver {
			called++
			return nil
		},
		newResolver: func(workbenchcontext.GrantLoader, string, string, workbenchcontext.OrganizationBusinessStatusChecker) *workbenchcontext.Resolver {
			called++
			return nil
		},
		newHandler: func() *workbenchcontexthttpapi.Handler { called++; return workbenchcontexthttpapi.NewHandler() },
		newModule:  func(*workbenchcontexthttpapi.Handler) kernelmodule.Module { called++; return nil },
	}

	result, err := buildWorkbenchContextModule(&config.Config{}, logrus.New(), factories)

	require.NoError(t, err)
	require.Nil(t, result.module)
	require.Nil(t, result.authDependencies)
	require.Zero(t, called)
}

func TestWorkbenchModuleBuildConstructsChainOnceWithOneBoundedHTTPClient(t *testing.T) {
	counts := map[string]int{}
	var sharedClient *http.Client
	var verifierClient *http.Client
	var authorizationClientHTTPClient *http.Client
	var resolverProjectID string
	var resolverContractVersion string
	var resolverStatus workbenchcontext.OrganizationBusinessStatusChecker
	var builtResolver *workbenchcontext.Resolver
	factories := workbenchContextFactories{
		newHTTPClient: func() *http.Client {
			counts["http"]++
			sharedClient = &http.Client{Timeout: 7 * time.Second}
			return sharedClient
		},
		newVerifier: func(cfg zitadelruntime.Config) zitadelruntime.Verifier {
			counts["verifier"]++
			verifierClient = cfg.HTTPClient
			require.Equal(t, "https://issuer.example", cfg.IssuerURL)
			require.Equal(t, "client-1", cfg.ClientID)
			require.Equal(t, "project-1", cfg.ProjectID)
			return mountedVerifierStub{identity: mountedVerifiedIdentity()}
		},
		newAuthorizationClient: func(apiURL string, client *http.Client) workbenchcontext.AuthorizationClient {
			counts["authorization-client"]++
			authorizationClientHTTPClient = client
			require.Equal(t, "https://authorization.example", apiURL)
			return workbenchAuthorizationClientStub{}
		},
		newGrantCache: func() *workbenchcontext.GrantCache {
			counts["cache"]++
			return workbenchcontext.NewGrantCache(nil)
		},
		newGrantResolver: func(client workbenchcontext.AuthorizationClient, cache *workbenchcontext.GrantCache) *workbenchcontext.GrantResolver {
			counts["grant-resolver"]++
			return workbenchcontext.NewGrantResolver(client, cache)
		},
		newResolver: func(loader workbenchcontext.GrantLoader, projectID string, contractVersion string, status workbenchcontext.OrganizationBusinessStatusChecker) *workbenchcontext.Resolver {
			counts["resolver"]++
			resolverProjectID = projectID
			resolverContractVersion = contractVersion
			resolverStatus = status
			builtResolver = workbenchcontext.NewResolver(loader, projectID, contractVersion, status)
			return builtResolver
		},
		newHandler: func() *workbenchcontexthttpapi.Handler {
			counts["handler"]++
			return workbenchcontexthttpapi.NewHandler()
		},
		newModule: func(handler *workbenchcontexthttpapi.Handler) kernelmodule.Module {
			counts["module"]++
			return workbenchcontexthttpapi.NewModule(handler)
		},
	}
	cfg := &config.Config{
		Workbench: config.WorkbenchConfig{Enabled: true},
		ListingKit: config.ListingKitConfig{Zitadel: config.ListingKitZitadelConfig{
			IssuerURL: "https://issuer.example", ClientID: "client-1", ClientSecret: "client-secret",
			ProjectID: "project-1", AuthorizationAPIURL: "https://authorization.example",
		}},
	}

	result, err := buildWorkbenchContextModule(cfg, logrus.New(), factories)

	require.NoError(t, err)
	require.NotNil(t, result.module)
	require.NotNil(t, result.authDependencies)
	require.Same(t, sharedClient, verifierClient)
	require.Same(t, sharedClient, authorizationClientHTTPClient)
	require.Equal(t, "project-1", resolverProjectID)
	require.Equal(t, "zitadel-authorization-v2", resolverContractVersion)
	require.Nil(t, resolverStatus)
	require.Same(t, builtResolver, result.authDependencies.organizationResolver)
	require.NotNil(t, result.authDependencies.auditRecorder)
	for _, name := range []string{"http", "verifier", "authorization-client", "cache", "grant-resolver", "resolver", "handler", "module"} {
		require.Equal(t, 1, counts[name], name)
	}
}

func TestWorkbenchRuntimeBundleConsumesInjectedAuthenticationDependencies(t *testing.T) {
	resolver := &mountedCapturingOrganizationResolver{}
	bundle := runtimeBundle{
		routes: []httproute.Descriptor{{
			Method: http.MethodGet, Path: "/api/v1/workbench/context", AuthPolicy: httproute.AuthPolicyVerifiedIdentity,
			OrganizationAccessPolicy: httproute.OrganizationAccessPolicyContextRead,
			Handler:                  func(c *gin.Context) { c.Status(http.StatusNoContent) },
		}},
		authDependencies: &routeAuthDependencies{
			workbenchVerifier:    mountedVerifierStub{identity: mountedVerifiedIdentity()},
			organizationResolver: resolver,
		},
	}

	server, _ := bundle.buildServerBundle(18080)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/context", nil)
	request.Header.Set("Authorization", "Bearer current-request-token")
	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	require.Equal(t, 1, resolver.calls)
}

func TestWorkbenchCompositionRegistersRoutesExactlyOnceWhenEnabled(t *testing.T) {
	workbenchModule := workbenchcontexthttpapi.NewModule(workbenchcontexthttpapi.NewHandler())
	composition := httpFeatureComposition{workbenchContextModule: workbenchModule}

	bundle, err := composition.buildRuntimeBundle(&config.Config{Workbench: config.WorkbenchConfig{Enabled: true}})

	require.NoError(t, err)
	require.Equal(t, 1, routeCount(bundle.routes, http.MethodGet, "/api/v1/workbench/context"))
	require.Equal(t, 1, routeCount(bundle.routes, http.MethodPut, "/api/v1/workbench/context/effective-organization"))
}

func TestBuildBootstrapFailsClosedWhenWorkbenchIsEnabledWithoutDatabase(t *testing.T) {
	configureProductImageRuntimePaths(t)
	restoreAuth := listingkithttpapi.SetListingKitZitadelAuthConfigForTesting(nil)
	t.Cleanup(restoreAuth)
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")
	t.Setenv("TASK_PROCESSOR_WORKBENCH_ENABLED", "true")
	t.Setenv("ZITADEL_ISSUER_URL", "http://127.0.0.1:1")
	t.Setenv("ZITADEL_CLIENT_ID", "client-1")
	t.Setenv("ZITADEL_CLIENT_SECRET", "secret-1")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID", "project-1")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTHORIZATION_API_URL", "http://127.0.0.1:1")

	bootstrap, err := buildBootstrap(logger, Options{ConfigPath: "../../../config/config-test.yaml", Port: 18080})

	require.Error(t, err)
	require.Nil(t, bootstrap)
	require.Contains(t, err.Error(), "durable database configuration is required")
	require.NotContains(t, err.Error(), "sk-test")
}

func routeCount(routes []httproute.Descriptor, method string, path string) int {
	count := 0
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			count++
		}
	}
	return count
}
