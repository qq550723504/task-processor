//go:build integration

package reportcenterapp

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http/httptest"
	"net/url"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/httproute"
	store "task-processor/internal/integration/persistence/reportcenter"
	"task-processor/internal/product/review"
	rc "task-processor/internal/reportcenter"
	reporthttp "task-processor/internal/reportcenter/httpapi"
	"task-processor/internal/workbenchcontext"
	"testing"
	"time"
)

func TestFactoryHTTPStoresHistoricalResultAndReplaysWithoutSource(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, e := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("reports_app"), tcpostgres.WithUsername("reports"), tcpostgres.WithPassword("reports"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, e)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, e := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, e)
	admin, e := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, e)
	adminRaw, _ := admin.DB()
	t.Cleanup(func() { _ = adminRaw.Close() })
	require.NoError(t, store.InstallSchema(admin))
	require.NoError(t, admin.Exec(`REVOKE CREATE ON DATABASE reports_app FROM PUBLIC; REVOKE CREATE ON SCHEMA public FROM PUBLIC; CREATE ROLE report_http_runtime LOGIN PASSWORD 'fixture-only' NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS`).Error)
	require.NoError(t, store.GrantRuntime(admin, "report_http_runtime"))
	u, e := url.Parse(dsn)
	require.NoError(t, e)
	u.User = url.UserPassword("report_http_runtime", "fixture-only")
	db, e := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, e)
	runtimeRaw, _ := db.DB()
	t.Cleanup(func() { _ = runtimeRaw.Close() })
	identity := authidentity.AuthenticatedIdentity{UserID: "actor-a", TenantID: "org-a", EffectiveOrganizationID: "org-a", TokenExpiresAt: time.Now().Add(time.Hour)}
	id := uuid.NewString()
	sourceCalls, writes := 0, 0
	resolver := resolveFunc(func(_ context.Context, p httproute.OrganizationAccessPolicy, in workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error) {
		require.Equal(t, "fixture-token", in.BearerToken)
		if p == httproute.OrganizationAccessPolicyLiveWrite {
			writes++
		}
		return identity, nil
	})
	view := review.View{ID: id, Owner: identity.UserID, Input: review.CreateInput{ProductKey: "product-a", BaseVersion: 1}, Before: "before", Title: "after", Policy: "title-review-v1", State: "accepted", Revision: 2}
	h, e := New(ctx, Dependencies{DB: db, Resolver: resolver, Policy: allowPolicy(true), Reviews: reviewFunc(func(context.Context, string) (review.View, error) { sourceCalls++; return view, nil })})
	require.NoError(t, e)
	routes, e := reporthttp.BuildRoutes(h)
	require.NoError(t, e)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(authidentity.WithAuthenticatedIdentity(c.Request.Context(), identity))
	})
	for _, d := range routes {
		router.Handle(d.Method, d.Path, d.Handler)
	}
	request := func(method, path, key, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, reporthttp.Base+path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer fixture-token")
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, r)
		return response
	}
	response := request("GET", "/sources/TITLE_REVIEW/"+id, "", "")
	require.Equal(t, 200, response.Code)
	require.NotContains(t, response.Body.String(), "after")
	var source rc.SourceInfo
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &source))
	body, _ := json.Marshal(struct {
		Source rc.SourceRef `json:"source"`
	}{source.Ref})
	key := uuid.NewString()
	response = request("POST", "", key, string(body))
	require.Equal(t, 200, response.Code)
	var first rc.Result
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &first))
	require.Equal(t, key, first.CommandID)
	require.Equal(t, 2, writes)
	// Response was lost; the original reader disappears before retry. Durable replay survives.
	h.Service.Sources = nil
	calls := sourceCalls
	response = request("POST", "", key, string(body))
	require.Equal(t, 200, response.Code)
	var replay rc.Result
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &replay))
	require.True(t, replay.Replayed)
	require.Equal(t, first.Report, replay.Report)
	require.Equal(t, calls, sourceCalls)
	response = request("POST", "/"+first.Report.ID+"/favorite", uuid.NewString(), `{"favorite":true}`)
	require.Equal(t, 200, response.Code)
	response = request("GET", "/summary", "", "")
	require.Equal(t, 200, response.Code)
	require.Contains(t, response.Body.String(), `"favorites":1`)
	identity.UserID = "other"
	response = request("GET", "/"+first.Report.ID, "", "")
	require.Equal(t, 404, response.Code)
}
