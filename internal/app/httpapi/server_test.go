package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	"task-processor/internal/authidentity"
	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	promptmgmtapi "task-processor/internal/promptmgmt/api"
	sdshttpapi "task-processor/internal/sds/httpapi"
	"task-processor/internal/sdslogin"
	"task-processor/internal/sheinlogin"
	"task-processor/internal/taskrpcapi"
	"task-processor/internal/tenantbridge"
	"task-processor/internal/workbenchcontext"
	workbenchcontexthttpapi "task-processor/internal/workbenchcontext/httpapi"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestBuildHTTPServerFromRoutesAtBindsLoopback(t *testing.T) {
	server := buildHTTPServerFromRoutesAt("127.0.0.1", 18085, nil)
	if server.Addr != "127.0.0.1:18085" {
		t.Fatalf("server address = %q, want loopback-only listener", server.Addr)
	}
	if got := buildHTTPServerFromRoutes(18085, nil).Addr; got != ":18085" {
		t.Fatalf("default server address = %q, want backward-compatible wildcard", got)
	}
}

type mountedOrganizationResolverStub struct {
	events *[]string
	roles  []string
	bearer *string
}

type mountedVerifierStub struct {
	identity authidentity.AuthenticatedIdentity
	err      error
	events   *[]string
	token    *string
}

func (stub mountedVerifierStub) Verify(_ context.Context, token string) (authidentity.AuthenticatedIdentity, error) {
	if stub.events != nil {
		*stub.events = append(*stub.events, "authentication")
	}
	if stub.token != nil {
		*stub.token = token
	}
	return stub.identity, stub.err
}

type appHTTPRoundTripFunc func(*http.Request) (*http.Response, error)

type workbenchBodyBoundaryReader struct {
	payload          []byte
	offset           int
	readsPastPayload int
}

func (reader *workbenchBodyBoundaryReader) Read(buffer []byte) (int, error) {
	if reader.offset >= len(reader.payload) {
		reader.readsPastPayload++
		return 0, io.EOF
	}
	read := copy(buffer, reader.payload[reader.offset:])
	reader.offset += read
	return read, nil
}

func (roundTrip appHTTPRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func mountedVerifiedIdentity() authidentity.AuthenticatedIdentity {
	return authidentity.AuthenticatedIdentity{UserID: "user-1", TokenExpiresAt: time.Now().Add(time.Minute)}
}

type mountedOrganizationResolverError struct{ err error }

type mountedCapturingOrganizationResolver struct {
	calls  int
	target string
}

func (resolver *mountedCapturingOrganizationResolver) Resolve(_ context.Context, _ httproute.OrganizationAccessPolicy, input workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error) {
	resolver.calls++
	resolver.target = input.RequestedOrganizationID
	identity := input.Identity
	identity.EffectiveOrganizationID = input.RequestedOrganizationID
	return identity, nil
}

func (stub mountedOrganizationResolverError) Resolve(context.Context, httproute.OrganizationAccessPolicy, workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error) {
	return authidentity.AuthenticatedIdentity{}, stub.err
}

func TestWorkbenchLiveSwitchRejectsInvalidTargetBeforeGrantLookup(t *testing.T) {
	testCases := []struct {
		name   string
		body   string
		header string
	}{
		{name: "unknown field", body: `{"organizationId":"org-a","unexpected":true}`},
		{name: "blank", body: `{"organizationId":" "}`},
		{name: "trailing", body: `{"organizationId":"org-a"}{}`},
		{name: "header mismatch", body: `{"organizationId":"org-a"}`, header: "org-b"},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &mountedCapturingOrganizationResolver{}
			handlerCalls := 0
			router := gin.New()
			mountRoutesWithAuthDependencies(router, []httproute.Descriptor{{
				Method:                     http.MethodPut,
				Path:                       "/api/v1/workbench/context/effective-organization",
				AuthPolicy:                 httproute.AuthPolicyVerifiedIdentity,
				OrganizationAccessPolicy:   httproute.OrganizationAccessPolicyLiveSwitch,
				OrganizationTargetResolver: workbenchcontexthttpapi.ResolveSwitchOrganizationTarget,
				Handler: func(c *gin.Context) {
					handlerCalls++
					c.Status(http.StatusNoContent)
				},
			}}, routeAuthDependencies{
				workbenchVerifier:    mountedVerifierStub{identity: mountedVerifiedIdentity()},
				organizationResolver: resolver,
			})
			request := httptest.NewRequest(http.MethodPut, "/api/v1/workbench/context/effective-organization", strings.NewReader(tt.body))
			request.Header.Set("Authorization", "Bearer current-request-token")
			request.Header.Set("X-Requested-Organization-ID", tt.header)
			request.Header.Set("X-Request-ID", "req-123")
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
			require.JSONEq(t, `{"code":"INVALID_REQUEST","message":"Request is invalid","requestId":"req-123","fieldErrors":[]}`, response.Body.String())
			require.Zero(t, resolver.calls)
			require.Zero(t, handlerCalls)
		})
	}
}

func TestWorkbenchLiveSwitchRejects4097ByteBodyWithoutReadingPastLimitBeforeGrantLookup(t *testing.T) {
	reader := &workbenchBodyBoundaryReader{payload: []byte(strings.Repeat("x", 4097))}
	resolver := &mountedCapturingOrganizationResolver{}
	handlerCalls := 0
	router := gin.New()
	mountRoutesWithAuthDependencies(router, []httproute.Descriptor{{
		Method:                     http.MethodPut,
		Path:                       "/api/v1/workbench/context/effective-organization",
		AuthPolicy:                 httproute.AuthPolicyVerifiedIdentity,
		OrganizationAccessPolicy:   httproute.OrganizationAccessPolicyLiveSwitch,
		OrganizationTargetResolver: workbenchcontexthttpapi.ResolveSwitchOrganizationTarget,
		Handler: func(c *gin.Context) {
			handlerCalls++
			c.Status(http.StatusNoContent)
		},
	}}, routeAuthDependencies{
		workbenchVerifier:    mountedVerifierStub{identity: mountedVerifiedIdentity()},
		organizationResolver: resolver,
	})
	request := httptest.NewRequest(http.MethodPut, "/api/v1/workbench/context/effective-organization", reader)
	request.Header.Set("Authorization", "Bearer current-request-token")
	request.Header.Set("X-Request-ID", "req-oversize")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
	require.JSONEq(t, `{"code":"INVALID_REQUEST","message":"Request is invalid","requestId":"req-oversize","fieldErrors":[]}`, response.Body.String())
	require.Zero(t, reader.readsPastPayload)
	require.Zero(t, resolver.calls)
	require.Zero(t, handlerCalls)
}

func TestWorkbenchLiveSwitchUsesBodyTargetAndRestoresBodyForHandler(t *testing.T) {
	const body = `{"organizationId":"org-a"}`
	resolver := &mountedCapturingOrganizationResolver{}
	handlerBody := ""
	router := gin.New()
	mountRoutesWithAuthDependencies(router, []httproute.Descriptor{{
		Method:                     http.MethodPut,
		Path:                       "/api/v1/workbench/context/effective-organization",
		AuthPolicy:                 httproute.AuthPolicyVerifiedIdentity,
		OrganizationAccessPolicy:   httproute.OrganizationAccessPolicyLiveSwitch,
		OrganizationTargetResolver: workbenchcontexthttpapi.ResolveSwitchOrganizationTarget,
		Handler: func(c *gin.Context) {
			data, err := io.ReadAll(c.Request.Body)
			require.NoError(t, err)
			handlerBody = string(data)
			c.Status(http.StatusNoContent)
		},
	}}, routeAuthDependencies{
		workbenchVerifier:    mountedVerifierStub{identity: mountedVerifiedIdentity()},
		organizationResolver: resolver,
	})
	request := httptest.NewRequest(http.MethodPut, "/api/v1/workbench/context/effective-organization", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer current-request-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	require.Equal(t, 1, resolver.calls)
	require.Equal(t, "org-a", resolver.target)
	require.Equal(t, body, handlerBody)
}

func (stub mountedOrganizationResolverStub) Resolve(_ context.Context, _ httproute.OrganizationAccessPolicy, input workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error) {
	*stub.events = append(*stub.events, "organization")
	if stub.bearer != nil {
		*stub.bearer = input.BearerToken
	}
	identity := input.Identity
	identity.TenantID = "org-a"
	identity.EffectiveOrganizationID = "org-a"
	identity.Roles = append([]string(nil), stub.roles...)
	return identity, nil
}

func TestWorkbenchAuthHandlerOrderIsAuthenticationOrganizationRoleThenHandler(t *testing.T) {
	events := []string{}
	resolvedBearer := ""
	router := gin.New()
	mountRoutesWithAuthDependencies(router, []httproute.Descriptor{{
		Method:                   http.MethodGet,
		Path:                     "/api/v1/workbench/order",
		AuthPolicy:               httproute.AuthPolicyVerifiedIdentity,
		OrganizationAccessPolicy: httproute.OrganizationAccessPolicyCachedRead,
		Handler: func(c *gin.Context) {
			events = append(events, "handler")
			c.Status(http.StatusNoContent)
		},
	}}, routeAuthDependencies{
		workbenchVerifier:    mountedVerifierStub{identity: mountedVerifiedIdentity(), events: &events},
		organizationResolver: mountedOrganizationResolverStub{events: &events, roles: []string{"listingkit_admin"}, bearer: &resolvedBearer},
		roleMiddleware: func(httproute.Descriptor) gin.HandlerFunc {
			return func(c *gin.Context) {
				events = append(events, "role")
				identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
				if !ok || identity.EffectiveOrganizationID != "org-a" || len(identity.Roles) != 1 || identity.Roles[0] != "listingkit_admin" {
					c.AbortWithStatus(http.StatusForbidden)
					return
				}
				c.Next()
			}
		},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/order", nil)
	request.Header.Set("Authorization", "Bearer pre-auth-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	require.Equal(t, []string{"authentication", "organization", "role", "handler"}, events)
	require.Equal(t, "pre-auth-token", resolvedBearer)
}

func TestProductionAuthShapeSkipsLegacyAllowlistBeforeWorkbenchOrganizationResolution(t *testing.T) {
	zitadel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"introspection_endpoint": "http://" + r.Host + "/oauth/v2/introspect",
			})
		case "/oauth/v2/introspect":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"active": true, "sub": "user-1", "exp": time.Now().Add(time.Minute).Unix(),
				"urn:zitadel:iam:user:resourceowner:id": "org-home",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer zitadel.Close()
	restore := listingkithttpapi.SetListingKitZitadelAuthConfigForTesting(nil)
	t.Cleanup(restore)
	listingkithttpapi.ConfigureListingKitZitadelAuth(config.ListingKitZitadelConfig{
		IssuerURL: zitadel.URL, ClientID: "listingkit-client", ProjectID: "project-1",
		AuthorizationRequired: true, AllowedTenantIDs: []string{"org-other"},
	})

	events := []string{}
	dependencies := newRouteAuthDependencies()
	dependencies.workbenchVerifier = zitadelruntime.NewVerifier(zitadelruntime.Config{
		IssuerURL: zitadel.URL, ClientID: "listingkit-client", ProjectID: "project-1", HTTPClient: zitadel.Client(),
	})
	dependencies.organizationResolver = mountedOrganizationResolverStub{events: &events, roles: []string{"listingkit_viewer"}}
	router := gin.New()
	mountRoutesWithAuthDependencies(router, []httproute.Descriptor{
		{
			Method: http.MethodGet, Path: "/api/v1/workbench/context", AuthPolicy: httproute.AuthPolicyVerifiedIdentity,
			OrganizationAccessPolicy: httproute.OrganizationAccessPolicyCachedRead,
			Handler: func(c *gin.Context) {
				events = append(events, "workbench-handler")
				c.Status(http.StatusNoContent)
			},
		},
		{
			Method: http.MethodGet, Path: "/api/v1/listing-kits/tasks", Module: "listing-kit",
			AuthPolicy: httproute.AuthPolicyVerifiedIdentity,
			Handler:    func(c *gin.Context) { t.Fatal("legacy handler ran after allowlist denial") },
		},
	}, dependencies)

	workbenchRequest := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/context", nil)
	workbenchRequest.Header.Set("Authorization", "Bearer user-token")
	workbenchResponse := httptest.NewRecorder()
	router.ServeHTTP(workbenchResponse, workbenchRequest)
	require.Equal(t, http.StatusNoContent, workbenchResponse.Code, workbenchResponse.Body.String())
	require.Equal(t, []string{"organization", "workbench-handler"}, events)

	legacyRequest := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks", nil)
	legacyRequest.Header.Set("Authorization", "Bearer user-token")
	legacyResponse := httptest.NewRecorder()
	router.ServeHTTP(legacyResponse, legacyRequest)
	require.Equal(t, http.StatusForbidden, legacyResponse.Code, legacyResponse.Body.String())
	require.Contains(t, legacyResponse.Body.String(), "zitadel_access_denied")
}

func TestLegacyBlankOrganizationPolicySkipsResolver(t *testing.T) {
	assertOrganizationPolicySkipsResolver(t, "")
}

func TestExplicitNoneOrganizationPolicySkipsResolver(t *testing.T) {
	assertOrganizationPolicySkipsResolver(t, httproute.OrganizationAccessPolicyNone)
}

func TestWorkbenchOrganizationPolicyFailsClosedWithoutAuthenticationMiddleware(t *testing.T) {
	router := gin.New()
	route := httproute.Descriptor{
		Method: http.MethodGet, Path: "/workbench", AuthPolicy: httproute.AuthPolicyVerifiedIdentity,
		OrganizationAccessPolicy: httproute.OrganizationAccessPolicyCachedRead,
		Handler: func(c *gin.Context) {
			t.Fatal("handler ran without authentication middleware")
		},
	}
	router.Handle(route.Method, route.Path, append(routeAuthHandlers(route, nil), route.Handler)...)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/workbench", nil))

	require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), "AUTHENTICATION_REQUIRED")
}

