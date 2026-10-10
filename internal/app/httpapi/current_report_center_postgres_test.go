//go:build integration

package httpapi

import (
	"context"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	store "task-processor/internal/integration/persistence/reportcenter"
	kernelmodule "task-processor/internal/kernel/module"
	rc "task-processor/internal/reportcenter"
	reporthttp "task-processor/internal/reportcenter/httpapi"
	"testing"
	"time"
)

func TestReportCenterNormalCompositionReadsSavedReportsWithoutOriginalOwners(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("reports"), tcpostgres.WithUsername("reports"), tcpostgres.WithPassword("fixture-only"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	owner, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	raw, _ := owner.DB()
	t.Cleanup(func() { _ = raw.Close() })
	require.NoError(t, owner.Exec(`REVOKE CREATE ON DATABASE reports FROM PUBLIC; REVOKE CREATE ON SCHEMA public FROM PUBLIC; CREATE ROLE report_center_runtime LOGIN PASSWORD 'fixture-runtime' NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS`).Error)
	require.NoError(t, owner.Transaction(func(tx *gorm.DB) error {
		if err := store.InstallSchema(tx); err != nil {
			return err
		}
		return store.GrantRuntime(tx, "report_center_runtime")
	}))
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.User = url.UserPassword("report_center_runtime", "fixture-runtime")
	reports, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, _ := reports.DB()
	t.Cleanup(func() { _ = pool.Close() })
	repo, err := store.New(reports)
	require.NoError(t, err)
	scope := rc.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	snapshot := rc.Snapshot{SourceInfo: rc.SourceInfo{Ref: rc.SourceRef{Kind: "TITLE_REVIEW", ID: uuid.NewString(), Version: "1:pending"}, Title: "保存时审核", ProductKey: "product-a"}, Content: rc.Document{SchemaVersion: 1, Sections: []rc.Section{{Title: "历史标题", Fields: []rc.Field{{Label: "提案", Value: "saved-before-source-disappeared"}}}}}}
	result, err := repo.Save(ctx, scope, uuid.NewString(), rc.Fingerprint("save", snapshot.Ref), snapshot, func(context.Context) error { return nil })
	require.NoError(t, err)
	identity := authidentity.AuthenticatedIdentity{UserID: scope.ActorID, TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, EffectiveMemberID: "member-a", HomeOrganizationID: scope.OrganizationID, Roles: []string{"listingkit_admin"}, TokenExpiresAt: time.Now().Add(time.Hour)}
	resolver := &productReviewResolverStub{value: identity}
	deps := routeAuthDependencies{workbenchVerifier: mountedVerifierStub{identity: identity}, organizationResolver: resolver}
	factories := currentApplicationFactories{
		buildWorkbench: func(*config.Config, *logrus.Logger) (workbenchContextBuildResult, error) {
			return workbenchContextBuildResult{module: currentApplicationTestModule{name: "workbench", routes: currentWorkbenchApplicationRoutes}, authDependencies: &deps}, nil
		},
		buildSourceAccount: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, nil },
		buildCommercial:    func(*gorm.DB, *gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, nil },
	}
	server, err := buildCurrentApplication(ctx, &gorm.DB{}, currentApplicationTestConfig(), logrus.New(), factories, WithReportCenter(reports))
	require.NoError(t, err)
	request := func(method, path, body, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, reporthttp.Base+path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer fixture-token")
		r.Header.Set("X-Requested-Organization-ID", resolver.value.EffectiveOrganizationID)
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		return w
	}
	response := request(http.MethodGet, "/"+result.Report.ID, "", "")
	require.Equal(t, 200, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), "saved-before-source-disappeared")
	response = request(http.MethodPost, "/"+result.Report.ID+"/favorite", `{"favorite":true}`, uuid.NewString())
	require.Equal(t, 200, response.Code, response.Body.String())
	response = request(http.MethodGet, "/summary", "", "")
	require.Equal(t, 200, response.Code, response.Body.String())
	for _, kind := range []string{"TITLE_REVIEW", "SHEIN_RECORD"} {
		response = request(http.MethodGet, "/sources/"+kind+"/"+snapshot.Ref.ID, "", "")
		require.Equal(t, 503, response.Code, response.Body.String())
	}
	response = request(http.MethodGet, "", "", "")
	require.Equal(t, 200, response.Code, response.Body.String())
	response = request(http.MethodGet, "/summary", "unexpected-body", "")
	require.Equal(t, 400, response.Code)
	resolver.value.UserID = "other-actor"
	response = request(http.MethodGet, "/"+result.Report.ID, "", "")
	require.NotEqual(t, 200, response.Code)
	resolver.value = identity
	resolver.value.EffectiveOrganizationID = "org-b"
	resolver.value.TenantID = "org-b"
	response = request(http.MethodGet, "/"+result.Report.ID, "", "")
	require.NotEqual(t, 200, response.Code)
	require.NoError(t, store.VerifySchema(ctx, reports))
	// The original factory/persistence tests cover capture and replay. This check
	// binds optional-source behavior and protected routes to native composition.
}
