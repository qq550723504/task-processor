//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"task-processor/internal/app/runtime/currentapplication"
	registrySchema "task-processor/internal/app/schema/sourceaccountregistry"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingsubscription"
	platformdatabase "task-processor/internal/platform/database"
)

func TestCommercialReadVerifiedPublicSchemaPostgres(t *testing.T) {
	gin.SetMode(gin.TestMode)
	owner, connection := commercialPostgres(t) // Never accepts an external DSN.
	ctx := context.Background()
	require.NoError(t, listingsubscription.AutoMigrateRepository(owner))
	require.NoError(t, registrySchema.Migrate(ctx, owner))
	require.NoError(t, owner.Exec(`REVOKE CREATE ON SCHEMA public FROM PUBLIC`).Error)
	require.NoError(t, owner.Exec(`REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC`).Error)
	for _, role := range []string{"commercial_reader", "source_account_runtime"} {
		require.NoError(t, owner.Exec(`CREATE ROLE `+role+` LOGIN PASSWORD 'synthetic-run1-password'`).Error)
		require.NoError(t, owner.Exec(`GRANT CONNECT ON DATABASE issue347 TO `+role).Error)
		require.NoError(t, owner.Exec(`GRANT USAGE ON SCHEMA public TO `+role).Error)
		for table, privileges := range run1AllowedPrivileges[role] {
			require.NoError(t, owner.Exec(`GRANT `+strings.Join(privileges, ",")+` ON `+pgx.Identifier{"public", table}.Sanitize()+` TO `+role).Error)
		}
	}
	require.NoError(t, owner.Exec(`ALTER ROLE commercial_reader SET default_transaction_read_only=on`).Error)
	require.NoError(t, owner.Exec(`INSERT INTO public.saas_plans(code,name,active,created_at,updated_at) VALUES ('public-plan','Public plan',true,now(),now()),('shadow-plan','Public alternate plan',true,now(),now())`).Error)
	require.NoError(t, owner.Exec(`INSERT INTO public.saas_tenant_subscriptions(tenant_id,plan_code,status,created_at,updated_at) VALUES ('org-B','public-plan','active',now(),now())`).Error)
	require.NoError(t, owner.Exec(`INSERT INTO public.saas_tenant_entitlements(tenant_id,module_code,status,limits,created_at,updated_at) VALUES ('org-B','listingkit','active','{"listingkit_generations_succeeded":17}',now(),now())`).Error)
	require.NoError(t, owner.Exec(`INSERT INTO public.saas_usage_buckets(tenant_id,module_code,metric,period_key,committed,reserved,updated_at) VALUES ('org-B','listingkit','listingkit_generations_succeeded',?,7,2,now())`, time.Now().UTC().Format("2006-01")).Error)
	tables := []string{"saas_tenant_subscriptions", "saas_plans", "saas_tenant_entitlements", "saas_usage_buckets"}
	var publicTables []string
	require.NoError(t, owner.Raw(`SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename`).Scan(&publicTables).Error)
	beforePublic := run1PermissionFacts(t, owner, publicTables)
	open := func() *gorm.DB {
		db, err := platformdatabase.OpenExistingReadOnlyContext(ctx, &platformdatabase.Config{Host: connection.Host, Port: connection.Port, User: "commercial_reader", Password: "synthetic-run1-password", Database: connection.Database, MaxConnections: 4, MaxIdleConnections: 4})
		require.True(t, err == nil, "runtime pool open failed")
		t.Cleanup(func() { require.NoError(t, platformdatabase.Close(db)) })
		return db
	}
	db := open()
	build := func(t *testing.T, pool *gorm.DB) http.Handler {
		t.Helper()
		module, err := buildCommercialReadModuleFromDatabase(ctx, pool, authz.DefaultListingKitAuthorizer())
		require.NoError(t, err, "actual public preflight must pass before returning the commercial module")
		registry := kernelmodule.NewRegistry()
		require.NoError(t, module.Register(registry))
		require.Len(t, registry.Routes(), 1)
		router := gin.New()
		router.GET(registry.Routes()[0].Path, registry.Routes()[0].Handler)
		return router
	}
	read := func(handler http.Handler, requestContext context.Context) *httptest.ResponseRecorder {
		identity := authidentity.AuthenticatedIdentity{UserID: "fixture", HomeOrganizationID: "home-A", TenantID: "org-B", EffectiveOrganizationID: "org-B", Roles: []string{"listingkit_operator"}, TokenExpiresAt: time.Now().Add(time.Hour)}
		request := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/commercial/overview", nil).WithContext(authidentity.WithAuthenticatedIdentity(requestContext, identity))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	assertPublic := func(t *testing.T, response *httptest.ResponseRecorder) {
		t.Helper()
		require.Equal(t, 200, response.Code, response.Body.String())
		var result listingsubscription.CommercialOverview
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
		require.NotNil(t, result.Subscription)
		require.Equal(t, "public-plan", result.Subscription.PlanCode)
		require.NotNil(t, result.Subscription.PlanName)
		require.Equal(t, "Public plan", *result.Subscription.PlanName)
		require.Len(t, result.Entitlements, 1)
		require.Len(t, result.Entitlements[0].Limits, 1)
		require.Equal(t, "17", result.Entitlements[0].Limits[0].RawValue)
		require.Equal(t, "7", *result.Usage[0].Committed)
		require.Equal(t, "2", *result.Usage[0].Reserved)
	}
	t.Run("default_public", func(t *testing.T) { assertPublic(t, read(build(t, db), ctx)) })
	// The test owner alone installs accessible shadows. The application has no
	// DDL, grants or repair authority, and retains the default "$user", public path.
	require.NoError(t, owner.Exec(`CREATE SCHEMA commercial_reader; GRANT USAGE ON SCHEMA commercial_reader TO commercial_reader`).Error)
	changes := []string{`UPDATE commercial_reader.saas_tenant_subscriptions SET plan_code='shadow-plan'`, `UPDATE commercial_reader.saas_plans SET name='Shadow plan'`, `UPDATE commercial_reader.saas_tenant_entitlements SET limits='{"listingkit_generations_succeeded":999}'`, `UPDATE commercial_reader.saas_usage_buckets SET committed=999,reserved=888`}
	for index, table := range tables {
		t.Run("shadow/"+table, func(t *testing.T) {
			target := pgx.Identifier{"commercial_reader", table}.Sanitize()
			require.NoError(t, owner.Exec(`CREATE TABLE `+target+` (LIKE public.`+table+` INCLUDING ALL); INSERT INTO `+target+` SELECT * FROM public.`+table).Error)
			require.NoError(t, owner.Exec(changes[index]).Error)
			require.NoError(t, owner.Exec(`GRANT SELECT ON `+target+` TO commercial_reader`).Error)
			// Keep only this shadow during its subtest, so every query is tested
			// independently even when an earlier query would return wrong facts.
			defer func() { require.NoError(t, owner.Exec(`ALTER TABLE `+target+` RENAME TO `+table+`_saved`).Error) }()
			// A fresh connection must resolve the newly installed shadow; reuse
			// of a prepared query bound before CREATE would mask this regression.
			assertPublic(t, read(build(t, open()), ctx))
		})
	}
	// Rename restoration also runs on RED: failed assertions must not conceal
	// subsequent independent table probes or cleanup.
	for _, table := range tables {
		if owner.Migrator().HasTable("commercial_reader." + table + "_saved") {
			require.NoError(t, owner.Exec(`ALTER TABLE commercial_reader.`+table+`_saved RENAME TO `+table).Error)
		}
	}
	snapshot := func() string {
		var values []string
		for _, table := range tables {
			var rows []string
			require.NoError(t, owner.Raw(`SELECT row_to_json(r)::text || ':' || xmin::text FROM `+pgx.Identifier{"commercial_reader", table}.Sanitize()+` r ORDER BY 1`).Scan(&rows).Error)
			values = append(values, strings.Join(rows, "|"))
		}
		return strings.Join(values, "\n")
	}
	beforeShadow := snapshot()
	db = open()
	handler := build(t, db)
	t.Run("all_shadows", func(t *testing.T) { assertPublic(t, read(handler, ctx)) })
	t.Run("four_actual_connections", func(t *testing.T) {
		lock := owner.Begin()
		require.NoError(t, lock.Error)
		defer lock.Rollback()
		// Lock both possible resolutions so RED also proves distinct backends.
		require.NoError(t, lock.Exec(`LOCK public.saas_tenant_subscriptions, commercial_reader.saas_tenant_subscriptions IN ACCESS EXCLUSIVE MODE`).Error)
		responses := make(chan *httptest.ResponseRecorder, 4)
		for range 4 {
			go func() { responses <- read(handler, ctx) }()
		}
		require.Eventually(t, func() bool {
			var count int64
			return owner.Raw(`SELECT count(DISTINCT pid) FROM pg_stat_activity WHERE usename='commercial_reader' AND wait_event_type='Lock'`).Scan(&count).Error == nil && count == 4
		}, 5*time.Second, 20*time.Millisecond)
		require.NoError(t, lock.Rollback().Error)
		for range 4 {
			assertPublic(t, <-responses)
		}
	})
	t.Run("new_pool_and_module", func(t *testing.T) { assertPublic(t, read(build(t, open()), ctx)) })
	t.Run("recreated_connections_existing_module", func(t *testing.T) {
		pool, err := db.DB()
		require.NoError(t, err)
		closed := pool.Stats().MaxLifetimeClosed
		pool.SetConnMaxLifetime(time.Nanosecond)
		defer pool.SetConnMaxLifetime(0)
		assertPublic(t, read(handler, ctx))
		require.Greater(t, pool.Stats().MaxLifetimeClosed, closed)
	})
	for _, table := range tables {
		for _, failure := range []string{"permission", "missing"} {
			t.Run(failure+"/"+table, func(t *testing.T) {
				if failure == "permission" {
					require.NoError(t, owner.Exec(`REVOKE SELECT ON public.`+table+` FROM commercial_reader`).Error)
					defer func() { require.NoError(t, owner.Exec(`GRANT SELECT ON public.`+table+` TO commercial_reader`).Error) }()
				} else {
					require.NoError(t, owner.Exec(`ALTER TABLE public.`+table+` RENAME TO `+table+`_unavailable`).Error)
					defer func() {
						require.NoError(t, owner.Exec(`ALTER TABLE public.`+table+`_unavailable RENAME TO `+table).Error)
					}()
				}
				module, err := buildCommercialReadModuleFromDatabase(ctx, db, authz.DefaultListingKitAuthorizer())
				require.Error(t, err)
				require.Nil(t, module, "must reject before serving, no shadow fallback")
				response := read(handler, ctx) // Previously built reader also fails closed.
				require.Equal(t, 503, response.Code)
				require.NotContains(t, response.Body.String(), "synthetic-run1-password")
				require.NotContains(t, response.Body.String(), "Shadow")
			})
		}
	}
	t.Run("transaction_and_cancellation", func(t *testing.T) {
		var mu sync.Mutex
		var settings []string
		require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:commercial_transaction", func(tx *gorm.DB) {
			if tx.Error != nil {
				return
			}
			var setting string
			err := tx.Statement.ConnPool.QueryRowContext(tx.Statement.Context, `SELECT current_setting('transaction_read_only') || '/' || current_setting('transaction_isolation') || '/' || current_setting('statement_timeout')`).Scan(&setting)
			tx.AddError(err)
			mu.Lock()
			settings = append(settings, setting)
			mu.Unlock()
		}))
		defer db.Callback().Query().Remove("test:commercial_transaction")
		assertPublic(t, read(handler, ctx))
		require.NotEmpty(t, settings)
		for _, setting := range settings {
			require.Equal(t, "on/repeatable read/10s", setting)
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		require.Equal(t, 504, read(handler, canceled).Code)
		lock := owner.Begin()
		require.NoError(t, lock.Error)
		defer lock.Rollback()
		require.NoError(t, lock.Exec(`LOCK public.saas_tenant_subscriptions IN ACCESS EXCLUSIVE MODE`).Error)
		deadline, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		started := time.Now()
		require.Equal(t, 504, read(handler, deadline).Code)
		require.Less(t, time.Since(started), 2*time.Second)
	})
	t.Run("normal_binary_restarts_with_shadows", func(t *testing.T) {
		config := currentapplication.DatabaseConfig{Host: connection.Host, Port: connection.Port, User: "commercial_reader", Password: "synthetic-run1-password", Database: connection.Database, MaxConnections: 4}
		source := config
		source.User = "source_account_runtime"
		run1PermissionBinary(t, owner, &currentapplication.Config{SchemaVersion: 1, Listen: currentapplication.ListenConfig{Host: "127.0.0.1", Port: 18443}, Identity: currentapplication.IdentityConfig{IssuerURL: "http://127.0.0.1:18080", AuthorizationAPIURL: "http://127.0.0.1:18080", ClientID: "run1", ClientSecret: "synthetic-identity", ProjectID: "run1"}, CommercialDatabase: config, SourceAccountDatabase: source})
	})
	require.Equal(t, beforePublic, run1PermissionFacts(t, owner, publicTables))
	require.Equal(t, beforeShadow, snapshot(), "reads or preflight changed shadow facts")
	if !t.Failed() {
		t.Log("PASS actual preflight -> module handler -> commercial service/repository: all four public owners, real pooled backends, recreated pool/module, binary restarts, fail-closed public absence, read-only/timeouts/cancel, public/shadow rows+xmin unchanged")
	}
}