func TestWorkbenchOrganizationResolverNilFailsClosed(t *testing.T) {
	router := gin.New()
	mountRoutesWithAuthDependencies(router, []httproute.Descriptor{{
		Method: http.MethodGet, Path: "/workbench", AuthPolicy: httproute.AuthPolicyVerifiedIdentity,
		OrganizationAccessPolicy: httproute.OrganizationAccessPolicyCachedRead,
		Handler:                  func(c *gin.Context) { t.Fatal("handler ran without organization resolver") },
	}}, routeAuthDependencies{workbenchVerifier: mountedVerifierStub{identity: mountedVerifiedIdentity()}})
	request := httptest.NewRequest(http.MethodGet, "/workbench", nil)
	request.Header.Set("Authorization", "Bearer current-request-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), "DEPENDENCY_UNAVAILABLE")
}

func assertOrganizationPolicySkipsResolver(t *testing.T, policy httproute.OrganizationAccessPolicy) {
	t.Helper()
	events := []string{}
	router := gin.New()
	mountRoutesWithAuthDependencies(router, []httproute.Descriptor{{
		Method: http.MethodGet, Path: "/policy", AuthPolicy: httproute.AuthPolicyPublic,
		OrganizationAccessPolicy: policy,
		Handler: func(c *gin.Context) {
			events = append(events, "handler")
			c.Status(http.StatusNoContent)
		},
	}}, routeAuthDependencies{
		organizationResolver: mountedOrganizationResolverStub{events: &events},
	})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/policy", nil))

	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	require.Equal(t, []string{"handler"}, events)
}

type mountedGrantLoader struct {
	grants []authidentity.OrganizationGrant
}

func (loader mountedGrantLoader) Load(context.Context, workbenchcontext.GrantSource, workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	return workbenchcontext.GrantResult{Grants: loader.grants}, nil
}

func (mountedGrantLoader) Invalidate(string, string) {}

type poisonLegacyTenantResolver struct {
	calls int
}

func (resolver *poisonLegacyTenantResolver) ResolveLegacyTenantID(context.Context, string) (int64, bool, error) {
	resolver.calls++
	return 0, false, errors.New("legacy numeric tenant resolver must not be reached by workbench routes")
}

func TestMountedWorkbenchContextDoesNotReachLegacyNumericTenantResolver(t *testing.T) {
	legacyResolver := &poisonLegacyTenantResolver{}
	restoreLegacyResolver := tenantbridge.ConfigureLegacyTenantResolver(legacyResolver)
	t.Cleanup(restoreLegacyResolver)

	now := time.Now().UTC()
	resolver := workbenchcontext.NewResolver(mountedGrantLoader{grants: []authidentity.OrganizationGrant{{
		OrganizationID: "org-canonical", ProjectID: "project-1", Roles: []string{"listingkit_viewer"},
	}}}, "project-1", "v1", nil)
	router := gin.New()
	mountRoutesWithAuthDependencies(router, []httproute.Descriptor{{
		Method: http.MethodGet, Path: "/api/v1/workbench/context", AuthPolicy: httproute.AuthPolicyVerifiedIdentity,
		OrganizationAccessPolicy: httproute.OrganizationAccessPolicyContextRead,
		Handler: func(c *gin.Context) {
			identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
			require.True(t, ok)
			require.Equal(t, "org-canonical", identity.TenantID)
			require.Equal(t, "org-canonical", identity.EffectiveOrganizationID)
			require.Equal(t, []string{"listingkit_viewer"}, identity.Roles)
			c.JSON(http.StatusOK, gin.H{
				"effectiveOrganizationId": identity.EffectiveOrganizationID,
				"roles":                   identity.Roles,
			})
		},
	}}, routeAuthDependencies{
		workbenchVerifier: mountedVerifierStub{identity: authidentity.AuthenticatedIdentity{
			TenantID: "246", UserID: "user-1", HomeOrganizationID: "org-canonical",
			Roles: []string{"legacy_admin"}, TokenExpiresAt: now.Add(time.Minute),
		}},
		organizationResolver: resolver,
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/context", nil)
	request.Header.Set("Authorization", "Bearer current-request-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.JSONEq(t, `{"effectiveOrganizationId":"org-canonical","roles":["listingkit_viewer"]}`, response.Body.String())
	require.Zero(t, legacyResolver.calls, "mounted Workbench route reached the legacy numeric tenant resolver")
}

func TestWorkbenchScopedRoleDoesNotCarryAdminFromOrganizationAToViewerOrganizationB(t *testing.T) {
	restore := listingkithttpapi.SetListingKitZitadelAuthConfigForTesting(nil)
	t.Cleanup(restore)
	now := time.Now().UTC()
	resolver := workbenchcontext.NewResolver(mountedGrantLoader{grants: []authidentity.OrganizationGrant{
		{OrganizationID: "org-a", ProjectID: "project-1", Roles: []string{"listingkit_admin"}},
		{OrganizationID: "org-b", ProjectID: "project-1", Roles: []string{"listingkit_viewer"}},
	}}, "project-1", "v1", nil)
	handlerCalls := 0
	router := gin.New()
	mountRoutesWithAuthDependencies(router, []httproute.Descriptor{{
		Method:                   http.MethodPost,
		Path:                     "/api/v1/workbench/admin-action",
		Module:                   "listing-kit-admin",
		Permission:               authz.PermissionListingKitAdminWrite,
		AuthPolicy:               httproute.AuthPolicyVerifiedIdentity,
		OrganizationAccessPolicy: httproute.OrganizationAccessPolicyCachedRead,
		Handler: func(c *gin.Context) {
			handlerCalls++
			c.Status(http.StatusNoContent)
		},
	}}, routeAuthDependencies{
		workbenchVerifier: mountedVerifierStub{identity: authidentity.AuthenticatedIdentity{
			TenantID: "org-a", UserID: "user-1", HomeOrganizationID: "org-a",
			Roles: []string{"listingkit_admin"}, TokenExpiresAt: now.Add(time.Minute),
		}},
		organizationResolver: resolver,
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/admin-action", nil)
	request.Header.Set("Authorization", "Bearer current-request-token")
	request.Header.Set("X-Requested-Organization-ID", "org-b")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
	require.Equal(t, 0, handlerCalls)
	var deniedBody struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &deniedBody))
	require.Equal(t, "PERMISSION_DENIED", deniedBody.Code)

	request = httptest.NewRequest(http.MethodPost, "/api/v1/workbench/admin-action", nil)
	request.Header.Set("Authorization", "Bearer current-request-token")
	request.Header.Set("X-Requested-Organization-ID", "org-a")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	require.Equal(t, 1, handlerCalls)
}

func TestWorkbenchOrganizationContextReadClearsLegacyBusinessScopeWhenSelectionIsRequired(t *testing.T) {
	now := time.Now().UTC()
	resolver := workbenchcontext.NewResolver(mountedGrantLoader{grants: []authidentity.OrganizationGrant{
		{OrganizationID: "org-a", ProjectID: "project-1", Roles: []string{"listingkit_admin"}},
		{OrganizationID: "org-b", ProjectID: "project-1", Roles: []string{"listingkit_viewer"}},
	}}, "project-1", "v1", nil)
	router := gin.New()
	mountRoutesWithAuthDependencies(router, []httproute.Descriptor{{
		Method: http.MethodGet, Path: "/api/v1/workbench/context", AuthPolicy: httproute.AuthPolicyVerifiedIdentity,
		OrganizationAccessPolicy: httproute.OrganizationAccessPolicyContextRead,
		Handler: func(c *gin.Context) {
			identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
			require.True(t, ok)
			require.Equal(t, "user-1", identity.UserID)
			require.Equal(t, "org-home", identity.HomeOrganizationID)
			require.Empty(t, identity.TenantID)
			require.Empty(t, identity.EffectiveOrganizationID)
			require.Empty(t, identity.Roles)
			require.Empty(t, c.GetHeader("X-Tenant-ID"))
			require.Empty(t, c.GetHeader("tenant-id"))
			require.Empty(t, c.GetHeader("X-User-Roles"))
			c.Status(http.StatusNoContent)
		},
	}}, routeAuthDependencies{
		workbenchVerifier: mountedVerifierStub{identity: authidentity.AuthenticatedIdentity{
			TenantID: "org-home", UserID: "user-1", HomeOrganizationID: "org-home",
			Roles: []string{"legacy_admin"}, TokenExpiresAt: now.Add(time.Minute),
		}},
		organizationResolver: resolver,
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/context", nil)
	request.Header.Set("Authorization", "Bearer current-request-token")
	request.Header.Set("X-Tenant-ID", "org-home")
	request.Header.Set("tenant-id", "org-home")
	request.Header.Set("X-User-Roles", "legacy_admin")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
}

func TestWorkbenchOrganizationAuthenticationFailureUsesStableProtocol(t *testing.T) {
	router := gin.New()
	mountRoutesWithAuthDependencies(router, []httproute.Descriptor{{
		Method:                   http.MethodGet,
		Path:                     "/api/v1/workbench/context",
		AuthPolicy:               httproute.AuthPolicyVerifiedIdentity,
		OrganizationAccessPolicy: httproute.OrganizationAccessPolicyContextRead,
		Handler: func(c *gin.Context) {
			t.Fatal("handler ran after authentication failure")
		},
	}}, routeAuthDependencies{
		workbenchVerifier:    mountedVerifierStub{err: errors.New("provider said secret-token and org-other")},
		organizationResolver: mountedOrganizationResolverStub{events: &[]string{}},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/context", nil)
	request.Header.Set("Authorization", "Bearer secret-token")
	request.Header.Set("X-Request-ID", "req-123")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusUnauthorized, response.Code)
	var body struct {
		Code        string `json:"code"`
		Message     string `json:"message"`
		RequestID   string `json:"requestId"`
		FieldErrors []any  `json:"fieldErrors"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, "AUTHENTICATION_REQUIRED", body.Code)
	require.Equal(t, "req-123", body.RequestID)
	require.NotNil(t, body.FieldErrors)
	require.NotContains(t, response.Body.String(), "secret-token")
	require.NotContains(t, response.Body.String(), "org-other")
}

func TestWorkbenchVerifierDependencyFailureUsesStableUnavailableProtocol(t *testing.T) {
	verifier := zitadelruntime.NewVerifier(zitadelruntime.Config{
		IssuerURL: "https://issuer.example", ClientID: "listingkit-client",
		HTTPClient: &http.Client{Transport: appHTTPRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial failed with secret-token and org-other")
		})},
	})
	router := gin.New()
	mountRoutesWithAuthDependencies(router, []httproute.Descriptor{{
		Method: http.MethodGet, Path: "/api/v1/workbench/context", AuthPolicy: httproute.AuthPolicyVerifiedIdentity,
		OrganizationAccessPolicy: httproute.OrganizationAccessPolicyContextRead,
		Handler:                  func(c *gin.Context) { t.Fatal("handler ran after verifier dependency failure") },
	}}, routeAuthDependencies{
		workbenchVerifier:    verifier,
		organizationResolver: mountedOrganizationResolverStub{events: &[]string{}},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/context", nil)
	request.Header.Set("Authorization", "Bearer secret-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), "DEPENDENCY_UNAVAILABLE")
	require.NotContains(t, response.Body.String(), "secret-token")
	require.NotContains(t, response.Body.String(), "org-other")
}

func TestWorkbenchVerifierPathPreservesGinRecoveryOnHandlerPanic(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())
	mountRoutesWithAuthDependencies(router, []httproute.Descriptor{{
		Method: http.MethodGet, Path: "/workbench", AuthPolicy: httproute.AuthPolicyVerifiedIdentity,
		OrganizationAccessPolicy: httproute.OrganizationAccessPolicyCachedRead,
		Handler:                  func(*gin.Context) { panic("handler panic") },
	}}, routeAuthDependencies{
		workbenchVerifier:    mountedVerifierStub{identity: mountedVerifiedIdentity()},
		organizationResolver: mountedOrganizationResolverStub{events: &[]string{}, roles: []string{"listingkit_viewer"}},
	})
	request := httptest.NewRequest(http.MethodGet, "/workbench", nil)
	request.Header.Set("Authorization", "Bearer current-request-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestWorkbenchOrganizationScopedSuccessStreamsWithoutResponseBuffering(t *testing.T) {
	restore := listingkithttpapi.SetListingKitZitadelAuthConfigForTesting(nil)
	t.Cleanup(restore)
	events := []string{}
	response := httptest.NewRecorder()
	flushedBeforeReturn := false
	visibleStatusBeforeReturn := 0
	visibleBodyBeforeReturn := ""
	visibleHeaderBeforeReturn := ""
	router := gin.New()
	mountRoutesWithAuthDependencies(router, []httproute.Descriptor{{
		Method: http.MethodGet, Path: "/workbench/stream", Module: "listing-kit-admin",
		Permission: authz.PermissionListingKitAdminRead, AuthPolicy: httproute.AuthPolicyVerifiedIdentity,
		OrganizationAccessPolicy: httproute.OrganizationAccessPolicyCachedRead,
		Handler: func(c *gin.Context) {
			c.Header("X-Workbench-Stream", "visible")
			c.Header("Content-Length", "5")
			c.Status(http.StatusPartialContent)
			_, _ = c.Writer.WriteString("hello")
			c.Writer.Flush()
			flushedBeforeReturn = response.Flushed
			visibleStatusBeforeReturn = response.Code
			visibleBodyBeforeReturn = response.Body.String()
			visibleHeaderBeforeReturn = response.Header().Get("X-Workbench-Stream")
		},
	}}, routeAuthDependencies{
		workbenchVerifier:    mountedVerifierStub{identity: mountedVerifiedIdentity()},
		organizationResolver: mountedOrganizationResolverStub{events: &events, roles: []string{"listingkit_admin"}},
	})
	request := httptest.NewRequest(http.MethodGet, "/workbench/stream", nil)
	request.Header.Set("Authorization", "Bearer current-request-token")

	router.ServeHTTP(response, request)

	require.True(t, flushedBeforeReturn)
	require.Equal(t, http.StatusPartialContent, visibleStatusBeforeReturn)
	require.Equal(t, "hello", visibleBodyBeforeReturn)
	require.Equal(t, "visible", visibleHeaderBeforeReturn)
	require.Equal(t, http.StatusPartialContent, response.Code)
	require.Equal(t, "hello", response.Body.String())
	require.Equal(t, "visible", response.Header().Get("X-Workbench-Stream"))
	require.Equal(t, "5", response.Header().Get("Content-Length"))
}

func TestWorkbenchOrganizationErrorsUseStableNonEnumeratingProtocol(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "selection", err: workbenchcontext.ErrOrganizationSelectionRequired, wantStatus: http.StatusConflict, wantCode: "ORGANIZATION_SELECTION_REQUIRED"},
		{name: "denied", err: workbenchcontext.ErrOrganizationAccessDenied, wantStatus: http.StatusForbidden, wantCode: "ORGANIZATION_ACCESS_DENIED"},
		{name: "revoked", err: workbenchcontext.ErrOrganizationAccessRevoked, wantStatus: http.StatusForbidden, wantCode: "ORGANIZATION_ACCESS_REVOKED"},
		{name: "suspended", err: workbenchcontext.ErrOrganizationSuspended, wantStatus: http.StatusForbidden, wantCode: "ORGANIZATION_SUSPENDED"},
		{name: "dependency", err: errors.New("provider exposed org-other and secret-token"), wantStatus: http.StatusServiceUnavailable, wantCode: "DEPENDENCY_UNAVAILABLE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			mountRoutesWithAuthDependencies(router, []httproute.Descriptor{{
				Method: http.MethodGet, Path: "/api/v1/workbench/context", AuthPolicy: httproute.AuthPolicyVerifiedIdentity,
				OrganizationAccessPolicy: httproute.OrganizationAccessPolicyCachedRead,
				Handler:                  func(c *gin.Context) { t.Fatal("handler ran after organization resolution error") },
			}}, routeAuthDependencies{
				workbenchVerifier:    mountedVerifierStub{identity: mountedVerifiedIdentity()},
				organizationResolver: mountedOrganizationResolverError{err: tt.err},
			})
			request := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/context", nil)
			request.Header.Set("Authorization", "Bearer secret-token")
			request.Header.Set("X-Requested-Organization-ID", "org-other")
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			require.Equal(t, tt.wantStatus, response.Code, response.Body.String())
			var body struct {
				Code string `json:"code"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			require.Equal(t, tt.wantCode, body.Code)
			require.NotContains(t, response.Body.String(), "org-other")
			require.NotContains(t, response.Body.String(), "secret-token")
		})
	}
}

func TestListingKitInvitationRouteRejectsForgedAdminHeadersWhenGlobalFlagsAreFalse(t *testing.T) {
	restore := listingkithttpapi.SetListingKitZitadelAuthConfigForTesting(nil)
	t.Cleanup(restore)
	listingkithttpapi.ConfigureListingKitZitadelAuth(config.ListingKitZitadelConfig{
		IssuerURL:             "https://issuer.example",
		ClientID:              "listingkit-client",
		AuthorizationRequired: false,
	})
	if err := listingkithttpapi.ConfigureListingKitAuthorization(nil, []string{"platform_admin"}); err != nil {
		t.Fatal(err)
	}

	handlerCalled := false
	server := buildHTTPServerFromRoutes(0, []httproute.Descriptor{{
		Method: http.MethodPost,
		Path:   "/api/v1/listing-kits/platform/tenants/:tenant_id/members/invitations",
		Module: "listing-kit-platform-admin",
		Handler: func(c *gin.Context) {
			handlerCalled = true
			c.Status(http.StatusCreated)
		},
	}})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/platform/tenants/org-1/members/invitations", strings.NewReader(`{"email":"victim@example.com"}`))
	request.Header.Set("X-User-ID", "forged-admin")
	request.Header.Set("X-User-Roles", "platform_admin")
	response := httptest.NewRecorder()

	server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusUnauthorized, response.Body.String())
	}
	if handlerCalled {
		t.Fatal("invitation handler ran for forged identity headers")
	}
}

func TestZitadelSMSWebhookBypassesBearerWithoutRelaxingListingKitRoutes(t *testing.T) {
	webhookCalled := false
	listingKitCalled := false
	server := buildHTTPServerFromRoutes(0, []httproute.Descriptor{
		{
			Method: http.MethodPost,
			Path:   "/api/v1/listing-kits/integrations/zitadel/sms",
			Module: "listing-kit-zitadel-sms-webhook",
			Handler: func(c *gin.Context) {
				webhookCalled = true
				c.Status(http.StatusNoContent)
			},
		},
		{
			Method: http.MethodGet,
			Path:   "/api/v1/listing-kits/tasks",
			Module: "listing-kit",
			Handler: func(c *gin.Context) {
				listingKitCalled = true
				c.Status(http.StatusOK)
			},
		},
	})

	webhookResponse := httptest.NewRecorder()
	server.Handler.ServeHTTP(webhookResponse, httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/integrations/zitadel/sms", nil))
	require.Equal(t, http.StatusNoContent, webhookResponse.Code)
	require.True(t, webhookCalled)

	listingKitResponse := httptest.NewRecorder()
	server.Handler.ServeHTTP(listingKitResponse, httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks", nil))
	require.Equal(t, http.StatusUnauthorized, listingKitResponse.Code)
	require.False(t, listingKitCalled)
}

func TestListingKitPromptRoutesRequireVerifiedIdentity(t *testing.T) {
	server := buildHTTPServerFromRoutes(0, promptmgmtapi.AppendRouteDescriptors(nil, &stubPromptTemplateHandler{}))
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/v1/listing-kits/prompts/catalog"},
		{method: http.MethodPut, path: "/api/v1/listing-kits/prompts", body: `{}`},
		{method: http.MethodPatch, path: "/api/v1/listing-kits/prompts/prompt-key/status", body: `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			response := httptest.NewRecorder()

			server.Handler.ServeHTTP(response, request)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusUnauthorized, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), "zitadel_token_missing") {
				t.Fatalf("body = %s, want zitadel_token_missing", response.Body.String())
			}
		})
	}
}

func TestListingKitAuthContextRouteRequiresVerifiedIdentity(t *testing.T) {
	handler := &stubListingKitHandler{}
	server := buildHTTPServerFromRoutes(0, listingkithttpapi.AppendRouteDescriptors(nil, handler))

	unauthenticated := httptest.NewRecorder()
	server.Handler.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/auth-context", nil))
	require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
	require.Contains(t, unauthenticated.Body.String(), "zitadel_token_missing")

	authenticated := httptest.NewRecorder()
	server.Handler.ServeHTTP(authenticated, withAppHTTPTestBearer(httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/auth-context", nil)))
	require.Equal(t, http.StatusOK, authenticated.Code)
	require.True(t, handler.getAuthContextCalled)
}

func TestListingKitPromptRoutesAllowVerifiedIdentity(t *testing.T) {
	server := buildHTTPServerFromRoutes(0, promptmgmtapi.AppendRouteDescriptors(nil, &stubPromptTemplateHandler{}))
	request := withAppHTTPTestBearer(httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/prompts/catalog", nil))
	response := httptest.NewRecorder()

	server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
}

func TestListingKitPromptWritesRejectVerifiedViewer(t *testing.T) {
	server := buildHTTPServerFromRoutes(0, promptmgmtapi.AppendRouteDescriptors(nil, &stubPromptTemplateHandler{}))
	request := httptest.NewRequest(http.MethodPut, "/api/v1/listing-kits/prompts", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+appHTTPTestViewerBearerToken)
	response := httptest.NewRecorder()

	server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
}

type stubAmazonListingHandler struct {
	generateCalled  bool
	listQueueCalled bool
	getResultCalled bool
	workbenchCalled bool
	reviewCalled    bool
	submitCalled    bool
}

type stubTaskRPCHandler struct {
	healthCalled     bool
	statusCalled     bool
	queueStatsCalled bool
}

type stubSheinLoginHandler struct{}

func (s *stubSheinLoginHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *stubSheinLoginHandler) ListAccounts(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": []any{}})
}

func (s *stubSheinLoginHandler) Login(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *stubSheinLoginHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *stubSheinLoginHandler) ListWarehouses(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": []any{}})
}

func (s *stubSheinLoginHandler) SubmitVerifyCode(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *stubSheinLoginHandler) CancelVerifyCodeWait(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *stubSheinLoginHandler) ClearCookie(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *stubSheinLoginHandler) GetLastFailure(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *stubSheinLoginHandler) ClearLastFailure(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type stubSDSLoginHandler struct{}

func (s *stubSDSLoginHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *stubSDSLoginHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *stubSDSLoginHandler) Login(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *stubSDSLoginHandler) ManualLogin(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *stubSDSLoginHandler) GetAuthState(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *stubSDSLoginHandler) ClearState(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *stubTaskRPCHandler) GetHealth(c *gin.Context) {
	s.healthCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *stubAmazonListingHandler) GenerateListing(c *gin.Context) {
	s.generateCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": "listing-task"})
}

func (s *stubAmazonListingHandler) ListTaskQueue(c *gin.Context) {
	s.listQueueCalled = true
	c.JSON(http.StatusOK, gin.H{"count": 1})
}

func (s *stubAmazonListingHandler) GetTaskResult(c *gin.Context) {
	s.getResultCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id")})
}

func (s *stubAmazonListingHandler) GetTaskWorkbench(c *gin.Context) {
	s.workbenchCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "ready": true})
}

func (s *stubAmazonListingHandler) ReviewTask(c *gin.Context) {
	s.reviewCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "needs_review"})
}

func (s *stubAmazonListingHandler) SubmitTask(c *gin.Context) {
	s.submitCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "submitted"})
}

func (s *stubTaskRPCHandler) GetTaskStatus(c *gin.Context) {
	s.statusCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "canonicalStatus": "processing"})
}

func (s *stubTaskRPCHandler) GetQueueStats(c *gin.Context) {
	s.queueStatsCalled = true
	c.JSON(http.StatusOK, gin.H{"queueStats": "ok"})
}

type stubProductHandler struct {
	generateCalled  bool
	getResultCalled bool
}

func (s *stubProductHandler) GenerateProduct(c *gin.Context) {
	s.generateCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": "product-task"})
}

func (s *stubProductHandler) GetTaskResult(c *gin.Context) {
	s.getResultCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id")})
}

type stubImageHandler struct {
	processCalled   bool
	getResultCalled bool
	reviewCalled    bool
}

func (s *stubImageHandler) ProcessImages(c *gin.Context) {
	s.processCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": "image-task"})
}

func (s *stubImageHandler) GetTaskResult(c *gin.Context) {
	s.getResultCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id")})
}

func (s *stubImageHandler) ReviewTask(c *gin.Context) {
	s.reviewCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "reviewed"})
}

type stubListingKitHandler struct {
	generateCalled                         bool
	generateStudioDesignsCalled            bool
	generateStudioProductImagesCalled      bool
	startStudioAsyncJobCalled              bool
	getStudioAsyncJobCalled                bool
	uploadImagesCalled                     bool
	getUploadedImageCalled                 bool
	listTasksCalled                        bool
	getResultCalled                        bool
	getPreviewCalled                       bool
	getGenerationCalled                    bool
	getGenerationQueueCalled               bool
	getGenerationReviewSessionCalled       bool
	getGenerationReviewPreviewCalled       bool
	dispatchGenerationNavigationCalled     bool
	retryGenerationCalled                  bool
	retryChildTaskCalled                   bool
	executeGenerationActionCalled          bool
	getHistoryCalled                       bool
	getHistoryDetailCalled                 bool
	getExportCalled                        bool
	revisionCalled                         bool
	validateCalled                         bool
	searchSheinCategoriesCalled            bool
	submitCalled                           bool
	listAdminStoresCalled                  bool
	listAdminStoreStatisticsCalled         bool
	listDeletedAdminStoresCalled           bool
	restoreAdminStoreCalled                bool
	permanentlyDeleteAdminStoreCalled      bool
	extendAdminStoreValidityCalled         bool
	listAdminImportTasksCalled             bool
	batchCreateAdminImportTasksCalled      bool
	deleteAdminImportTaskCalled            bool
	listAdminFilterRulesCalled             bool
	createAdminFilterRuleCalled            bool
	deleteAdminFilterRuleCalled            bool
	listAdminProfitRulesCalled             bool
	createAdminProfitRuleCalled            bool
	deleteAdminProfitRuleCalled            bool
	listAdminPricingRulesCalled            bool
	createAdminPricingRuleCalled           bool
	deleteAdminPricingRuleCalled           bool
	listAdminOperationStrategiesCalled     bool
	createAdminOperationStrategyCalled     bool
	deleteAdminOperationStrategyCalled     bool
	listAdminSensitiveWordsCalled          bool
	createAdminSensitiveWordCalled         bool
	deleteAdminSensitiveWordCalled         bool
	listAdminGenerationTopicPoliciesCalled bool
	createAdminGenerationTopicPolicyCalled bool
	deleteAdminGenerationTopicPolicyCalled bool
	listAdminProductImportMappingsCalled   bool
	createAdminProductImportMappingCalled  bool
	deleteAdminProductImportMappingCalled  bool
	listAdminCategoriesCalled              bool
	createAdminCategoryCalled              bool
	deleteAdminCategoryCalled              bool
	listAdminProductDataCalled             bool
	createAdminProductDataCalled           bool
	deleteAdminProductDataCalled           bool
	getAuthContextCalled                   bool
}

func (s *stubListingKitHandler) GenerateListingKit(c *gin.Context) {
	s.generateCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": "listing-kit-task"})
}

func (s *stubListingKitHandler) DeliverZitadelSMS(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func (s *stubListingKitHandler) GetAuthContext(c *gin.Context) {
	s.getAuthContextCalled = true
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *stubListingKitHandler) AnalyzeStudioReferenceStyle(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"style": gin.H{}})
}

func (s *stubListingKitHandler) GenerateStudioDesigns(c *gin.Context) {
	s.generateStudioDesignsCalled = true
	c.JSON(http.StatusOK, gin.H{"images": []any{}})
}

func (s *stubListingKitHandler) GenerateStudioProductImages(c *gin.Context) {
	s.generateStudioProductImagesCalled = true
	c.JSON(http.StatusOK, gin.H{"images": []any{}})
}

func (s *stubListingKitHandler) GetSDSBaselineReadiness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (s *stubListingKitHandler) WarmSDSBaseline(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (s *stubListingKitHandler) StartStudioAsyncJob(c *gin.Context) {
	s.startStudioAsyncJobCalled = true
	c.JSON(http.StatusAccepted, gin.H{"job_id": "studio-job-1", "status": "running"})
}

func (s *stubListingKitHandler) GetStudioAsyncJob(c *gin.Context) {
	s.getStudioAsyncJobCalled = true
	c.JSON(http.StatusOK, gin.H{"job_id": c.Param("job_id"), "status": "succeeded"})
}

func (s *stubListingKitHandler) UploadListingKitImages(c *gin.Context) {
	s.uploadImagesCalled = true
	c.JSON(http.StatusOK, gin.H{"image_urls": []string{"/api/v1/listing-kits/uploads/files/test.jpg"}})
}

func (s *stubListingKitHandler) GetUploadedListingKitImage(c *gin.Context) {
	s.getUploadedImageCalled = true
	c.Data(http.StatusOK, "image/jpeg", []byte{0xFF, 0xD8, 0xFF})
}

func (s *stubListingKitHandler) DeleteUploadedListingKitImage(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"key": c.Param("key"), "size": 3})
}

func (s *stubListingKitHandler) ListTasks(c *gin.Context) {
	s.listTasksCalled = true
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) ListAdminStores(c *gin.Context) {
	s.listAdminStoresCalled = true
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) ListAdminStoreStatistics(c *gin.Context) {
	s.listAdminStoreStatisticsCalled = true
	c.JSON(http.StatusOK, []any{})
}

func (s *stubListingKitHandler) ListPlatformStoreStatistics(c *gin.Context) {
	c.JSON(http.StatusOK, []any{})
}

func (s *stubListingKitHandler) GetAdminDispatchEventSummary(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}

func (s *stubListingKitHandler) ListAdminDispatchEvents(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) ListDeletedAdminStores(c *gin.Context) {
	s.listDeletedAdminStoresCalled = true
	c.JSON(http.StatusOK, []any{})
}

func (s *stubListingKitHandler) GetAdminStore(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) CreateAdminStore(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{"id": 1})
}

func (s *stubListingKitHandler) UpdateAdminStore(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) UpdateAdminStoreStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) DeleteAdminStore(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *stubListingKitHandler) ListSimpleAdminStores(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}})
}

func (s *stubListingKitHandler) RestoreAdminStore(c *gin.Context) {
	s.restoreAdminStoreCalled = true
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) PermanentlyDeleteAdminStore(c *gin.Context) {
	s.permanentlyDeleteAdminStoreCalled = true
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *stubListingKitHandler) ExtendAdminStoreValidity(c *gin.Context) {
	s.extendAdminStoreValidityCalled = true
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) ListAdminImportTasks(c *gin.Context) {
	s.listAdminImportTasksCalled = true
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) BatchCreateAdminImportTasks(c *gin.Context) {
	s.batchCreateAdminImportTasksCalled = true
	c.JSON(http.StatusCreated, gin.H{"createdCount": 1, "items": []any{}})
}

func (s *stubListingKitHandler) DeleteAdminImportTask(c *gin.Context) {
	s.deleteAdminImportTaskCalled = true
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *stubListingKitHandler) ListAdminFilterRules(c *gin.Context) {
	s.listAdminFilterRulesCalled = true
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) GetAdminFilterRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) CreateAdminFilterRule(c *gin.Context) {
	s.createAdminFilterRuleCalled = true
	c.JSON(http.StatusCreated, gin.H{"id": 1})
}

func (s *stubListingKitHandler) UpdateAdminFilterRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) UpdateAdminFilterRuleStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) DeleteAdminFilterRule(c *gin.Context) {
	s.deleteAdminFilterRuleCalled = true
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *stubListingKitHandler) ListAdminProfitRules(c *gin.Context) {
	s.listAdminProfitRulesCalled = true
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) GetAdminProfitRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) CreateAdminProfitRule(c *gin.Context) {
	s.createAdminProfitRuleCalled = true
	c.JSON(http.StatusCreated, gin.H{"id": 1})
}

func (s *stubListingKitHandler) UpdateAdminProfitRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) UpdateAdminProfitRuleStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) DeleteAdminProfitRule(c *gin.Context) {
	s.deleteAdminProfitRuleCalled = true
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *stubListingKitHandler) ListAdminPricingRules(c *gin.Context) {
	s.listAdminPricingRulesCalled = true
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) GetAdminPricingRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) CreateAdminPricingRule(c *gin.Context) {
	s.createAdminPricingRuleCalled = true
	c.JSON(http.StatusCreated, gin.H{"id": 1})
}

func (s *stubListingKitHandler) UpdateAdminPricingRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) UpdateAdminPricingRuleStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) DeleteAdminPricingRule(c *gin.Context) {
	s.deleteAdminPricingRuleCalled = true
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *stubListingKitHandler) ListAdminOperationStrategies(c *gin.Context) {
	s.listAdminOperationStrategiesCalled = true
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) GetAdminOperationStrategy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) CreateAdminOperationStrategy(c *gin.Context) {
	s.createAdminOperationStrategyCalled = true
	c.JSON(http.StatusCreated, gin.H{"id": 1})
}

func (s *stubListingKitHandler) UpdateAdminOperationStrategy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) UpdateAdminOperationStrategyStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) DeleteAdminOperationStrategy(c *gin.Context) {
	s.deleteAdminOperationStrategyCalled = true
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *stubListingKitHandler) ListAdminScheduledTaskConfigs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) GetAdminScheduledTaskConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) UpsertAdminScheduledTaskConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": 1})
}

func (s *stubListingKitHandler) UpdateAdminScheduledTaskConfigStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) DeleteAdminScheduledTaskConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *stubListingKitHandler) ListAdminSensitiveWords(c *gin.Context) {
	s.listAdminSensitiveWordsCalled = true
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) GetAdminSensitiveWord(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) CreateAdminSensitiveWord(c *gin.Context) {
	s.createAdminSensitiveWordCalled = true
	c.JSON(http.StatusCreated, gin.H{"id": 1})
}

func (s *stubListingKitHandler) UpdateAdminSensitiveWord(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) UpdateAdminSensitiveWordStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) DeleteAdminSensitiveWord(c *gin.Context) {
	s.deleteAdminSensitiveWordCalled = true
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *stubListingKitHandler) ListAdminGenerationTopicCatalog(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}})
}

func (s *stubListingKitHandler) ListAdminGenerationTopicOverrides(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) GetAdminGenerationTopicOverride(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) CreateAdminGenerationTopicOverride(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{"id": 1})
}

func (s *stubListingKitHandler) UpdateAdminGenerationTopicOverride(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) UpdateAdminGenerationTopicOverrideStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) DeleteAdminGenerationTopicOverride(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *stubListingKitHandler) ListAdminGenerationTopicPolicies(c *gin.Context) {
	s.listAdminGenerationTopicPoliciesCalled = true
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) GetAdminGenerationTopicPolicy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) CreateAdminGenerationTopicPolicy(c *gin.Context) {
	s.createAdminGenerationTopicPolicyCalled = true
	c.JSON(http.StatusCreated, gin.H{"id": 1})
}

func (s *stubListingKitHandler) UpdateAdminGenerationTopicPolicy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) UpdateAdminGenerationTopicPolicyStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) DeleteAdminGenerationTopicPolicy(c *gin.Context) {
	s.deleteAdminGenerationTopicPolicyCalled = true
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *stubListingKitHandler) ListAdminProductImportMappings(c *gin.Context) {
	s.listAdminProductImportMappingsCalled = true
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) GetAdminProductImportMapping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) CreateAdminProductImportMapping(c *gin.Context) {
	s.createAdminProductImportMappingCalled = true
	c.JSON(http.StatusCreated, gin.H{"id": 1})
}

func (s *stubListingKitHandler) UpdateAdminProductImportMapping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) UpdateAdminProductImportMappingStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) DeleteAdminProductImportMapping(c *gin.Context) {
	s.deleteAdminProductImportMappingCalled = true
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *stubListingKitHandler) ListAdminCategories(c *gin.Context) {
	s.listAdminCategoriesCalled = true
	c.JSON(http.StatusOK, []any{})
}

func (s *stubListingKitHandler) GetAdminCategory(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) CreateAdminCategory(c *gin.Context) {
	s.createAdminCategoryCalled = true
	c.JSON(http.StatusCreated, gin.H{"id": 1})
}

func (s *stubListingKitHandler) UpdateAdminCategory(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) UpdateAdminCategoryStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) DeleteAdminCategory(c *gin.Context) {
	s.deleteAdminCategoryCalled = true
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *stubListingKitHandler) ListAdminProductData(c *gin.Context) {
	s.listAdminProductDataCalled = true
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) GetAdminProductData(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) CreateAdminProductData(c *gin.Context) {
	s.createAdminProductDataCalled = true
	c.JSON(http.StatusCreated, gin.H{"id": 1})
}

func (s *stubListingKitHandler) UpdateAdminProductData(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) UpdateAdminProductDataStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) DeleteAdminProductData(c *gin.Context) {
	s.deleteAdminProductDataCalled = true
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *stubListingKitHandler) GetCurrentSubscription(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"tenant_id": "test"})
}

func (s *stubListingKitHandler) ListSubscriptionModules(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}})
}

func (s *stubListingKitHandler) ListSubscriptionEntitlements(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"tenant_id": "test"})
}

func (s *stubListingKitHandler) UpsertSubscriptionEntitlement(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"module_code": c.Param("module_code")})
}

func (s *stubListingKitHandler) ListPlatformTenantSubscriptions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []gin.H{{"tenant_id": "org-286"}}})
}

func (s *stubListingKitHandler) ListPlatformTenantDirectory(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []gin.H{{"tenant_id": "org-286", "tenant_display_name": "Test tenant"}}})
}

func (s *stubListingKitHandler) ListPlatformSubscriptionPlans(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []gin.H{{"plan": gin.H{"code": "professional"}}}})
}

func (s *stubListingKitHandler) UpsertPlatformSubscriptionPlan(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"plan": gin.H{"code": c.Param("plan_code")}})
}

func (s *stubListingKitHandler) UpsertPlatformSubscriptionPlanModule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"plan": gin.H{"code": c.Param("plan_code")}, "modules": []gin.H{{"module_code": c.Param("module_code")}}})
}

func (s *stubListingKitHandler) DeletePlatformSubscriptionPlanModule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"plan": gin.H{"code": c.Param("plan_code")}})
}

func (s *stubListingKitHandler) SetPlatformSubscriptionPlanStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"plan": gin.H{"code": c.Param("plan_code"), "active": false}})
}

func (s *stubListingKitHandler) ListPlatformSubscriptionPlanTenants(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []gin.H{{"tenant_id": "org-286", "plan_code": c.Param("plan_code")}}})
}

func (s *stubListingKitHandler) ListPlatformSubscriptionPlanAuditLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []gin.H{{"action": "plan_apply", "reason": c.Param("plan_code")}}})
}

func (s *stubListingKitHandler) GetPlatformTenantSubscription(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"tenant_id": c.Param("tenant_id")})
}

func (s *stubListingKitHandler) ApplyPlatformTenantSubscriptionPlan(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"tenant_id": c.Param("tenant_id"), "plan_code": "professional"})
}

func (s *stubListingKitHandler) UpsertPlatformTenantSubscriptionEntitlement(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"tenant_id": c.Param("tenant_id"), "module_code": c.Param("module_code")})
}

func (s *stubListingKitHandler) SetPlatformTenantSubscriptionUsage(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"tenant_id": c.Param("tenant_id"), "module_code": c.Param("module_code"), "metric": c.Param("metric")})
}

func (s *stubListingKitHandler) ListPlatformTenantSubscriptionAuditLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []gin.H{}})
}

func (s *stubListingKitHandler) InviteTenantMember(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{"tenant_id": c.Param("tenant_id")})
}

func (s *stubListingKitHandler) GetTaskResult(c *gin.Context) {
	s.getResultCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id")})
}

func (s *stubListingKitHandler) RequeuePendingTasks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"requeued_count": 0})
}

func (s *stubListingKitHandler) RecoverTaskNow(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "pending"})
}

func (s *stubListingKitHandler) BulkRecoverTasks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"recovered_count": 0})
}

func (s *stubListingKitHandler) GetTaskPreview(c *gin.Context) {
	s.getPreviewCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "selected_platform": c.Query("platform")})
}

func (s *stubListingKitHandler) LookupSheinPODImages(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []gin.H{}, "total": 0})
}

func (s *stubListingKitHandler) GetTaskGenerationTasks(c *gin.Context) {
	s.getGenerationCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "tasks": []any{}})
}

func (s *stubListingKitHandler) GetTaskGenerationQueue(c *gin.Context) {
	s.getGenerationQueueCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "items": []any{}})
}

func (s *stubListingKitHandler) GetTaskGenerationReviewSession(c *gin.Context) {
	s.getGenerationReviewSessionCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "slot": c.Query("slot")})
}

func (s *stubListingKitHandler) GetTaskGenerationReviewPreview(c *gin.Context) {
	s.getGenerationReviewPreviewCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "slot": c.Query("slot")})
}

func (s *stubListingKitHandler) DispatchTaskGenerationNavigation(c *gin.Context) {
	s.dispatchGenerationNavigationCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "dispatch_kind": "session"})
}

func (s *stubListingKitHandler) RetryTaskGenerationTasks(c *gin.Context) {
	s.retryGenerationCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "retried"})
}

func (s *stubListingKitHandler) RetryTaskChildTask(c *gin.Context) {
	s.retryChildTaskCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "retried"})
}

func (s *stubListingKitHandler) GetTaskSDSRepair(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "repairable"})
}

func (s *stubListingKitHandler) RepairAndRetryTaskSDS(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "retried"})
}

func (s *stubListingKitHandler) ExecuteTaskGenerationAction(c *gin.Context) {
	s.executeGenerationActionCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "executed"})
}

func (s *stubListingKitHandler) GetTaskRevisionHistory(c *gin.Context) {
	s.getHistoryCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "limit": c.Query("limit")})
}

func (s *stubListingKitHandler) GetTaskRevisionHistoryDetail(c *gin.Context) {
	s.getHistoryDetailCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "revision_id": c.Param("revision_id")})
}

func (s *stubListingKitHandler) GetTaskExport(c *gin.Context) {
	s.getExportCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "selected_platform": c.Query("platform")})
}

func (s *stubListingKitHandler) ApplyTaskRevision(c *gin.Context) {
	s.revisionCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "revised"})
}

func (s *stubListingKitHandler) ValidateTaskRevision(c *gin.Context) {
	s.validateCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "valid": false})
}

func (s *stubListingKitHandler) SubmitTask(c *gin.Context) {
	s.submitCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "submitted"})
}

func (s *stubListingKitHandler) RefreshSubmissionStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "status": "refreshed"})
}

func (s *stubListingKitHandler) ListSettingsNamespaces(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []gin.H{{"namespace": "ai"}, {"namespace": "shein"}}})
}

func (s *stubListingKitHandler) GetSettingsNamespaceSchema(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"namespace": c.Param("namespace"), "label": "schema"})
}

func (s *stubListingKitHandler) GetSettingsNamespace(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"namespace": c.Param("namespace"), "method": "GET"})
}

func (s *stubListingKitHandler) GetSettingsHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *stubListingKitHandler) GetReadiness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (s *stubListingKitHandler) UpdateSettingsNamespace(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"namespace": c.Param("namespace"), "method": "PUT"})
}

func (s *stubListingKitHandler) GetSheinSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"default_store_id": 869})
}

func (s *stubListingKitHandler) ListSheinStoreProfiles(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []gin.H{{"id": 1, "store_id": 869}}})
}

func (s *stubListingKitHandler) UpsertSheinStoreProfile(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": 1, "store_id": 869})
}

func (s *stubListingKitHandler) DeleteSheinStoreProfile(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func (s *stubListingKitHandler) UpdateSheinSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"default_store_id": 869})
}

func (s *stubListingKitHandler) GetAIClientSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"client_name": "default", "api_key_set": false})
}

func (s *stubListingKitHandler) UpdateAIClientSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"client_name": "default", "api_key_set": true})
}

func (s *stubListingKitHandler) ListTenantStores(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}})
}

func (s *stubListingKitHandler) CreateTenantStore(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": 1})
}

func (s *stubListingKitHandler) UpdateTenantStore(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) DeleteTenantStore(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func (s *stubListingKitHandler) ListSimpleTenantStores(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}})
}

func (s *stubListingKitHandler) ListSheinEnrollmentDashboard(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) TriggerSheinStoreSync(c *gin.Context) {
	c.JSON(http.StatusAccepted, gin.H{"job": gin.H{"status": "queued"}})
}

func (s *stubListingKitHandler) GetSheinEnrollmentStoreSummary(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"summary": gin.H{"store_id": c.Param("store_id")}})
}

func (s *stubListingKitHandler) ListSheinSyncedProducts(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) UpdateSheinSyncedProductCost(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
}

func (s *stubListingKitHandler) ListSheinSourceSDSCostGroups(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) SyncSheinSourceSDSProduct(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"source_code": c.Param("source_code"), "synced_count": 0})
}

func (s *stubListingKitHandler) GetSheinActivityStrategy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"configured": false})
}

func (s *stubListingKitHandler) UpdateSheinActivityStrategy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"configured": true})
}

func (s *stubListingKitHandler) RefreshSheinActivityCandidates(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"result": gin.H{"eligible_count": 0}})
}

func (s *stubListingKitHandler) ListSheinActivityCandidates(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) ResetSheinActivityCandidates(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"result": gin.H{"reset_count": 0}})
}

func (s *stubListingKitHandler) ReviewSheinActivityCandidate(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"candidate": gin.H{"id": c.Param("id")}})
}

func (s *stubListingKitHandler) ExecuteSheinActivityEnrollment(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"run": gin.H{"status": "queued"}})
}

func (s *stubListingKitHandler) ListSheinActivityEnrollmentRuns(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

func (s *stubListingKitHandler) ListSheinActivityEnrollmentRunItems(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
}

type stubPromptTemplateHandler struct{}

func (s *stubPromptTemplateHandler) ListPromptTemplateCatalog(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []gin.H{{"key": "prompt-key"}}})
}

func (s *stubPromptTemplateHandler) GetPromptTemplateSchema(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"key": c.Param("key"), "category": "shein"})
}

func (s *stubPromptTemplateHandler) ListPromptTemplates(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}})
}

func (s *stubPromptTemplateHandler) UpsertPromptTemplate(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"key": "prompt-key"})
}

func (s *stubPromptTemplateHandler) SetPromptTemplateStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"key": c.Param("key")})
}

func (s *stubListingKitHandler) PreviewSheinPrice(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ready": true})
}

func (s *stubListingKitHandler) SearchSheinCategories(c *gin.Context) {
	s.searchSheinCategoriesCalled = true
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "query": c.Query("query"), "items": []any{}})
}

func (s *stubListingKitHandler) UpdateSheinFinalDraft(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id")})
}

func (s *stubListingKitHandler) GetSubmissionEvents(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"task_id": c.Param("task_id"), "items": []any{}})
}

func TestRegisterRoutes_AmazonListingEndpoints(t *testing.T) {
	t.Parallel()

	handler := &stubAmazonListingHandler{}
	router := mustBuildTestRouterFromModules(t, amazonlistinghttpapi.NewHTTPModule(handler))

	tests := []struct {
		name     string
		method   string
		path     string
		body     any
		assertFn func(*testing.T)
	}{
		{
			name:   "generate",
			method: http.MethodPost,
			path:   "/api/v1/amazon/listings/generate",
			body: map[string]any{
				"marketplace": "amazon",
			},
			assertFn: func(t *testing.T) {
				if !handler.generateCalled {
					t.Fatal("GenerateListing handler was not called")
				}
			},
		},
		{
			name:   "list queue",
			method: http.MethodGet,
			path:   "/api/v1/amazon/listings/tasks",
			assertFn: func(t *testing.T) {
				if !handler.listQueueCalled {
					t.Fatal("ListTaskQueue handler was not called")
				}
			},
		},
		{
			name:   "get result",
			method: http.MethodGet,
			path:   "/api/v1/amazon/listings/tasks/task-123",
			assertFn: func(t *testing.T) {
				if !handler.getResultCalled {
					t.Fatal("GetTaskResult handler was not called")
				}
			},
		},
		{
			name:   "get workbench",
			method: http.MethodGet,
			path:   "/api/v1/amazon/listings/tasks/task-123/workbench",
			assertFn: func(t *testing.T) {
				if !handler.workbenchCalled {
					t.Fatal("GetTaskWorkbench handler was not called")
				}
			},
		},
		{
			name:   "review",
			method: http.MethodPost,
			path:   "/api/v1/amazon/listings/tasks/task-123/review",
			body: map[string]any{
				"action": "approve",
			},
			assertFn: func(t *testing.T) {
				if !handler.reviewCalled {
					t.Fatal("ReviewTask handler was not called")
				}
			},
		},
		{
			name:   "submit",
			method: http.MethodPost,
			path:   "/api/v1/amazon/listings/tasks/task-123/submit",
			body: map[string]any{
				"action": "preview",
			},
			assertFn: func(t *testing.T) {
				if !handler.submitCalled {
					t.Fatal("SubmitTask handler was not called")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyReader *bytes.Reader
			if tt.body != nil {
				payload, err := json.Marshal(tt.body)
				if err != nil {
					t.Fatalf("marshal request: %v", err)
				}
				bodyReader = bytes.NewReader(payload)
			} else {
				bodyReader = bytes.NewReader(nil)
			}

			req := httptest.NewRequest(tt.method, tt.path, bodyReader)
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("%s %s = %d, want 200", tt.method, tt.path, resp.Code)
			}
			tt.assertFn(t)
		})
	}
}

func TestRegisterRoutes_ProductEndpoints(t *testing.T) {
	t.Parallel()

	handler := &stubProductHandler{}
	router := mustBuildTestRouterFromModules(t, productenrichhttpapi.NewHTTPModule(handler, nil))

	// generate endpoint
	generatePayload := map[string]any{"text": "test"}
	body, _ := json.Marshal(generatePayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/products/generate = %d, want 200", resp.Code)
	}
	if !handler.generateCalled {
		t.Fatal("GenerateProduct handler was not called")
	}

	// get task result
	handler.getResultCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/products/tasks/task-123", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/products/tasks/task-123 = %d, want 200", resp.Code)
	}
	if !handler.getResultCalled {
		t.Fatal("GetTaskResult handler was not called")
	}
}

func TestRegisterRoutes_ImageEndpoints(t *testing.T) {
	t.Parallel()

	handler := &stubImageHandler{}
	router := mustBuildTestRouterFromModules(t, productenrichhttpapi.NewHTTPModule(nil, handler))

	// process endpoint
	processPayload := map[string]any{"image_urls": []string{"https://example.com/1.jpg"}, "marketplace": "amazon"}
	body, _ := json.Marshal(processPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/process", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/images/process = %d, want 200", resp.Code)
	}
	if !handler.processCalled {
		t.Fatal("ProcessImages handler was not called")
	}

	// get task result
	handler.getResultCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/images/tasks/task-123", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/images/tasks/task-123 = %d, want 200", resp.Code)
	}
	if !handler.getResultCalled {
		t.Fatal("image GetTaskResult handler was not called")
	}

	// review endpoint
	handler.reviewCalled = false
	reviewPayload := map[string]any{"action": "approve"}
	body, _ = json.Marshal(reviewPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/images/tasks/task-123/review", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/images/tasks/task-123/review = %d, want 200", resp.Code)
	}
	if !handler.reviewCalled {
		t.Fatal("ReviewTask handler was not called")
	}
}

func TestRegisterRoutes_ListingKitEndpoints(t *testing.T) {
	t.Parallel()

	handler := &stubListingKitHandler{}
	promptHandler := &stubPromptTemplateHandler{}
	router := mustBuildTestRouterFromModules(
		t,
		listingkithttpapi.NewHTTPModule(handler),
		promptmgmtapi.NewHTTPModule(promptHandler),
	)

	body, _ := json.Marshal(map[string]any{
		"text":      "test",
		"platforms": []string{"amazon", "shein"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/generate = %d, want 200", resp.Code)
	}
	if !handler.generateCalled {
		t.Fatal("GenerateListingKit handler was not called")
	}

	handler.generateStudioDesignsCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/designs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/studio/designs = %d, want 200", resp.Code)
	}
	if !handler.generateStudioDesignsCalled {
		t.Fatal("listing kit GenerateStudioDesigns handler was not called")
	}

	handler.generateStudioProductImagesCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/product-images", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/studio/product-images = %d, want 200", resp.Code)
	}
	if !handler.generateStudioProductImagesCalled {
		t.Fatal("listing kit GenerateStudioProductImages handler was not called")
	}

	handler.startStudioAsyncJobCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/async-jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v1/listing-kits/studio/async-jobs = %d, want 202", resp.Code)
	}
	if !handler.startStudioAsyncJobCalled {
		t.Fatal("listing kit StartStudioAsyncJob handler was not called")
	}

	handler.getStudioAsyncJobCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/async-jobs/studio-job-1", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/studio/async-jobs/:job_id = %d, want 200", resp.Code)
	}
	if !handler.getStudioAsyncJobCalled {
		t.Fatal("listing kit GetStudioAsyncJob handler was not called")
	}

	handler.uploadImagesCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/uploads/images", bytes.NewReader([]byte("--x--")))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/uploads/images = %d, want 200", resp.Code)
	}
	if !handler.uploadImagesCalled {
		t.Fatal("listing kit UploadListingKitImages handler was not called")
	}

	handler.getUploadedImageCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/uploads/files/test.jpg", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/uploads/files/test.jpg = %d, want 200", resp.Code)
	}
	if !handler.getUploadedImageCalled {
		t.Fatal("listing kit GetUploadedListingKitImage handler was not called")
	}

	handler.listAdminStoresCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/stores", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/admin/stores = %d, want 200", resp.Code)
	}
	if !handler.listAdminStoresCalled {
		t.Fatal("listing kit ListAdminStores handler was not called")
	}

	handler.listAdminStoreStatisticsCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/store-statistics?date=2026-05-15", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/admin/store-statistics = %d, want 200", resp.Code)
	}
	if !handler.listAdminStoreStatisticsCalled {
		t.Fatal("listing kit ListAdminStoreStatistics handler was not called")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/settings/ai?scope=tenant&client_name=default", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/settings/ai = %d, want 200", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), `"namespace":"ai"`) {
		t.Fatalf("GET /api/v1/listing-kits/settings/ai body = %s, want namespace response", resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/listing-kits/prompts", bytes.NewReader([]byte(`{"key":"tmpl.key"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/listing-kits/prompts = %d, want 200", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), `"key":"prompt-key"`) {
		t.Fatalf("PUT /api/v1/listing-kits/prompts body = %s, want prompt response", resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/prompts/catalog", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/prompts/catalog = %d, want 200", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), `"key":"prompt-key"`) {
		t.Fatalf("GET /api/v1/listing-kits/prompts/catalog body = %s, want prompt catalog response", resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/prompts/schema/prompt-key", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/prompts/schema/:key = %d, want 200", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), `"category":"shein"`) {
		t.Fatalf("GET /api/v1/listing-kits/prompts/schema/:key body = %s, want schema response", resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/settings", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/settings = %d, want 200", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), `"namespace":"ai"`) {
		t.Fatalf("GET /api/v1/listing-kits/settings body = %s, want namespace list", resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/settings/ai/schema", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/settings/ai/schema = %d, want 200", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), `"namespace":"ai"`) {
		t.Fatalf("GET /api/v1/listing-kits/settings/ai/schema body = %s, want schema response", resp.Body.String())
	}

	handler.listDeletedAdminStoresCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/stores/deleted", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/admin/stores/deleted = %d, want 200", resp.Code)
	}
	if !handler.listDeletedAdminStoresCalled {
		t.Fatal("listing kit ListDeletedAdminStores handler was not called")
	}

	handler.restoreAdminStoreCalled = false
	req = httptest.NewRequest(http.MethodPut, "/api/v1/listing-kits/admin/stores/1/restore", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/listing-kits/admin/stores/:id/restore = %d, want 200", resp.Code)
	}
	if !handler.restoreAdminStoreCalled {
		t.Fatal("listing kit RestoreAdminStore handler was not called")
	}

	handler.permanentlyDeleteAdminStoreCalled = false
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/listing-kits/admin/stores/1/permanent", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/listing-kits/admin/stores/:id/permanent = %d, want 200", resp.Code)
	}
	if !handler.permanentlyDeleteAdminStoreCalled {
		t.Fatal("listing kit PermanentlyDeleteAdminStore handler was not called")
	}

	handler.extendAdminStoreValidityCalled = false
	req = httptest.NewRequest(http.MethodPut, "/api/v1/listing-kits/admin/stores/1/extend-validity?days=30", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/listing-kits/admin/stores/:id/extend-validity = %d, want 200", resp.Code)
	}
	if !handler.extendAdminStoreValidityCalled {
		t.Fatal("listing kit ExtendAdminStoreValidity handler was not called")
	}

	handler.listAdminImportTasksCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/import-tasks", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/admin/import-tasks = %d, want 200", resp.Code)
	}
	if !handler.listAdminImportTasksCalled {
		t.Fatal("listing kit ListAdminImportTasks handler was not called")
	}

	handler.batchCreateAdminImportTasksCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/admin/import-tasks/batch", bytes.NewReader([]byte(`{"productIds":["B001"]}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/listing-kits/admin/import-tasks/batch = %d, want 201", resp.Code)
	}
	if !handler.batchCreateAdminImportTasksCalled {
		t.Fatal("listing kit BatchCreateAdminImportTasks handler was not called")
	}

	handler.deleteAdminImportTaskCalled = false
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/listing-kits/admin/import-tasks/1", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/listing-kits/admin/import-tasks/:id = %d, want 200", resp.Code)
	}
	if !handler.deleteAdminImportTaskCalled {
		t.Fatal("listing kit DeleteAdminImportTask handler was not called")
	}

	handler.listAdminFilterRulesCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/filter-rules", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/admin/filter-rules = %d, want 200", resp.Code)
	}
	if !handler.listAdminFilterRulesCalled {
		t.Fatal("listing kit ListAdminFilterRules handler was not called")
	}

	handler.createAdminFilterRuleCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/admin/filter-rules", bytes.NewReader([]byte(`{"name":"Rule","ruleCode":"FR-1"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/listing-kits/admin/filter-rules = %d, want 201", resp.Code)
	}
	if !handler.createAdminFilterRuleCalled {
		t.Fatal("listing kit CreateAdminFilterRule handler was not called")
	}

	handler.deleteAdminFilterRuleCalled = false
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/listing-kits/admin/filter-rules/1", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/listing-kits/admin/filter-rules/:id = %d, want 200", resp.Code)
	}
	if !handler.deleteAdminFilterRuleCalled {
		t.Fatal("listing kit DeleteAdminFilterRule handler was not called")
	}

	handler.listAdminProfitRulesCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/profit-rules", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/admin/profit-rules = %d, want 200", resp.Code)
	}
	if !handler.listAdminProfitRulesCalled {
		t.Fatal("listing kit ListAdminProfitRules handler was not called")
	}

	handler.createAdminProfitRuleCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/admin/profit-rules", bytes.NewReader([]byte(`{"name":"Rule","ruleCode":"PR-1"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/listing-kits/admin/profit-rules = %d, want 201", resp.Code)
	}
	if !handler.createAdminProfitRuleCalled {
		t.Fatal("listing kit CreateAdminProfitRule handler was not called")
	}

	handler.deleteAdminProfitRuleCalled = false
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/listing-kits/admin/profit-rules/1", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/listing-kits/admin/profit-rules/:id = %d, want 200", resp.Code)
	}
	if !handler.deleteAdminProfitRuleCalled {
		t.Fatal("listing kit DeleteAdminProfitRule handler was not called")
	}

	handler.listAdminPricingRulesCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/pricing-rules", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/admin/pricing-rules = %d, want 200", resp.Code)
	}
	if !handler.listAdminPricingRulesCalled {
		t.Fatal("listing kit ListAdminPricingRules handler was not called")
	}

	handler.createAdminPricingRuleCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/admin/pricing-rules", bytes.NewReader([]byte(`{"name":"Rule","ruleCode":"AR-1"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/listing-kits/admin/pricing-rules = %d, want 201", resp.Code)
	}
	if !handler.createAdminPricingRuleCalled {
		t.Fatal("listing kit CreateAdminPricingRule handler was not called")
	}

	handler.deleteAdminPricingRuleCalled = false
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/listing-kits/admin/pricing-rules/1", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/listing-kits/admin/pricing-rules/:id = %d, want 200", resp.Code)
	}
	if !handler.deleteAdminPricingRuleCalled {
		t.Fatal("listing kit DeleteAdminPricingRule handler was not called")
	}

	handler.listAdminOperationStrategiesCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/operation-strategies", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/admin/operation-strategies = %d, want 200", resp.Code)
	}
	if !handler.listAdminOperationStrategiesCalled {
		t.Fatal("listing kit ListAdminOperationStrategies handler was not called")
	}

	handler.createAdminOperationStrategyCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/admin/operation-strategies", bytes.NewReader([]byte(`{"name":"Strategy","platform":"SHEIN","storeId":1}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/listing-kits/admin/operation-strategies = %d, want 201", resp.Code)
	}
	if !handler.createAdminOperationStrategyCalled {
		t.Fatal("listing kit CreateAdminOperationStrategy handler was not called")
	}

	handler.deleteAdminOperationStrategyCalled = false
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/listing-kits/admin/operation-strategies/1", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/listing-kits/admin/operation-strategies/:id = %d, want 200", resp.Code)
	}
	if !handler.deleteAdminOperationStrategyCalled {
		t.Fatal("listing kit DeleteAdminOperationStrategy handler was not called")
	}

	handler.listAdminSensitiveWordsCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/sensitive-words", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/admin/sensitive-words = %d, want 200", resp.Code)
	}
	if !handler.listAdminSensitiveWordsCalled {
		t.Fatal("listing kit ListAdminSensitiveWords handler was not called")
	}

	handler.createAdminSensitiveWordCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/admin/sensitive-words", bytes.NewReader([]byte(`{"word":"restricted","language":"en"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/listing-kits/admin/sensitive-words = %d, want 201", resp.Code)
	}
	if !handler.createAdminSensitiveWordCalled {
		t.Fatal("listing kit CreateAdminSensitiveWord handler was not called")
	}

	handler.deleteAdminSensitiveWordCalled = false
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/listing-kits/admin/sensitive-words/1", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/listing-kits/admin/sensitive-words/:id = %d, want 200", resp.Code)
	}
	if !handler.deleteAdminSensitiveWordCalled {
		t.Fatal("listing kit DeleteAdminSensitiveWord handler was not called")
	}

	handler.listAdminGenerationTopicPoliciesCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/generation-topic-policies", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/admin/generation-topic-policies = %d, want 200", resp.Code)
	}
	if !handler.listAdminGenerationTopicPoliciesCalled {
		t.Fatal("listing kit ListAdminGenerationTopicPolicies handler was not called")
	}

	handler.createAdminGenerationTopicPolicyCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/admin/generation-topic-policies", bytes.NewReader([]byte(`{"platform":"shein","topicKey":"children"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/listing-kits/admin/generation-topic-policies = %d, want 201", resp.Code)
	}
	if !handler.createAdminGenerationTopicPolicyCalled {
		t.Fatal("listing kit CreateAdminGenerationTopicPolicy handler was not called")
	}

	handler.deleteAdminGenerationTopicPolicyCalled = false
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/listing-kits/admin/generation-topic-policies/1", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/listing-kits/admin/generation-topic-policies/:id = %d, want 200", resp.Code)
	}
	if !handler.deleteAdminGenerationTopicPolicyCalled {
		t.Fatal("listing kit DeleteAdminGenerationTopicPolicy handler was not called")
	}

	handler.listAdminProductImportMappingsCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/product-import-mappings", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/admin/product-import-mappings = %d, want 200", resp.Code)
	}
	if !handler.listAdminProductImportMappingsCalled {
		t.Fatal("listing kit ListAdminProductImportMappings handler was not called")
	}

	handler.createAdminProductImportMappingCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/admin/product-import-mappings", bytes.NewReader([]byte(`{"importTaskId":1001,"storeId":11,"platform":"SHEIN","region":"US","productId":"B001"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/listing-kits/admin/product-import-mappings = %d, want 201", resp.Code)
	}
	if !handler.createAdminProductImportMappingCalled {
		t.Fatal("listing kit CreateAdminProductImportMapping handler was not called")
	}

	handler.deleteAdminProductImportMappingCalled = false
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/listing-kits/admin/product-import-mappings/1", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/listing-kits/admin/product-import-mappings/:id = %d, want 200", resp.Code)
	}
	if !handler.deleteAdminProductImportMappingCalled {
		t.Fatal("listing kit DeleteAdminProductImportMapping handler was not called")
	}

	handler.listAdminCategoriesCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/categories", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/admin/categories = %d, want 200", resp.Code)
	}
	if !handler.listAdminCategoriesCalled {
		t.Fatal("listing kit ListAdminCategories handler was not called")
	}

	handler.createAdminCategoryCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/admin/categories", bytes.NewReader([]byte(`{"name":"Apparel","code":"APPAREL","level":1}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/listing-kits/admin/categories = %d, want 201", resp.Code)
	}
	if !handler.createAdminCategoryCalled {
		t.Fatal("listing kit CreateAdminCategory handler was not called")
	}

	handler.deleteAdminCategoryCalled = false
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/listing-kits/admin/categories/1", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/listing-kits/admin/categories/:id = %d, want 200", resp.Code)
	}
	if !handler.deleteAdminCategoryCalled {
		t.Fatal("listing kit DeleteAdminCategory handler was not called")
	}

	handler.listAdminProductDataCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/product-data", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/admin/product-data = %d, want 200", resp.Code)
	}
	if !handler.listAdminProductDataCalled {
		t.Fatal("listing kit ListAdminProductData handler was not called")
	}

	handler.createAdminProductDataCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/admin/product-data", bytes.NewReader([]byte(`{"productId":"B001","platform":"SHEIN"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/listing-kits/admin/product-data = %d, want 201", resp.Code)
	}
	if !handler.createAdminProductDataCalled {
		t.Fatal("listing kit CreateAdminProductData handler was not called")
	}

	handler.deleteAdminProductDataCalled = false
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/listing-kits/admin/product-data/1", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/listing-kits/admin/product-data/:id = %d, want 200", resp.Code)
	}
	if !handler.deleteAdminProductDataCalled {
		t.Fatal("listing kit DeleteAdminProductData handler was not called")
	}

	handler.getResultCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123 = %d, want 200", resp.Code)
	}
	if !handler.getResultCalled {
		t.Fatal("listing kit GetTaskResult handler was not called")
	}

	handler.getPreviewCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/preview?platform=shein", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/preview = %d, want 200", resp.Code)
	}
	if !handler.getPreviewCalled {
		t.Fatal("listing kit GetTaskPreview handler was not called")
	}

	handler.getHistoryCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/revision-history?limit=5", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/revision-history = %d, want 200", resp.Code)
	}
	if !handler.getHistoryCalled {
		t.Fatal("listing kit GetTaskRevisionHistory handler was not called")
	}

	handler.getGenerationCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/generation-tasks", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/generation-tasks = %d, want 200", resp.Code)
	}
	if !handler.getGenerationCalled {
		t.Fatal("listing kit GetTaskGenerationTasks handler was not called")
	}

	handler.getGenerationQueueCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/generation-queue", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/generation-queue = %d, want 200", resp.Code)
	}
	if !handler.getGenerationQueueCalled {
		t.Fatal("listing kit GetTaskGenerationQueue handler was not called")
	}

	handler.getGenerationReviewSessionCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/generation-review-session?platform=shein&slot=main", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/generation-review-session = %d, want 200", resp.Code)
	}
	if !handler.getGenerationReviewSessionCalled {
		t.Fatal("listing kit GetTaskGenerationReviewSession handler was not called")
	}

	handler.getGenerationReviewPreviewCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/generation-review-preview?platform=shein&slot=main", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/generation-review-preview = %d, want 200", resp.Code)
	}
	if !handler.getGenerationReviewPreviewCalled {
		t.Fatal("listing kit GetTaskGenerationReviewPreview handler was not called")
	}

	handler.dispatchGenerationNavigationCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-123/generation-navigation/dispatch", bytes.NewReader([]byte(`{"target":{"dispatch_kind":"session","session_query":{"platform":"shein","slot":"main"}}}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/tasks/task-123/generation-navigation/dispatch = %d, want 200", resp.Code)
	}
	if !handler.dispatchGenerationNavigationCalled {
		t.Fatal("listing kit DispatchTaskGenerationNavigation handler was not called")
	}

	handler.retryGenerationCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-123/generation-tasks/retry", bytes.NewReader([]byte(`{"task_ids":["amazon:amazon-lifestyle"]}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/tasks/task-123/generation-tasks/retry = %d, want 200", resp.Code)
	}
	if !handler.retryGenerationCalled {
		t.Fatal("listing kit RetryTaskGenerationTasks handler was not called")
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-123/generation-actions/execute", bytes.NewReader([]byte(`{"action_key":"generate_missing_assets"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/tasks/task-123/generation-actions/execute = %d, want 200", resp.Code)
	}
	if !handler.executeGenerationActionCalled {
		t.Fatal("listing kit ExecuteTaskGenerationAction handler was not called")
	}

	handler.getHistoryDetailCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/revision-history/rev-123", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/revision-history/rev-123 = %d, want 200", resp.Code)
	}
	if !handler.getHistoryDetailCalled {
		t.Fatal("listing kit GetTaskRevisionHistoryDetail handler was not called")
	}

	handler.getExportCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/export?platform=shein", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/export = %d, want 200", resp.Code)
	}
	if !handler.getExportCalled {
		t.Fatal("listing kit GetTaskExport handler was not called")
	}

	handler.revisionCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-123/revision", bytes.NewReader([]byte(`{"platform":"shein","shein":{"spu_name":"updated"}}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/tasks/task-123/revision = %d, want 200", resp.Code)
	}
	if !handler.revisionCalled {
		t.Fatal("listing kit ApplyTaskRevision handler was not called")
	}

	handler.searchSheinCategoriesCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-123/shein/categories?query=mask", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/listing-kits/tasks/task-123/shein/categories = %d, want 200", resp.Code)
	}
	if !handler.searchSheinCategoriesCalled {
		t.Fatal("listing kit SearchSheinCategories handler was not called")
	}

	handler.validateCalled = false
	req = httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-123/revision/validate", bytes.NewReader([]byte(`{"platform":"shein","shein":{"category_id":0}}`)))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/listing-kits/tasks/task-123/revision/validate = %d, want 200", resp.Code)
	}
	if !handler.validateCalled {
		t.Fatal("listing kit ValidateTaskRevision handler was not called")
	}
}

