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
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	orgresourceadapter "task-processor/internal/integration/orgresource"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	moneystore "task-processor/internal/integration/persistence/money"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/storecenter"
	"task-processor/internal/workbenchcontext"
	contextapi "task-processor/internal/workbenchcontext/httpapi"
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

func (g *commercialGrantFixture) ListOwnProjectAuthorizations(ctx context.Context, _, subject, project string) ([]authidentity.OrganizationGrant, error) {
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
	role := "listingkit_operator"
	if subject == "viewer" || mode == 4 {
		role = "listingkit_viewer"
	}
	for _, org := range []string{"org-B", "org-C", "org-custom", "org-empty", "org-expired", "org-disabled", "org-future"} {
		grants = append(grants, authidentity.OrganizationGrant{OrganizationID: org, OrganizationName: org, ProjectID: project, Roles: []string{role}})
	}
	grants = append(grants, authidentity.OrganizationGrant{OrganizationID: "org-viewer", OrganizationName: "org-viewer", ProjectID: project, Roles: []string{"listingkit_viewer"}})
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

func commercialOwnerTableSnapshot(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var tables []string
	require.NoError(t, db.Raw("SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename").Scan(&tables).Error)
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

func commercialOwnerFixtureDB(t *testing.T, admin *gorm.DB, base *config.DatabaseConfig, suffix, role, password string, migrate func(*gorm.DB) error) (*gorm.DB, *config.DatabaseConfig) {
	t.Helper()
	database := "issue455_" + suffix + "_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	roleSQL := role
	require.NoError(t, admin.Exec("CREATE DATABASE \""+database+"\"").Error)
	require.NoError(t, admin.Exec("CREATE ROLE "+roleSQL+" LOGIN PASSWORD '"+password+"'").Error)
	ownerConfig := *base
	ownerConfig.Database = database
	ownerConfig.User = "issue347"
	ownerConfig.Password = "issue347-fixture"
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", ownerConfig.Host, ownerConfig.Port, ownerConfig.User, ownerConfig.Password, ownerConfig.Database)
	owner, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	ownerSQL, err := owner.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ownerSQL.Close()) })
	require.NoError(t, migrate(owner))
	require.NoError(t, admin.Exec("GRANT CONNECT ON DATABASE \""+database+"\" TO "+roleSQL).Error)
	require.NoError(t, owner.Exec("GRANT USAGE ON SCHEMA public TO "+roleSQL).Error)
	require.NoError(t, owner.Exec("GRANT SELECT ON ALL TABLES IN SCHEMA public TO "+roleSQL).Error)
	require.NoError(t, owner.Exec("ALTER ROLE "+roleSQL+" SET default_transaction_read_only=on").Error)
	reader := *base
	reader.Database, reader.User, reader.Password = database, role, password
	return owner, &reader
}

func openCommercialFixtureReader(t *testing.T, cfg *config.DatabaseConfig) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	return db
}

