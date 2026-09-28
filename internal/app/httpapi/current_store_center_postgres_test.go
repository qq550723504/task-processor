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
	storeapp "task-processor/internal/app/storecenter"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/integration/shein"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
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
	for _, q := range []string{`REVOKE CREATE ON SCHEMA public FROM PUBLIC`, `REVOKE ALL ON DATABASE store_center FROM PUBLIC`, `GRANT CONNECT ON DATABASE store_center TO store_center_runtime`, `GRANT USAGE ON SCHEMA public TO store_center_runtime`, `GRANT SELECT,INSERT,UPDATE ON workbench_stores TO store_center_runtime`, `GRANT SELECT,INSERT ON workbench_store_audit_logs TO store_center_runtime`, `GRANT SELECT,INSERT,UPDATE ON workbench_store_member_grants TO store_center_runtime`, `GRANT SELECT,INSERT ON workbench_store_member_grant_operations TO store_center_runtime`, `GRANT SELECT,INSERT,UPDATE ON workbench_store_service_operations TO store_center_runtime`} {
		require.NoError(t, recordsOwner.Exec(q).Error)
	}
	require.NoError(t, listingsubscription.AutoMigrateRepository(commercial))
	for _, statement := range []string{`GRANT SELECT,INSERT,UPDATE ON workbench_store_connections,workbench_store_connection_attempts TO store_center_runtime`, `GRANT SELECT,INSERT ON workbench_store_merchant_bindings TO store_center_runtime`} {
		require.NoError(t, recordsOwner.Exec(statement).Error)
	}
	require.NoError(t, listingsubscription.GrantStoreQuotaRuntimeAccess(ctx, commercial))
	entitlement, err := listingsubscription.NewGormRepository(commercial).UpsertEntitlement(ctx, &listingsubscription.Entitlement{TenantID: "B", ModuleCode: listingsubscription.ModuleStoreManagement, Status: listingsubscription.StatusActive, Limits: map[string]int{"store_count": 2}})
	require.NoError(t, err)
	require.NotNil(t, entitlement)
	records, quota := open("store_center", "store_center_runtime"), open("commercial", "store_quota_runtime")
	charges, err := orgresource.NewConsumerChargeService(currentStoreChargeFixture{}, map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner{orgresource.ConsumerStoreService: currentStoreChargeFixture{}})
	require.NoError(t, err)
	module, err := buildCurrentStoreCenterModule(ctx, records, quota, authz.DefaultListingKitAuthorizer(), charges, nil, nil)
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
	status, connection := request("GET", root+"/"+id+"/connection", "B", "", "", 0)
	require.Equal(t, 200, status, connection)
	require.Equal(t, "unavailable", connection["connectionStatus"])
	status, connection = request("POST", root+"/"+id+"/connection/begin", "B", "", uuid.NewString(), 2)
	require.Equal(t, 503, status, connection)
	require.Equal(t, "STORE_OFFICIAL_SETUP_UNAVAILABLE", connection["code"])
	status, connection = request("POST", root+"/"+id+"/activate", "B", `{}`, uuid.NewString(), 2)
	require.Equal(t, 503, status, connection)
	require.Equal(t, "STORE_CONNECTION_UNAVAILABLE", connection["code"])
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
	fresh, err := buildCurrentStoreCenterModule(ctx, open("store_center", "store_center_runtime"), open("commercial", "store_quota_runtime"), authz.DefaultListingKitAuthorizer(), charges, nil, nil)
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
	t.Run("official-credential-shape-on-narrow-postgres", func(t *testing.T) {
		repo, err := storecenter.NewMemberScopedStoreRepository(records, postgresConnectionAccess{})
		require.NoError(t, err)
		candidate, err := storecenter.NewStore(storecenter.CreateStoreInput{ID: uuid.NewString(), OrganizationID: "official-fixture", ActorSubject: "synthetic-operator", Name: "Official fixture", Platform: "shein", Region: "SG", ExternalStoreID: "metadata-only", CreateIdempotencyKey: uuid.NewString(), QuotaAllocationID: uuid.NewString(), OccurredAt: time.Now().UTC().Add(-time.Minute)})
		require.NoError(t, err)
		candidate, _, err = repo.CreateOrReplay(ctx, "official-fixture", candidate)
		require.NoError(t, err)
		require.NoError(t, candidate.TransitionTo(storecenter.RecordStatusActive, "synthetic-operator", candidate.UpdatedAt().Add(time.Second)))
		require.NoError(t, repo.Save(ctx, "official-fixture", candidate, 1))
		protection, err := shein.NewCredentialProtection("synthetic-key", make([]byte, 32))
		require.NoError(t, err)
		app, err := storeapp.NewOfficialConnections(repo, postgresConnectionProvider{}, protection)
		require.NoError(t, err)
		begin, err := app.Begin(ctx, storecenter.OfficialConnectionCommand{OrganizationID: "official-fixture", StoreID: candidate.ID(), AttemptID: uuid.NewString(), ExpectedStoreVersion: candidate.Version()})
		require.NoError(t, err)
		authorized, err := url.Parse(begin.AuthorizationURL)
		require.NoError(t, err)
		_, query, _ := strings.Cut(authorized.Fragment, "?")
		params, err := url.ParseQuery(query)
		require.NoError(t, err)
		complete := storecenter.CompleteOfficialConnection{OrganizationID: "official-fixture", StoreID: candidate.ID(), AttemptID: begin.AttemptID, AppID: "synthetic-app", State: params.Get("state"), TempToken: "synthetic-temp"}
		view, err := app.Complete(ctx, complete)
		require.NoError(t, err)
		require.Equal(t, storecenter.ConnectionStatusConnected, view.Status)
		current, err := repo.Get(ctx, "official-fixture", candidate.ID())
		require.NoError(t, err)
		status, err := app.Status(ctx, storecenter.ConnectionStatusInput{OrganizationID: "official-fixture", StoreID: current.ID(), Platform: storecenter.PlatformShein, ConnectionRef: current.ConnectionRef()})
		require.NoError(t, err)
		require.Equal(t, storecenter.ConnectionStatusConnected, status)
		_, err = app.Disconnect(ctx, storecenter.OfficialConnectionCommand{OrganizationID: "official-fixture", StoreID: current.ID(), AttemptID: uuid.NewString(), ExpectedStoreVersion: current.Version()})
		require.NoError(t, err)
		_, err = app.Complete(ctx, complete)
		require.ErrorIs(t, err, storecenter.ErrNotFound)
		require.Error(t, records.Exec(`UPDATE workbench_store_merchant_bindings SET open_key_id='other'`).Error)
		require.Error(t, records.Exec(`DELETE FROM workbench_store_connection_attempts`).Error)
	})
	t.Run("weakened-store-constraint", func(t *testing.T) {
		require.NoError(t, recordsOwner.Exec(`ALTER TABLE workbench_stores DROP CONSTRAINT store_service_period`).Error)
		require.NoError(t, recordsOwner.Exec(`ALTER TABLE workbench_stores ADD CONSTRAINT store_service_period CHECK (TRUE)`).Error)
		require.Error(t, storecenter.VerifyCurrentSchema(ctx, records), "a constraint name alone is not a service contract")
	})

}