func TestRegisterRoutes_NilHandlersDoNotExposeModuleRoutes(t *testing.T) {
	t.Parallel()

	router := mustBuildTestRouterFromModules(t)

	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/v1/products/generate"},
		{method: http.MethodPost, path: "/api/v1/images/process"},
		{method: http.MethodPost, path: "/api/v1/amazon/listings/generate"},
		{method: http.MethodPost, path: "/api/v1/listing-kits/generate"},
		{method: http.MethodGet, path: "/api/v1/management/tasks/health"},
		{method: http.MethodGet, path: "/api/v1/management/tasks/123/status"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("%s %s = %d, want 404", tt.method, tt.path, resp.Code)
		}
	}
}

func TestBuildRouteDescriptorsWithSheinLoginExposesOnlyAPI(t *testing.T) {
	t.Parallel()

	routes, err := buildRegisteredTestRoutes(nil, sheinlogin.NewHTTPModule(&stubSheinLoginHandler{}))
	require.NoError(t, err)

	index := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		index[route.Method+" "+route.Path] = struct{}{}
	}

	if _, ok := index[http.MethodGet+" /api/v1/shein-login/accounts"]; !ok {
		t.Fatal("expected SHEIN login accounts API route to remain registered")
	}
	if _, ok := index[http.MethodGet+" /shein-login"]; ok {
		t.Fatal("legacy embedded SHEIN login HTML route should not be registered")
	}
}

