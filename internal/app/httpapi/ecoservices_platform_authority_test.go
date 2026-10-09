package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	b "task-processor/internal/commercial/billing"
	e "task-processor/internal/ecoservices"
	ecohttp "task-processor/internal/ecoservices/httpapi"
	"task-processor/internal/httproute"
)

// Real route descriptors and authentication/authorization middleware run before
// these owner ports. Denied identities must never reach any of them.
type platformOwnerProbe struct {
	t     *testing.T
	calls int
}

func (p *platformOwnerProbe) check(s e.Scope) {
	p.calls++
	require.True(p.t, s.Platform)
	require.Empty(p.t, s.OrganizationID)
	require.Equal(p.t, "verified-user", s.ActorID)
}
func (p *platformOwnerProbe) Read(_ context.Context, q e.Query) (e.Page, error) {
	p.check(q.Scope)
	return e.Page{Requests: []e.Request{{OrderID: "11111111-1111-4111-8111-111111111111"}}}, nil
}
func (p *platformOwnerProbe) Mutate(_ context.Context, c e.Command) (e.Result, error) {
	p.check(c.Scope)
	return e.Result{}, e.ErrUnavailable
}
func (p *platformOwnerProbe) Upload(_ context.Context, s e.Scope, _, _, _, _ string, _ []byte) (e.File, error) {
	p.check(s)
	return e.File{}, e.ErrUnavailable
}
func (p *platformOwnerProbe) DownloadForKind(_ context.Context, s e.Scope, _, _ string) (e.File, []byte, error) {
	p.check(s)
	return e.File{}, nil, e.ErrUnavailable
}
func (p *platformOwnerProbe) Checkout(context.Context, string, string, string) (string, error) {
	p.calls++
	return "", e.ErrUnavailable
}
func (p *platformOwnerProbe) ReadFinancialFacts(context.Context, string) (b.ServiceFinancialView, error) {
	p.calls++
	return b.ServiceFinancialView{}, e.ErrUnavailable
}
func (p *platformOwnerProbe) RefreshChannelFees(context.Context, string, string) (b.ServiceFinancialView, error) {
	p.calls++
	return b.ServiceFinancialView{}, e.ErrUnavailable
}

func TestEcoservicesPlatformRoutesRequireGlobalAuthority(t *testing.T) {
	for _, identity := range []struct {
		name                                    string
		roles, configuredUsers, configuredRoles []string
		allowed                                 bool
	}{
		{name: "tenant admin", roles: []string{"admin"}},
		{name: "tenant listingkit admin", roles: []string{"listingkit_admin"}},
		{name: "forged role header", roles: []string{"listingkit_viewer"}},
		{name: "verified global role", roles: []string{"platform_admin"}, allowed: true},
		{name: "configured global user", roles: []string{"admin"}, configuredUsers: []string{"verified-user"}, allowed: true},
		{name: "configured global role", roles: []string{"support-platform"}, configuredRoles: []string{"support-platform"}, allowed: true},
	} {
		t.Run(identity.name, func(t *testing.T) {
			authorizer, err := authz.NewListingKitAuthorizer(identity.configuredUsers, identity.configuredRoles)
			require.NoError(t, err)
			probe := &platformOwnerProbe{t: t}
			handler, err := ecohttp.NewHandler(probe, probe, probe)
			require.NoError(t, err)
			handler.SetFinancial(probe)
			count := 0
			for _, route := range ecohttp.Routes(handler) {
				if !strings.HasPrefix(route.Path, ecohttp.AdminBase+"/") {
					continue
				}
				count++
				t.Run(route.Method+" "+route.Path, func(t *testing.T) {
					probe.t = t
					probe.calls = 0
					router := gin.New()
					mountRoutesWithAuthDependencies(router, []httproute.Descriptor{route}, routeAuthDependencies{
						workbenchVerifier: mountedVerifierStub{identity: authidentity.AuthenticatedIdentity{
							UserID: "verified-user", TenantID: "home-org", EffectiveOrganizationID: "home-org",
							Roles: identity.roles, TokenExpiresAt: time.Now().Add(time.Minute),
							OrganizationGrants: []authidentity.OrganizationGrant{{OrganizationID: "other-org", Roles: []string{"admin"}}},
						}}, authorizer: authorizer,
					})
					body := `{"reason":"platform reason"}`
					if strings.Contains(route.Path, "refund-") {
						body = `{"refundVersion":"1","reason":"platform reason"}`
					}
					if strings.HasSuffix(route.Path, "/fees") {
						body = `{"date":"2026-10-09"}`
					}
					if route.Method == http.MethodGet {
						body = ""
					}
					request := httptest.NewRequest(route.Method, strings.ReplaceAll(route.Path, ":id", "11111111-1111-4111-8111-111111111111"), strings.NewReader(body))
					request.Header.Set("Authorization", "Bearer verified-token")
					request.Header.Set("Content-Type", "application/json")
					request.Header.Set("Idempotency-Key", "22222222-2222-4222-8222-222222222222")
					request.Header.Set("If-Match", `"1"`)
					request.Header.Set("X-User-Roles", "platform_admin")
					request.Header.Set("X-Tenant-ID", "other-org")
					response := httptest.NewRecorder()
					router.ServeHTTP(response, request)
					if identity.allowed {
						require.NotEqual(t, http.StatusForbidden, response.Code, response.Body.String())
						require.Positive(t, probe.calls, response.Body.String())
					} else {
						require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
						require.Zero(t, probe.calls)
					}
				})
			}
			require.Equal(t, 11, count)
		})
	}
}

func TestTenantAdminCannotReachReferralPlatformReviewRoutes(t *testing.T) {
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	count := 0
	for _, route := range (referralHTTPModule{}).routes() {
		if route.Permission != authz.PermissionListingKitPlatformAdm {
			continue
		}
		count++
		t.Run(route.Path, func(t *testing.T) {
			called := false
			route.Handler = func(c *gin.Context) { called = true; c.Status(http.StatusNoContent) }
			router := gin.New()
			mountRoutesWithAuthDependencies(router, []httproute.Descriptor{route}, routeAuthDependencies{
				workbenchVerifier: mountedVerifierStub{identity: authidentity.AuthenticatedIdentity{UserID: "verified-user", Roles: []string{"admin"}, TokenExpiresAt: time.Now().Add(time.Minute)}},
				authorizer:        authorizer,
			})
			request := httptest.NewRequest(route.Method, strings.ReplaceAll(route.Path, ":withdrawal_id", "11111111-1111-4111-8111-111111111111"), nil)
			request.Header.Set("Authorization", "Bearer verified-token")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, http.StatusForbidden, response.Code)
			require.False(t, called)
		})
	}
	require.Equal(t, 2, count)
}

func TestTopUpRefundRejectsOrganizationNeutralTenantAdmin(t *testing.T) {
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	a := topUpRuntimeAuthorizer{directory: financialRecoveryAuthorizer{authorizer: authorizer}}
	identity := authidentity.AuthenticatedIdentity{UserID: "verified-user", Roles: []string{"admin"}, TokenExpiresAt: time.Now().Add(time.Minute)}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), identity)
	require.ErrorIs(t, a.AuthorizeTopUp(ctx, "", identity.UserID, true), b.ErrAuthorizationRevoked)
}
