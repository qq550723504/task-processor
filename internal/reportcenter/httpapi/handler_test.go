package reporthttp

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/httproute"
	rc "task-processor/internal/reportcenter"
	"testing"
	"time"
)

type repository struct {
	rc.Repository
	calls int
}

func (r *repository) List(_ context.Context, s rc.Scope, _ rc.Filter) (rc.Page, error) {
	r.calls++
	return rc.Page{Items: []rc.ReportSummary{}}, nil
}
func TestStrictRequestAndDescriptors(t *testing.T) {
	r := &repository{}
	h := &Handler{Service: &rc.Service{Repository: r, Authorize: func(c context.Context, _ rc.Scope, _ bool) (context.Context, error) { return c, nil }}, Bind: func(c context.Context, _ string) (context.Context, error) {
		return authidentity.WithAuthenticatedIdentity(c, authidentity.AuthenticatedIdentity{UserID: "actor-a", EffectiveOrganizationID: "org-a"}), nil
	}}
	routes, e := BuildRoutes(h)
	require.NoError(t, e)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	for _, d := range routes {
		require.NoError(t, ValidateDescriptor(d))
		require.Equal(t, 10*time.Second, d.RequestTimeout)
		if d.Method == "GET" {
			require.True(t, d.RejectUnreadRequestBody)
			require.Equal(t, httproute.OrganizationAccessPolicyCachedRead, d.OrganizationAccessPolicy)
		} else {
			require.Equal(t, httproute.OrganizationAccessPolicyLiveWrite, d.OrganizationAccessPolicy)
		}
		router.Handle(d.Method, d.Path, d.Handler)
	}
	for _, query := range []string{"?actor=other", "?limit=20&limit=10", "?limit=51", "?view=other", "?search=%ZZ"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", Base+query, nil))
		require.Equal(t, 400, response.Code, query)
	}
	require.Zero(t, r.calls)
	for _, body := range []string{`{"favorite":true,"favorite":false}`, `{"favorite":true,"actor":"other"}`, `{"favorite":null}`, strings.Repeat("x", 2049)} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest("POST", Base+"/1510eced-9831-49de-a28c-098cb16deba1/favorite", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "1510eced-9831-49de-a28c-098cb16deba2")
		router.ServeHTTP(response, request)
		require.Equal(t, 400, response.Code)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, Base+"?view=all&limit=20", nil))
	require.Equal(t, 200, response.Code)
	require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
	require.Equal(t, 1, r.calls)
}