func TestBuildRouteDescriptorsWithSDSLoginExposesOnlyAPI(t *testing.T) {
	t.Parallel()

	routes, err := buildRegisteredTestRoutes(nil, sdslogin.NewHTTPModule(&stubSDSLoginHandler{}))
	require.NoError(t, err)

	index := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		index[route.Method+" "+route.Path] = struct{}{}
	}

	if _, ok := index[http.MethodGet+" /api/v1/sds-login/status"]; !ok {
		t.Fatal("expected SDS login status API route to remain registered")
	}
	if _, ok := index[http.MethodGet+" /sds-login"]; ok {
		t.Fatal("legacy embedded SDS login HTML route should not be registered")
	}
}

func TestRegisterRoutes_TaskRPCEndpoints(t *testing.T) {
	t.Parallel()

	handler := &stubTaskRPCHandler{}
	router := mustBuildTestRouterFromModules(t, taskrpcapi.NewHTTPModule(handler))

	tests := []struct {
		name     string
		method   string
		path     string
		assertFn func(*testing.T)
	}{
		{
			name:   "health",
			method: http.MethodGet,
			path:   "/api/v1/management/tasks/health",
			assertFn: func(t *testing.T) {
				if !handler.healthCalled {
					t.Fatal("GetHealth handler was not called")
				}
			},
		},
		{
			name:   "status",
			method: http.MethodGet,
			path:   "/api/v1/management/tasks/123/status",
			assertFn: func(t *testing.T) {
				if !handler.statusCalled {
					t.Fatal("GetTaskStatus handler was not called")
				}
			},
		},
		{
			name:   "queue-stats",
			method: http.MethodGet,
			path:   "/api/v1/management/tasks/queue-stats",
			assertFn: func(t *testing.T) {
				if !handler.queueStatsCalled {
					t.Fatal("GetQueueStats handler was not called")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("%s %s = %d, want 200", tt.method, tt.path, resp.Code)
			}
			tt.assertFn(t)
		})
	}
}

