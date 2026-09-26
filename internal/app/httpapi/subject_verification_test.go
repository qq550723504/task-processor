package httpapi

import (
	"context"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	verificationhttp "task-processor/internal/subjectverification/httpapi"
	"task-processor/internal/workbenchcontext"
	"testing"
	"time"
)

type verificationResolver struct {
	role    string
	revoked bool
	calls   int
}

func (r *verificationResolver) Resolve(_ context.Context, policy httproute.OrganizationAccessPolicy, in workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error) {
	r.calls++
	if r.revoked || in.RequestedOrganizationID != "org" {
		return authidentity.AuthenticatedIdentity{}, workbenchcontext.ErrOrganizationAccessDenied
	}
	if policy != httproute.OrganizationAccessPolicyLiveWrite {
		panic("verification reused cached grants")
	}
	return authidentity.AuthenticatedIdentity{UserID: in.Identity.UserID, EffectiveOrganizationID: "org", TenantID: "org", Roles: []string{r.role}}, nil
}
func TestSubjectVerificationAlwaysRechecksOrganizationAdmin(t *testing.T) {
	for _, tc := range []struct {
		role, org string
		revoked   bool
		want      int
	}{{"listingkit_admin", "org", false, 204}, {"listingkit_operator", "org", false, 403}, {"listingkit_admin", "org", true, 403}, {"listingkit_admin", "other", false, 403}} {
		for _, method := range []string{"GET", "POST"} {
			resolver := &verificationResolver{role: tc.role, revoked: tc.revoked}
			authorizer, _ := authz.NewListingKitAuthorizer(nil, nil)
			router := gin.New()
			routes := (verificationhttp.Handler{}).Routes(accountOrganizationTarget)
			for i := range routes {
				if routes[i].Path != verificationhttp.CallbackPath {
					routes[i].Handler = func(c *gin.Context) { c.Status(204) }
				}
			}
			mountRoutesWithAuthDependencies(router, routes, routeAuthDependencies{workbenchVerifier: mountedVerifierStub{identity: authidentity.AuthenticatedIdentity{UserID: "user", HomeOrganizationID: "home", TokenExpiresAt: time.Now().Add(time.Hour)}}, organizationResolver: resolver, authorizer: authorizer})
			path := verificationhttp.BasePath
			if method == "POST" {
				path += "/applications"
			}
			request := httptest.NewRequest(method, path, nil)
			request.Header.Set("Authorization", "Bearer token")
			request.Header.Set("X-Requested-Organization-ID", tc.org)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, request)
			if w.Code != tc.want || resolver.calls != 1 {
				t.Fatalf("%s role=%s org=%s revoked=%v: %d %s", method, tc.role, tc.org, tc.revoked, w.Code, w.Body.String())
			}
		}
	}
}
