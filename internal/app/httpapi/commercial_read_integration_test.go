//go:build integration

package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	orgresourceadapter "task-processor/internal/integration/orgresource"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingsubscription"
	"task-processor/internal/workbenchcontext"
)

// This fixture creates its own loopback-only, disposable PostgreSQL. It never
// accepts a caller-provided DSN or connects to application/customer databases.
func commercialPostgres(t *testing.T) (*gorm.DB, *config.DatabaseConfig) {
	t.Helper()
	name := "issue347-commercial-" + uuid.NewString()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	output, err := exec.CommandContext(ctx, "docker", "run", "--rm", "-d", "--name", name, "-e", "POSTGRES_USER=issue347", "-e", "POSTGRES_PASSWORD=issue347-fixture", "-e", "POSTGRES_DB=issue347", "-p", "127.0.0.1::5432", "postgres:17.2-alpine").CombinedOutput()
	require.NoError(t, err, string(output))
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer stopCancel()
		out, stopErr := exec.CommandContext(stopCtx, "docker", "stop", name).CombinedOutput()
		require.NoError(t, stopErr, string(out))
	})
	output, err = exec.CommandContext(ctx, "docker", "port", name, "5432/tcp").Output()
	require.NoError(t, err)
	address := strings.TrimSpace(string(output))
	require.True(t, strings.HasPrefix(address, "127.0.0.1:"))
	port, err := strconv.Atoi(strings.TrimPrefix(address, "127.0.0.1:"))
	require.NoError(t, err)
	dsn := fmt.Sprintf("host=127.0.0.1 port=%d user=issue347 password=issue347-fixture dbname=issue347 sslmode=disable", port)
	var db *gorm.DB
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	return db, &config.DatabaseConfig{Host: "127.0.0.1", Port: port, User: "commercial_reader", Password: "issue347-reader-fixture", Database: "issue347", MaxConnections: 4, MaxIdleConnections: 2}
}

type commercialGrantFixture struct {
	mode  atomic.Int32
	calls atomic.Int32
}

func (g *commercialGrantFixture) ListOwnProjectAuthorizations(ctx context.Context, _, _, project string) ([]authidentity.OrganizationGrant, error) {
	g.calls.Add(1)
	mode := g.mode.Load()
	if mode == 1 {
		return nil, nil
	}
	if mode == 2 {
		return nil, errors.New("fixture provider unavailable")
	}
	if mode == 3 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	grants := []authidentity.OrganizationGrant{}
	for _, org := range []string{"org-B", "org-C", "org-custom", "org-empty", "org-expired", "org-disabled", "org-future"} {
		grants = append(grants, authidentity.OrganizationGrant{OrganizationID: org, ProjectID: project, Roles: []string{"listingkit_operator"}})
	}
	grants = append(grants, authidentity.OrganizationGrant{OrganizationID: "org-viewer", ProjectID: project, Roles: []string{"listingkit_viewer"}})
	return grants, nil
}

func commercialTableSnapshot(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var tables []string
	require.NoError(t, db.Raw("SELECT tablename FROM pg_tables WHERE schemaname='public' AND tablename LIKE 'saas_%' ORDER BY tablename").Scan(&tables).Error)
	var rows []string
	for _, table := range tables {
		var values []string
		require.NoError(t, db.Raw(fmt.Sprintf(`SELECT row_to_json(t)::text || ':' || xmin::text FROM %q t`, table)).Scan(&values).Error)
		sort.Strings(values)
		rows = append(rows, table+":"+strings.Join(values, "|"))
	}
	sum := sha256.Sum256([]byte(strings.Join(rows, "\n")))
	return hex.EncodeToString(sum[:])
}