func TestBuildRouteDescriptorsMatchMountedRoutes(t *testing.T) {
	t.Parallel()

	router := gin.New()
	modules := testHTTPModules(
		productenrichhttpapi.NewHTTPModule(&stubProductHandler{}, &stubImageHandler{}),
		amazonlistinghttpapi.NewHTTPModule(&stubAmazonListingHandler{}),
		listingkithttpapi.NewHTTPModule(&stubListingKitHandler{}),
		taskrpcapi.NewHTTPModule(&stubTaskRPCHandler{}),
	)

	routes, err := buildRegisteredRoutesForModules(nil, modules)
	require.NoError(t, err)
	mountRoutes(router, routes)

	registered := router.Routes()
	if len(registered) != len(routes) {
		t.Fatalf("registered routes = %d, want %d", len(registered), len(routes))
	}

	index := make(map[string]struct{}, len(registered))
	for _, route := range registered {
		index[route.Method+" "+route.Path] = struct{}{}
	}

	for _, route := range routes {
		key := route.Method + " " + route.Path
		if _, ok := index[key]; !ok {
			t.Fatalf("expected mounted route %s", key)
		}
	}
}

func TestBuildHTTPServerBundleFromModulesMountsRegisteredRoutes(t *testing.T) {
	t.Parallel()

	modules := testHTTPModules(
		productenrichhttpapi.NewHTTPModule(&stubProductHandler{}, &stubImageHandler{}),
		amazonlistinghttpapi.NewHTTPModule(&stubAmazonListingHandler{}),
		listingkithttpapi.NewHTTPModule(&stubListingKitHandler{}),
		promptmgmtapi.NewHTTPModule(&stubPromptTemplateHandler{}),
		listingkithttpapi.NewStudioHTTPModule(&stubStudioSessionHandler{}),
		sheinlogin.NewHTTPModule(&stubSheinLoginHandler{}),
		sdslogin.NewHTTPModule(&stubSDSLoginHandler{}),
		taskrpcapi.NewHTTPModule(&stubTaskRPCHandler{}),
		sdshttpapi.NewHTTPModule(&stubSDSCatalogRouteHandler{}),
	)

	server, routes, err := buildHTTPServerBundleFromModules(18080, nil, modules)
	if err != nil {
		t.Fatalf("buildHTTPServerBundleFromModules returned error: %v", err)
	}

	expectedRoutes, err := buildRegisteredRoutesForModules(nil, modules)
	if err != nil {
		t.Fatalf("buildRegisteredRoutesForModules returned error: %v", err)
	}
	if got, want := routePaths(routes), routePaths(expectedRoutes); !equalStringSlices(got, want) {
		t.Fatalf("mounted routes mismatch\n got: %v\nwant: %v", got, want)
	}

	router, ok := server.Handler.(*gin.Engine)
	if !ok {
		t.Fatalf("server handler type = %T, want *gin.Engine", server.Handler)
	}

	req := withAppHTTPTestBearer(httptest.NewRequest(http.MethodGet, "/api/v1/sds/categories", nil))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/sds/categories = %d, want 200", resp.Code)
	}
}

