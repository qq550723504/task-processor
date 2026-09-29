//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http/httptest"
	"strings"
	"task-processor/internal/core/config"
	store "task-processor/internal/integration/persistence/knowledge"
	kernelmodule "task-processor/internal/kernel/module"
	k "task-processor/internal/knowledge"
	knowledgehttp "task-processor/internal/knowledge/httpapi"
	"testing"
)

type unusedKnowledgeObjects struct{ k.KnowledgeObjectStore }

func TestCurrentKnowledgePostgresLiveAuthorizationAndRestart(t *testing.T) {
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine", tcpostgres.WithDatabase("knowledge"), tcpostgres.WithUsername("knowledge_owner"), tcpostgres.WithPassword("isolated-test-password"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, _ := db.DB()
	t.Cleanup(func() { _ = pool.Close() })
	require.NoError(t, store.Install(ctx, db))
	f := newAccountFixture(t)
	cfg := &config.Config{Workbench: config.WorkbenchConfig{Enabled: true}, ListingKit: config.ListingKitConfig{Zitadel: config.ListingKitZitadelConfig{IssuerURL: f.provider.URL, ClientID: "fixture-client", ClientSecret: "fixture-secret", ProjectID: "project", AuthorizationAPIURL: f.provider.URL}}}
	auth, err := buildWorkbenchContextModule(cfg, logrus.New(), defaultWorkbenchContextFactories())
	require.NoError(t, err)
	construct := func() *httptest.Server {
		repo, e := store.NewRepository(ctx, db)
		require.NoError(t, e)
		service, e := k.NewService(repo, unusedKnowledgeObjects{})
		require.NoError(t, e)
		handler, e := knowledgehttp.NewHandler(service)
		require.NoError(t, e)
		reg := kernelmodule.NewRegistry()
		require.NoError(t, knowledgehttp.NewModule(handler).Register(reg))
		httpServer := buildCurrentApplicationHTTPServer(reg.Routes(), *auth.authDependencies)
		return httptest.NewServer(httpServer.Handler)
	}
	server := construct()
	t.Cleanup(func() { server.Close() })
	request := func(method, path, org, body, key string) (int, map[string]any) {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer u1")
		r.Header.Set("X-Requested-Organization-ID", org)
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		// Exercise the same composed runtime Handler without going through a test BFF.
		clientRequest := r.Clone(ctx)
		clientRequest.URL.Scheme = "http"
		clientRequest.URL.Host = strings.TrimPrefix(server.URL, "http://")
		clientRequest.RequestURI = ""
		response, e := server.Client().Do(clientRequest)
		require.NoError(t, e)
		defer response.Body.Close()
		w.WriteHeader(response.StatusCode)
		out := map[string]any{}
		require.NoError(t, json.NewDecoder(response.Body).Decode(&out))
		require.Contains(t, response.Header.Get("Cache-Control"), "no-store")
		return response.StatusCode, out
	}
	root := "/api/v1/workbench/knowledge-bases"
	status, _ := request("GET", root, "B", "", "")
	require.Equal(t, 403, status)
	f.role.Store("listingkit_operator")
	status, _ = request("GET", root, "B", "", "")
	require.Equal(t, 200, status)
	status, _ = request("POST", root, "B", `{"name":"Brand"}`, uuid.NewString())
	require.Equal(t, 403, status)
	f.role.Store("listingkit_admin")
	key := uuid.NewString()
	status, result := request("POST", root, "B", `{"name":"Brand"}`, key)
	require.Equal(t, 201, status, result)
	id := result["knowledgeBase"].(map[string]any)["id"].(string)
	status, replay := request("POST", root, "B", `{"name":"Brand"}`, key)
	require.Equal(t, 201, status)
	require.Equal(t, id, replay["knowledgeBase"].(map[string]any)["id"])
	status, _ = request("GET", root+"/"+id, "C", "", "")
	require.Equal(t, 404, status)
	server.Close()
	server = construct()
	status, out := request("GET", root+"/"+id, "B", "", "")
	require.Equal(t, 200, status)
	require.Equal(t, "Brand", out["name"])
	f.revoked.Store(true)
	status, _ = request("GET", root+"/"+id, "B", "", "")
	require.Equal(t, 403, status)
	f.revoked.Store(false)
	f.role.Store("listingkit_viewer")
	status, _ = request("GET", root+"/"+id, "B", "", "")
	require.Equal(t, 403, status)
	require.Greater(t, f.grantReads.Load(), int32(5))
}