func TestCommercialHTTPPostgresBFFClientZeroWrites(t *testing.T) {
	db, dbConfig := commercialPostgres(t)
	repo := listingsubscription.NewGormRepository(db)
	require.NoError(t, listingsubscription.AutoMigrateRepository(db))
	require.NoError(t, orgresourceadapter.AutoMigrate(db))
	service, err := listingsubscription.NewService(repo)
	require.NoError(t, err)
	ctx := context.Background()
	now := time.Now().UTC()
	_, err = service.UpsertPlan(ctx, listingsubscription.PlanInput{Code: "paid_pilot", Name: "受邀试点", Active: true, Modules: []listingsubscription.PlanModuleInput{{ModuleCode: listingsubscription.ModuleListingKit, Limits: map[string]int{"listingkit_generations_succeeded": 0, "product_image_jobs_succeeded": 0, "product_image_jobs": 100}}, {ModuleCode: listingsubscription.ModuleOSSStorage, Limits: map[string]int{"storage_bytes_current": 0}}}}, "fixture-platform-admin")
	require.NoError(t, err)
	for _, org := range []string{"org-B", "org-C", "org-expired", "org-disabled", "org-future"} {
		input := listingsubscription.PlanApplyInput{PlanCode: "paid_pilot", Status: listingsubscription.StatusActive}
		if org == "org-expired" {
			expires := now.Add(-time.Hour)
			input.ExpiresAt = &expires
		}
		if org == "org-disabled" {
			input.Status = listingsubscription.StatusDisabled
		}
		if org == "org-future" {
			starts := now.Add(time.Hour)
			input.StartsAt = &starts
		}
		_, err = service.ApplyPlan(ctx, org, input, "fixture-platform-admin")
		require.NoError(t, err)
	}
	ledger := listingsubscription.NewGormUsageLedger(repo)
	_, err = service.UpsertPlan(ctx, listingsubscription.PlanInput{Code: "定制 plan", Name: "Existing paid contract", Active: true}, "fixture-platform-admin")
	require.NoError(t, err)
	_, err = service.ApplyPlan(ctx, "org-custom", listingsubscription.PlanApplyInput{PlanCode: "定制 plan", Status: listingsubscription.StatusActive}, "fixture-platform-admin")
	require.NoError(t, err)
	seed := func(org, metric string, q int64, month time.Time, commit bool) {
		module := listingsubscription.ModuleListingKit
		if metric == "storage_bytes_current" {
			module = listingsubscription.ModuleOSSStorage
		}
		key := uuid.NewString()
		reserved, reserveErr := ledger.Reserve(ctx, listingsubscription.ReserveUsageInput{TenantID: org, ModuleCode: module, Metric: metric, Quantity: q, PeriodKey: month.Format("2006-01"), SourceType: "commercial_fixture", SourceID: key, IdempotencyKey: key, OccurredAt: month})
		require.NoError(t, reserveErr)
		if commit {
			_, commitErr := ledger.Commit(ctx, reserved.Event.EventID)
			require.NoError(t, commitErr)
		}
	}
	seed("org-B", "listingkit_generations_succeeded", 1, now, true)
	seed("org-B", "listingkit_generations_succeeded", 1, now.AddDate(0, -1, 0), true)
	seed("org-C", "listingkit_generations_succeeded", 1, now, true)
	seed("org-C", "listingkit_generations_succeeded", 1, now, true)
	seed("org-B", "storage_bytes_current", 9007199254740993, now, true)
	seed("org-B", "storage_bytes_current", -1, now, false)
	// This role cannot perform any business write, even if the GET accidentally
	// invokes a mutation. Only the fixture setup connection has write authority.
	require.NoError(t, db.Exec("CREATE ROLE commercial_reader LOGIN PASSWORD 'issue347-reader-fixture'").Error)
	require.NoError(t, db.Exec("GRANT USAGE ON SCHEMA public TO commercial_reader").Error)
	require.NoError(t, db.Exec("GRANT SELECT ON ALL TABLES IN SCHEMA public TO commercial_reader").Error)
	require.NoError(t, db.Exec("ALTER ROLE commercial_reader SET default_transaction_read_only = on").Error)
	before := commercialTableSnapshot(t, db)
	appCfg := &config.Config{Database: dbConfig, Workbench: config.WorkbenchConfig{Enabled: true}}
	log := logrus.New()
	log.SetOutput(io.Discard)
	result, err := buildCommercialReadModule(appCfg, log)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, result.closer()) })
	registry := kernelmodule.NewRegistry()
	require.NoError(t, result.module.Register(registry))
	require.Len(t, registry.Routes(), 1)
	grants := &commercialGrantFixture{}
	resolver := workbenchcontext.NewResolver(workbenchcontext.NewGrantResolver(grants, workbenchcontext.NewGrantCache(nil)), "fixture-project", "fixture-contract", nil)
	auth := routeAuthDependencies{workbenchVerifier: mountedVerifierStub{identity: authidentity.AuthenticatedIdentity{UserID: "fixture-user", HomeOrganizationID: "home-A", TokenExpiresAt: now.Add(time.Hour)}}, organizationResolver: resolver, authorizer: authz.DefaultListingKitAuthorizer(), auditRecorder: workbenchcontext.NewStructuredAuditRecorder(log), auditNow: time.Now}
	appServer := buildHTTPServerFromRoutes(0, registry.Routes(), auth)
	mux := http.NewServeMux()
	mux.Handle("/", appServer.Handler)
	mux.HandleFunc("/fixture/mode", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		var value struct{ Mode int32 }
		if json.NewDecoder(io.LimitReader(r.Body, 64)).Decode(&value) != nil || value.Mode < 0 || value.Mode > 3 {
			w.WriteHeader(400)
			return
		}
		grants.mode.Store(value.Mode)
		w.WriteHeader(204)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	// Node starts an HTTP BFF around the actual Next route export. Auth.js token
	// retrieval is explicitly substituted; API/Resolver/Casbin/owner/PG are real.
	webDir, err := filepath.Abs(filepath.Join("..", "..", "..", "web", "listingkit-ui"))
	require.NoError(t, err)
	args := []string{"exec", "vitest", "run", "--config", "e2e/issue347-commercial.config.ts"}
	command := exec.Command("pnpm", args...)
	if runtime.GOOS == "windows" {
		command = exec.Command("cmd", "/c", "pnpm.cmd")
		command.Args = append(command.Args, args...)
	}
	command.Dir = webDir
	command.Env = append(os.Environ(), "COMMERCIAL_INTEGRATION_ORIGIN="+server.URL)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	require.NoError(t, err)
	require.Greater(t, grants.calls.Load(), int32(5), "actual live grants executed")
	require.Equal(t, before, commercialTableSnapshot(t, db), "all subscription, usage, resource tables and xmin unchanged")
	// An actual PostgreSQL permission failure is not converted to an empty/zero.
	require.NoError(t, db.Exec("REVOKE SELECT ON saas_usage_buckets FROM commercial_reader").Error)
	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/workbench/commercial/overview", nil)
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer fixture-token")
	request.Header.Set("X-Requested-Organization-ID", "org-B")
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, 503, response.StatusCode)
	require.Equal(t, before, commercialTableSnapshot(t, db))
	t.Logf("ZERO_WRITE: SELECT-only role + default_transaction_read_only + repeatable-read/read-only transaction + all saas table values/xmin SHA256 unchanged: %s", before)
}