func TestBuildHTTPServerBundleFromModulesReturnsRouteBuildErrors(t *testing.T) {
	t.Parallel()

	server, routes, err := buildHTTPServerBundleFromModules(18080, nil, []kernelmodule.Module{
		httpModule{
			name: "broken",
			register: func(*kernelmodule.Registry) error {
				return errors.New("boom")
			},
		},
	})

	if server != nil {
		t.Fatalf("server = %#v, want nil", server)
	}
	if routes != nil {
		t.Fatalf("routes = %#v, want nil", routes)
	}
	if err == nil || err.Error() != "register module broken: boom" {
		t.Fatalf("err = %v, want register module broken: boom", err)
	}
}

func TestBuildBootstrapBuildsServerFromRegisteredModules(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")

	bootstrap, err := buildBootstrap(logger, Options{
		ConfigPath: "../../../config/config-test.yaml",
		Port:       18080,
	})
	if err != nil {
		t.Fatalf("buildBootstrap returned error: %v", err)
	}
	if bootstrap.server == nil {
		t.Fatal("expected bootstrap server")
	}
	if len(bootstrap.routes) == 0 {
		t.Fatal("expected bootstrap routes")
	}
	if !containsRoute(bootstrap.routes, http.MethodGet, "/health") {
		t.Fatal("expected bootstrap routes to include GET /health")
	}
	if !containsRoute(bootstrap.routes, http.MethodGet, "/api/v1/sds/categories") {
		t.Fatal("expected bootstrap routes to include GET /api/v1/sds/categories")
	}
	if containsRoute(bootstrap.routes, http.MethodGet, "/shein-login") {
		t.Fatal("did not expect legacy shein-login HTML route")
	}

	router, ok := bootstrap.server.Handler.(*gin.Engine)
	if !ok {
		t.Fatalf("server handler type = %T, want *gin.Engine", bootstrap.server.Handler)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /health = %d, want 200", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/sds/categories", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code == http.StatusNotFound {
		t.Fatal("expected SDS catalog categories endpoint to be mounted")
	}
}

func TestSingleSDSCatalogHandlerPanicsOnMultipleHandlers(t *testing.T) {
	t.Parallel()

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic for multiple SDS catalog handlers")
		}
		message, ok := recovered.(string)
		if !ok {
			t.Fatalf("panic type = %T, want string", recovered)
		}
		if !strings.Contains(message, "expected at most 1 SDS catalog handler") {
			t.Fatalf("panic message = %q, want multiple SDS catalog handler error", message)
		}
	}()

	singleSDSCatalogHandler(&stubSDSCatalogRouteHandler{}, &stubSDSCatalogRouteHandler{})
}

