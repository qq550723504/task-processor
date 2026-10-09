package httpapi

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	tm "task-processor/internal/toolmarket"
	"testing"
)

type repo struct{ called bool }

func (*repo) Activations(context.Context, tm.Scope) ([]tm.Activation, error) {
	return []tm.Activation{}, nil
}
func (*repo) Requests(context.Context, tm.Scope, bool, string, int) (tm.RequestPage, error) {
	return tm.RequestPage{Items: []tm.RequestSummary{}}, nil
}
func (*repo) Detail(context.Context, tm.Scope, bool, string) (tm.Detail, error) {
	return tm.Detail{}, tm.ErrNotFound
}
func (r *repo) Execute(ctx context.Context, c tm.Command, g func(context.Context) error, beforeApply ...func(context.Context) error) (tm.Receipt, error) {
	if e := g(ctx); e != nil {
		return tm.Receipt{}, e
	}
	for _, admit := range beforeApply {
		if e := admit(ctx); e != nil {
			return tm.Receipt{}, e
		}
	}
	r.called = true
	return tm.Receipt{ID: c.ID, Revision: "1"}, nil
}
func TestRoutesSeparateEnterpriseAndPlatform(t *testing.T) {
	for _, r := range Routes(nil) {
		if strings.HasPrefix(r.Path, AdminBase) {
			require.Equal(t, httproute.AuthPolicyCurrentIdentityWithVerifiedRoles, r.AuthPolicy)
			require.Equal(t, httproute.OrganizationAccessPolicyNone, r.OrganizationAccessPolicy)
			require.Equal(t, authz.PermissionListingKitPlatformAdm, r.Permission)
		} else {
			require.Equal(t, httproute.AuthPolicyCurrentIdentity, r.AuthPolicy)
			if r.Method != "GET" {
				require.Equal(t, httproute.OrganizationAccessPolicyLiveWrite, r.OrganizationAccessPolicy)
			}
		}
	}
}

func TestBuildRoutesRequiresAllInjectedDependencies(t *testing.T) {
	bound := func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }
	authorize := func(context.Context, string, bool) (tm.Scope, error) {
		return tm.Scope{ActorID: "admin", OrganizationID: "org-a"}, nil
	}
	for _, h := range []*Handler{
		nil,
		{Bind: bound, Authorize: authorize},
		{Repository: &repo{}, Authorize: authorize},
		{Repository: &repo{}, Bind: bound},
	} {
		routes, err := BuildRoutes(h)
		require.ErrorIs(t, err, tm.ErrUnavailable)
		require.Nil(t, routes)
	}
	routes, err := BuildRoutes(&Handler{Repository: &repo{}, Bind: bound, Authorize: authorize})
	require.NoError(t, err)
	require.Len(t, routes, 10)
	for _, route := range routes {
		require.NoError(t, ValidateDescriptor(route))
	}
}
func TestStrictBodyAndFreshAuthorizationPreventWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &repo{}
	count := 0
	h := &Handler{Repository: db, Readiness: tm.Readiness{LocalCapture: true}, Authorize: func(ctx context.Context, p string, platform bool) (tm.Scope, error) {
		count++
		if count > 1 {
			return tm.Scope{}, tm.ErrForbidden
		}
		return tm.Scope{ActorID: "admin", OrganizationID: "org-a"}, nil
	}}
	router := gin.New()
	for _, r := range Routes(h) {
		router.Handle(r.Method, r.Path, r.Handler)
	}
	send := func(body string) *httptest.ResponseRecorder {
		count = 0
		req := httptest.NewRequest("PUT", Base+"/activations/"+tm.AcquisitionID, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "1510eced-9831-49de-a28c-098cb16deba1")
		req.Header.Set("If-None-Match", "*")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	require.Equal(t, 400, send(`{"enabled":true,"organizationId":"org-b"}`).Code)
	require.Equal(t, 400, send(`{"enabled":true,"enabled":false}`).Code)
	require.Equal(t, 403, send(`{"enabled":true}`).Code)
	require.False(t, db.called)
}
