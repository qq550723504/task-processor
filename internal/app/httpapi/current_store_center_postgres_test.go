//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	storeschema "task-processor/internal/app/schema/storecenter"
	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingsubscription"
	"task-processor/internal/storecenter"
)

// Synthetic development checks use a newly allocated container, never an
// environment DSN or an existing account/application database.
func TestCurrentStorePostgresDelivery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("commercial"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("synthetic-store-test"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	open := func(database, role string) *gorm.DB {
		t.Helper()
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		u.Path = "/" + database
		u.User = url.UserPassword(role, "synthetic-store-test")
		db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, err)
		pool, err := db.DB()
		require.NoError(t, err)
		pool.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = pool.Close() })
		return db
	}
	commercial := open("commercial", "postgres")
	for _, q := range []string{
		`CREATE ROLE store_center_owner LOGIN PASSWORD 'synthetic-store-test'`,
		`CREATE ROLE store_center_runtime LOGIN PASSWORD 'synthetic-store-test'`,
		`CREATE ROLE store_quota_runtime LOGIN PASSWORD 'synthetic-store-test'`,
		`CREATE DATABASE store_center OWNER store_center_owner`,
		`REVOKE CREATE ON SCHEMA public FROM PUBLIC`,
		`REVOKE ALL ON DATABASE commercial FROM PUBLIC`,
	} {
		require.NoError(t, commercial.Exec(q).Error)
	}
	recordsOwner := open("store_center", "store_center_owner")
	require.NoError(t, storeschema.Migrate(ctx, recordsOwner))
	require.NoError(t, storeschema.Migrate(ctx, recordsOwner))
	for _, q := range []string{`REVOKE CREATE ON SCHEMA public FROM PUBLIC`, `REVOKE ALL ON DATABASE store_center FROM PUBLIC`, `GRANT CONNECT ON DATABASE store_center TO store_center_runtime`, `GRANT USAGE ON SCHEMA public TO store_center_runtime`, `GRANT SELECT,INSERT,UPDATE ON workbench_stores TO store_center_runtime`, `GRANT SELECT,INSERT ON workbench_store_audit_logs TO store_center_runtime`} {
		require.NoError(t, recordsOwner.Exec(q).Error)
	}
	require.NoError(t, listingsubscription.AutoMigrateRepository(commercial))
	require.NoError(t, listingsubscription.GrantStoreQuotaRuntimeAccess(ctx, commercial))
	entitlement, err := listingsubscription.NewGormRepository(commercial).UpsertEntitlement(ctx, &listingsubscription.Entitlement{TenantID: "B", ModuleCode: listingsubscription.ModuleStoreManagement, Status: listingsubscription.StatusActive, Limits: map[string]int{"store_count": 2}})
	require.NoError(t, err)
	require.NotNil(t, entitlement)
	records, quota := open("store_center", "store_center_runtime"), open("commercial", "store_quota_runtime")
	module, err := buildCurrentStoreCenterModule(ctx, records, quota)
	require.NoError(t, err)
	f := newAccountFixture(t)
	cfg := &config.Config{Workbench: config.WorkbenchConfig{Enabled: true}, ListingKit: config.ListingKitConfig{Zitadel: config.ListingKitZitadelConfig{IssuerURL: f.provider.URL, ClientID: "fixture-client", ClientSecret: "fixture-secret", ProjectID: "project", AuthorizationAPIURL: f.provider.URL}}}
	auth, err := buildWorkbenchContextModule(cfg, logrus.New(), defaultWorkbenchContextFactories())
	require.NoError(t, err)
	reg := kernelmodule.NewRegistry()
	require.NoError(t, module.Register(reg))
	server := buildCurrentApplicationHTTPServer(reg.Routes(), *auth.authDependencies)
	request := func(method, path, org, body, key string, version int64) (int, map[string]any) {
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
		if version > 0 {
			r.Header.Set("If-Match", fmt.Sprintf("\"%d\"", version))
		}
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		out := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	root := "/api/v1/workbench/stores"
	status, out := request("GET", root, "B", "", "", 0)
	require.Equal(t, 200, status, out)
	status, out = request("POST", root, "B", `{"name":"Fixture","platform":"shein","region":"US"}`, uuid.NewString(), 0)
	require.Equal(t, 403, status, out)
	f.role.Store("listingkit_operator")
	key := uuid.NewString()
	body := `{"name":"Fixture","platform":"shein","region":"US"}`
	status, out = request("POST", root, "B", body, key, 0)
	require.Equal(t, 201, status, out)
	id := out["id"].(string)
	require.Equal(t, "active", out["recordStatus"])
	require.Equal(t, "pending_activation", out["serviceStatus"])
	require.Nil(t, out["serviceExpiresAt"])
	require.Equal(t, "unavailable", out["connectionStatus"])
	require.NotContains(t, out, "lifecycleStatus")
	status, replay := request("POST", root, "B", body, key, 0)
	require.Equal(t, 201, status, replay)
	require.Equal(t, id, replay["id"])
	status, out = request("PUT", root+"/"+id, "B", `{"name":"Edited","region":"EU"}`, "", 2)
	require.Equal(t, 200, status, out)
	require.Equal(t, "Edited", out["name"])

	start := time.Now().UTC().Truncate(time.Microsecond)
	expiry := start.Add(30 * 24 * time.Hour)
	require.NoError(t, recordsOwner.Table("workbench_stores").Where("id = ?", id).Updates(map[string]any{"service_status": "active", "service_started_at": start, "service_expires_at": expiry}).Error)
	status, out = request("POST", root+"/"+id+"/disable", "B", "", "", 3)
	require.Equal(t, 200, status, out)
	require.Equal(t, "disabled", out["recordStatus"])
	require.Equal(t, "active", out["serviceStatus"])
	require.Equal(t, expiry.Format(time.RFC3339Nano), out["serviceExpiresAt"])
	status, out = request("POST", root+"/"+id+"/enable", "B", "", "", 4)
	require.Equal(t, 200, status, out)
	require.Equal(t, "active", out["serviceStatus"])
	require.Equal(t, expiry.Format(time.RFC3339Nano), out["serviceExpiresAt"])
	status, out = request("GET", root+"/"+id, "C", "", "", 0)
	require.Equal(t, 404, status, out)
	require.Greater(t, f.grantReads.Load(), int32(5))
	// A fresh module/pool reads the durable results after reconstructing runtime.
	fresh, err := buildCurrentStoreCenterModule(ctx, open("store_center", "store_center_runtime"), open("commercial", "store_quota_runtime"))
	require.NoError(t, err)
	reg = kernelmodule.NewRegistry()
	require.NoError(t, fresh.Register(reg))
	server = buildCurrentApplicationHTTPServer(reg.Routes(), *auth.authDependencies)
	status, out = request("GET", root+"/"+id, "B", "", "", 0)
	require.Equal(t, 200, status, out)
	require.Equal(t, "Edited", out["name"])
	require.Equal(t, "active", out["serviceStatus"])
	require.Equal(t, expiry.Format(time.RFC3339Nano), out["serviceExpiresAt"])
	f.revoked.Store(true)
	status, out = request("GET", root+"/"+id, "B", "", "", 0)
	require.Equal(t, 403, status, out)
	f.revoked.Store(false)
	status, out = request("DELETE", root+"/"+id, "B", "", uuid.NewString(), 5)
	require.Equal(t, 403, status, out)
	f.role.Store("listingkit_admin")
	deleteKey := uuid.NewString()
	status, out = request("DELETE", root+"/"+id, "B", "", deleteKey, 5)
	require.Equal(t, 200, status, out)
	status, out = request("DELETE", root+"/"+id, "B", "", deleteKey, 5)
	require.Equal(t, 200, status, out)
	status, out = request("GET", root, "B", "", "", 0)
	require.Equal(t, 200, status, out)
	require.EqualValues(t, 0, out["quota"].(map[string]any)["used"])
	// Serving credentials cannot mutate entitlements, delete audit or install DDL.
	require.Error(t, quota.Exec(`UPDATE saas_tenant_entitlements SET status='inactive'`).Error)
	require.Error(t, records.Exec(`DELETE FROM workbench_store_audit_logs`).Error)
	require.Error(t, records.Exec(`CREATE TABLE forbidden(id int)`).Error)
	require.Error(t, records.Exec(`CREATE TEMP TABLE workbench_stores(id text)`).Error)
	require.Error(t, quota.Exec(`CREATE TEMP TABLE saas_store_quota_buckets(organization_id text)`).Error)
	require.NoError(t, recordsOwner.Exec(`GRANT UPDATE ON workbench_store_audit_logs TO store_center_runtime`).Error)
	require.Error(t, storecenter.VerifyRuntimePermissions(ctx, records))
	require.NoError(t, recordsOwner.Exec(`REVOKE UPDATE ON workbench_store_audit_logs FROM store_center_runtime`).Error)
	require.NoError(t, commercial.Exec(`GRANT UPDATE(status) ON saas_tenant_entitlements TO store_quota_runtime`).Error)
	require.Error(t, listingsubscription.VerifyStoreQuotaRuntime(ctx, quota))
	require.NoError(t, commercial.Exec(`REVOKE UPDATE(status) ON saas_tenant_entitlements FROM store_quota_runtime`).Error)

	for _, target := range []struct {
		name, role     string
		owner, runtime *gorm.DB
		verify         func(context.Context, *gorm.DB) error
	}{
		{"records", "store_center_runtime", recordsOwner, records, storecenter.VerifyRuntimePermissions},
		{"quota", "store_quota_runtime", commercial, quota, listingsubscription.VerifyStoreQuotaRuntime},
	} {
		t.Run("extra-schema-permissions-"+target.name, func(t *testing.T) {
			require.NoError(t, target.owner.Exec(`CREATE SCHEMA pgx_store_shadow`).Error)
			require.NoError(t, target.owner.Exec(`CREATE TABLE pgx_store_shadow.workbench_stores(id text)`).Error)
			require.NoError(t, target.owner.Exec(`GRANT USAGE ON SCHEMA pgx_store_shadow TO `+target.role).Error)
			require.NoError(t, target.owner.Exec(`GRANT SELECT ON pgx_store_shadow.workbench_stores TO `+target.role).Error)
			require.Error(t, target.verify(ctx, target.runtime), "additional schema grants must not bypass the narrow runtime contract")
			require.NoError(t, target.owner.Exec(`REVOKE SELECT ON pgx_store_shadow.workbench_stores FROM `+target.role).Error)
			require.NoError(t, target.runtime.Exec(`SET search_path TO pgx_store_shadow,public`).Error)
			require.Error(t, target.verify(ctx, target.runtime), "unqualified tables must resolve the inspected canonical schema")
			require.NoError(t, target.runtime.Exec(`RESET search_path`).Error)
			require.NoError(t, target.owner.Exec(`DROP TABLE pgx_store_shadow.workbench_stores`).Error)
			require.NoError(t, target.owner.Exec(`DROP SCHEMA pgx_store_shadow`).Error)
		})
	}
	t.Run("wrong-record-search-path", func(t *testing.T) {
		require.NoError(t, records.Exec(`SET search_path TO pg_catalog`).Error)
		defer func() { require.NoError(t, records.Exec(`RESET search_path`).Error) }()
		require.Error(t, storecenter.VerifyRuntimePermissions(ctx, records), "catalog preflight must match the unqualified repository target")
	})
	t.Run("wrong-quota-search-path", func(t *testing.T) {
		require.NoError(t, quota.Exec(`SET search_path TO pg_catalog`).Error)
		defer func() { require.NoError(t, quota.Exec(`RESET search_path`).Error) }()
		require.Error(t, listingsubscription.VerifyStoreQuotaRuntime(ctx, quota), "quota must resolve the inspected canonical public facts")
	})
	t.Run("missing-store-primary-identity", func(t *testing.T) {
		require.NoError(t, recordsOwner.Exec(`ALTER TABLE workbench_stores DROP CONSTRAINT workbench_stores_pkey`).Error)
		require.Error(t, storecenter.VerifyCurrentSchema(ctx, records))
		require.NoError(t, recordsOwner.Exec(`ALTER TABLE workbench_stores ADD PRIMARY KEY(id)`).Error)
	})
	t.Run("missing-quota-bucket-primary", func(t *testing.T) {
		require.NoError(t, commercial.Exec(`ALTER TABLE saas_store_quota_buckets DROP CONSTRAINT saas_store_quota_buckets_pkey`).Error)
		require.Error(t, listingsubscription.VerifyStoreQuotaRuntime(ctx, quota))
		require.NoError(t, commercial.Exec(`ALTER TABLE saas_store_quota_buckets ADD PRIMARY KEY(organization_id)`).Error)
	})
	t.Run("missing-quota-uniqueness", func(t *testing.T) {
		require.NoError(t, commercial.Exec(`DROP INDEX idx_saas_store_quota_org_request`).Error)
		require.Error(t, listingsubscription.VerifyStoreQuotaRuntime(ctx, quota), "replay requires the installed canonical uniqueness boundary")
		require.NoError(t, commercial.Exec(`CREATE UNIQUE INDEX idx_saas_store_quota_org_request ON saas_store_quota_allocations(organization_id,request_key)`).Error)
	})
	require.NoError(t, storecenter.VerifyCurrentSchema(ctx, records))
	require.NoError(t, listingsubscription.VerifyStoreQuotaRuntime(ctx, quota))
	t.Run("weakened-store-constraint", func(t *testing.T) {
		require.NoError(t, recordsOwner.Exec(`ALTER TABLE workbench_stores DROP CONSTRAINT store_service_period`).Error)
		require.NoError(t, recordsOwner.Exec(`ALTER TABLE workbench_stores ADD CONSTRAINT store_service_period CHECK (TRUE)`).Error)
		require.Error(t, storecenter.VerifyCurrentSchema(ctx, records), "a constraint name alone is not a service contract")
	})

}