type postgresConnectionAccess struct{}

func (postgresConnectionAccess) AuthorizeStoreMember(_ context.Context, org string) (storecenter.StoreMemberAccess, error) {
	return storecenter.StoreMemberAccess{OrganizationID: org, ActorID: "synthetic-operator", MemberID: "synthetic-membership", CanWrite: true}, nil
}

type postgresConnectionProvider struct{}

func (postgresConnectionProvider) Application() storecenter.OfficialApplication {
	return storecenter.OfficialApplication{AppID: "synthetic-app", Version: "config-v1", CallbackURL: "https://localhost/callback"}
}
func (postgresConnectionProvider) AuthorizationURL(state string) (string, error) {
	return "https://openapi-sem.sheincorp.com/#/empower?state=" + state, nil
}
func (postgresConnectionProvider) Exchange(context.Context, string, string) (storecenter.OfficialMerchantCredential, error) {
	return storecenter.OfficialMerchantCredential{AppID: "synthetic-app", OpenKeyID: "synthetic-open-key", SecretKey: "synthetic-merchant-secret", SupplierID: "123"}, nil
}
func (postgresConnectionProvider) QueryStore(context.Context, storecenter.OfficialMerchantCredential) (storecenter.OfficialStoreInformation, error) {
	return storecenter.OfficialStoreInformation{}, nil
}