func TestCommercialHTTPPostgresBFFClientZeroWrites(t *testing.T) {
	db, dbConfig := commercialPostgres(t)
	require.NoError(t, orgresourceadapter.AutoMigrate(db))
	ctx := context.Background()
	now := time.Now().UTC()
	for _, seed := range []struct {
		org      string
		quantity int64
	}{{"org-B", 9007199254740993}, {"org-C", 2}} {
		require.NoError(t, db.Exec("INSERT INTO saas_organization_resource_buckets (organization_id,resource_type,available,allocated,reserved,consumed,created_at,updated_at) VALUES (?, 'ai_point', ?,0,0,0,?,?)", seed.org, seed.quantity, now, now).Error)
	}
	// This role cannot perform any business write, even if the GET accidentally
	// invokes a mutation. Only the fixture setup connection has write authority.
	require.NoError(t, db.Exec("CREATE ROLE commercial_reader LOGIN PASSWORD 'issue347-reader-fixture'").Error)
	require.NoError(t, db.Exec("GRANT USAGE ON SCHEMA public TO commercial_reader").Error)
	require.NoError(t, db.Exec("GRANT SELECT ON ALL TABLES IN SCHEMA public TO commercial_reader").Error)
	require.NoError(t, db.Exec("ALTER ROLE commercial_reader SET default_transaction_read_only = on").Error)
	billingOwner, billingConfig := commercialOwnerFixtureDB(t, db, dbConfig, "billing", "issue455_billing_reader", "issue455-billing-fixture", func(owner *gorm.DB) error {
		if err := commercialstore.AutoMigrate(owner); err != nil {
			return err
		}
		return orgresourceadapter.AutoMigrate(owner)
	})
	moneyOwner, moneyConfig := commercialOwnerFixtureDB(t, db, dbConfig, "money", "issue455_money_reader", "issue455-money-fixture", moneystore.AutoMigrate)
	billingBefore, moneyBefore := commercialOwnerTableSnapshot(t, billingOwner), commercialOwnerTableSnapshot(t, moneyOwner)
	before := commercialTableSnapshot(t, db)
	appCfg := &config.Config{Database: dbConfig, Workbench: config.WorkbenchConfig{Enabled: true}}
	log := logrus.New()
	log.SetOutput(io.Discard)
	storeOwner, storeConfig := commercialOwnerFixtureDB(t, db, dbConfig, "stores", "commercial_store_reader", "unified-store-fixture", func(owner *gorm.DB) error {
		sqlDB, err := owner.DB()
		if err != nil {
			return err
		}
		tx, err := sqlDB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if err = storecenter.InstallCurrentSchemaTx(ctx, tx); err != nil {
			return err
		}
		return tx.Commit()
	})
	storeBefore := commercialOwnerTableSnapshot(t, storeOwner)
	module, err := buildUnifiedCommercialRead(ctx, openCommercialFixtureReader(t, dbConfig), openCommercialFixtureReader(t, storeConfig), authz.DefaultListingKitAuthorizer())
	require.NoError(t, err)
	registry := kernelmodule.NewRegistry()
	require.NoError(t, module.Register(registry))
	billingModule, err := buildCommercialBillingModule(ctx, openCommercialFixtureReader(t, billingConfig), openCommercialFixtureReader(t, moneyConfig), authz.DefaultListingKitAuthorizer(), appCfg)
	require.NoError(t, err)
	require.NoError(t, billingModule.Register(registry))
	require.Greater(t, len(registry.Routes()), 1)
	require.NoError(t, contextapi.NewModule(contextapi.NewHandlerWithWorkbenchAuthorizer(authz.DefaultListingKitAuthorizer())).Register(registry))
	grants := &commercialGrantFixture{}
	var verifier zitadel.Verifier = mountedVerifierStub{identity: authidentity.AuthenticatedIdentity{UserID: "fixture-user", HomeOrganizationID: "home-A", TokenExpiresAt: now.Add(time.Hour)}}
	resolver := workbenchcontext.NewResolver(workbenchcontext.NewGrantResolver(grants, workbenchcontext.NewGrantCache(nil)), "fixture-project", "fixture-contract", nil)
	auth := routeAuthDependencies{workbenchVerifier: verifier, organizationResolver: resolver, authorizer: authz.DefaultListingKitAuthorizer(), auditRecorder: workbenchcontext.NewStructuredAuditRecorder(log), auditNow: time.Now}
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
	require.Equal(t, before, commercialTableSnapshot(t, db), "all resource tables and xmin unchanged")
	require.Equal(t, billingBefore, commercialOwnerTableSnapshot(t, billingOwner), "GET routes changed commercial billing owner rows")
	require.Equal(t, moneyBefore, commercialOwnerTableSnapshot(t, moneyOwner), "GET routes changed canonical money owner rows")
	// An actual PostgreSQL permission failure is not converted to an empty/zero.
	require.NoError(t, db.Exec("REVOKE SELECT ON saas_organization_resource_buckets FROM commercial_reader").Error)
	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/workbench/commercial/overview", nil)
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer fixture-token")
	request.Header.Set("X-Requested-Organization-ID", "org-B")
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, 200, response.StatusCode)
	var payload struct {
		Resources struct {
			State string `json:"state"`
			Value any    `json:"value"`
		} `json:"resources"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&payload))
	require.Equal(t, "unavailable", payload.Resources.State)
	require.Nil(t, payload.Resources.Value)
	require.Equal(t, storeBefore, commercialOwnerTableSnapshot(t, storeOwner))
	require.Equal(t, before, commercialTableSnapshot(t, db))
	t.Logf("ZERO_WRITE: SELECT-only role + default_transaction_read_only + all native owner table values/xmin SHA256 unchanged: %s", before)
}