func mustBuildTestRouterFromModules(t *testing.T, modules ...kernelmodule.Module) *gin.Engine {
	t.Helper()

	routes, err := buildRegisteredRoutesForModules(nil, testHTTPModules(modules...))
	require.NoError(t, err)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		withAppHTTPTestBearer(c.Request)
		c.Next()
	})
	mountRoutes(router, routes)
	return router
}

func singleSDSCatalogHandler(sdsCatalogHandlers ...sdshttpapi.HTTPRouteHandler) sdshttpapi.HTTPRouteHandler {
	if len(sdsCatalogHandlers) == 0 {
		return nil
	}
	if len(sdsCatalogHandlers) == 1 {
		return sdsCatalogHandlers[0]
	}
	panic(fmt.Sprintf("expected at most 1 SDS catalog handler, got %d", len(sdsCatalogHandlers)))
}

func buildRegisteredTestRoutes(cfg *config.Config, modules ...kernelmodule.Module) ([]httproute.Descriptor, error) {
	return buildRegisteredRoutesForModules(cfg, testHTTPModules(modules...))
}

func testHTTPModules(modules ...kernelmodule.Module) []kernelmodule.Module {
	return append([]kernelmodule.Module{newCoreHTTPModule()}, modules...)
}

func routePaths(routes []httproute.Descriptor) []string {
	out := make([]string, 0, len(routes))
	for _, route := range routes {
		out = append(out, route.Method+" "+route.Path)
	}
	return out
}

func equalStringSlices(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func containsRoute(routes []httproute.Descriptor, method string, path string) bool {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}
