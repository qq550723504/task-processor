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
	d "task-processor/internal/agentcustomization"
	customhttp "task-processor/internal/agentcustomization/httpapi"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	"task-processor/internal/workbenchcontext"
)

type customizationOwnerProbe struct {
	calls  int
	scopes []d.Scope
}

func (p *customizationOwnerProbe) mark(s d.Scope) { p.calls++; p.scopes = append(p.scopes, s) }
func (p *customizationOwnerProbe) Execute(_ context.Context, c d.Command) (d.Receipt, error) {
	p.mark(c.Scope)
	return d.Receipt{}, d.ErrUnavailable
}
func (p *customizationOwnerProbe) List(_ context.Context, s d.Scope, _ string) (d.Page, error) {
	p.mark(s)
	return d.Page{Items: []d.Request{}}, nil
}
func (p *customizationOwnerProbe) Read(_ context.Context, s d.Scope, _ string, _ int64) (d.Detail, error) {
	p.mark(s)
	return d.Detail{}, d.ErrNotFound
}
func (p *customizationOwnerProbe) Download(_ context.Context, s d.Scope, _, _ string) (d.Attachment, []byte, error) {
	p.mark(s)
	return d.Attachment{}, nil, d.ErrNotFound
}

func customizationMountedRequest(route httproute.Descriptor) *http.Request {
	path := strings.ReplaceAll(strings.ReplaceAll(route.Path, ":id", "11111111-1111-4111-8111-111111111111"), ":file", "22222222-2222-4222-8222-222222222222")
	body := ""
	if route.Method == http.MethodPost {
		if strings.HasSuffix(path, "/progress") {
			body = `{"stage":"EVALUATING","note":"平台评估"}`
		} else {
			body = `{"name":"需求","scenario":"场景","direction":"OTHER","description":"说明","contactName":"测试","contactMethod":"test","consent":true}`
		}
	}
	request := httptest.NewRequest(route.Method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer verified-token")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "33333333-3333-4333-8333-333333333333")
	if strings.HasSuffix(path, "/progress") {
		request.Header.Set("If-Match", `"1"`)
	}
	request.Header.Set("X-User-ID", "forged-user")
	request.Header.Set("X-User-Roles", "platform_admin")
	request.Header.Set("X-Tenant-ID", "forged-org")
	request.Header.Set("X-Requested-Organization-ID", "requested-org")
	return request
}

func TestAgentCustomizationPlatformUsesOnlyVerifiedGlobalAuthority(t *testing.T) {
	for _, candidate := range []struct {
		name         string
		roles, users []string
		allowed      bool
	}{
		{name: "enterprise administrator", roles: []string{"admin"}},
		{name: "enterprise listingkit administrator", roles: []string{"listingkit_admin"}},
		{name: "forged platform header", roles: []string{"listingkit_operator"}},
		{name: "verified platform role", roles: []string{"platform_admin"}, allowed: true},
		{name: "configured platform specialist", roles: []string{"listingkit_viewer"}, users: []string{"verified-user"}, allowed: true},
	} {
		t.Run(candidate.name, func(t *testing.T) {
			authorizer, err := authz.NewListingKitAuthorizer(candidate.users, nil)
			require.NoError(t, err)
			probe := &customizationOwnerProbe{}
			handler, err := customhttp.NewHandler(probe)
			require.NoError(t, err)
			for _, route := range customhttp.Routes(handler) {
				if !strings.HasPrefix(route.Path, customhttp.AdminBase) {
					continue
				}
				t.Run(route.Method+route.Path, func(t *testing.T) {
					probe.calls = 0
					probe.scopes = nil
					router := gin.New()
					mountRoutesWithAuthDependencies(router, []httproute.Descriptor{route}, routeAuthDependencies{authorizer: authorizer, workbenchVerifier: mountedVerifierStub{identity: authidentity.AuthenticatedIdentity{UserID: "verified-user", Roles: candidate.roles, EffectiveOrganizationID: "token-org", TokenExpiresAt: time.Now().Add(time.Minute), OrganizationGrants: []authidentity.OrganizationGrant{{OrganizationID: "requested-org", Roles: []string{"admin"}}}}}})
					response := httptest.NewRecorder()
					router.ServeHTTP(response, customizationMountedRequest(route))
					if !candidate.allowed {
						require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
						require.Zero(t, probe.calls)
					} else {
						require.Positive(t, probe.calls, response.Body.String())
						for _, scope := range probe.scopes {
							require.True(t, scope.Platform)
							require.Empty(t, scope.OrganizationID)
							require.Equal(t, "verified-user", scope.ActorID)
						}
					}
				})
			}
		})
	}
}

func TestAgentCustomizationRevocationRejectsReadsDownloadsAndReplays(t *testing.T) {
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	probe := &customizationOwnerProbe{}
	handler, err := customhttp.NewHandler(probe)
	require.NoError(t, err)
	for _, route := range customhttp.Routes(handler) {
		if strings.HasPrefix(route.Path, customhttp.AdminBase) {
			continue
		}
		t.Run(route.Method+route.Path, func(t *testing.T) {
			probe.calls = 0
			router := gin.New()
			mountRoutesWithAuthDependencies(router, []httproute.Descriptor{route}, routeAuthDependencies{authorizer: authorizer, workbenchVerifier: mountedVerifierStub{identity: authidentity.AuthenticatedIdentity{UserID: "verified-user", Roles: []string{"platform_admin"}, TokenExpiresAt: time.Now().Add(time.Minute)}}, organizationResolver: mountedOrganizationResolverError{err: workbenchcontext.ErrOrganizationAccessRevoked}, auditRecorder: &mountedWorkbenchAuditStub{}})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, customizationMountedRequest(route))
			require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
			require.Contains(t, response.Body.String(), "ORGANIZATION_ACCESS_REVOKED")
			require.Zero(t, probe.calls)
		})
	}
}
