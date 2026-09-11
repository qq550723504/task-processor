//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/app/runtime/currentapplication"
	registrySchema "task-processor/internal/app/schema/sourceaccountregistry"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	sourceaccountstore "task-processor/internal/integration/persistence/sourceaccountregistry"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingsubscription"
)

type startupPermissionTraceKey struct{}

// pgx's actual BeginTx query is paused after the actual permission query has
// completed. This tests driver-boundary cancellation, not a fake application or
// a claimed server-side SQL lock. Production has no test hook or bypass.
type startupSchemaTrace struct {
	permissions atomic.Bool
	blocked     atomic.Bool
	entered     chan context.Context
	release     chan struct{}
}

func (trace *startupSchemaTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(data.SQL, "SELECT current_user,") {
		return context.WithValue(ctx, startupPermissionTraceKey{}, true)
	}
	if strings.HasPrefix(data.SQL, "begin") && trace.permissions.Load() && trace.blocked.CompareAndSwap(false, true) {
		trace.entered <- ctx
		select {
		case <-ctx.Done():
		case <-trace.release:
		}
	}
	return ctx
}

func (trace *startupSchemaTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if ctx.Value(startupPermissionTraceKey{}) == true && data.Err == nil {
		trace.permissions.Store(true)
	}
}

func TestCurrentApplicationStartupSchemaContextPostgres(t *testing.T) {
	gin.SetMode(gin.TestMode)
	owner, connection := commercialPostgres(t)
	ctx := context.Background()
	require.NoError(t, listingsubscription.AutoMigrateRepository(owner))
	require.NoError(t, registrySchema.Migrate(ctx, owner))
	require.NoError(t, owner.Exec(`REVOKE CREATE ON SCHEMA public FROM PUBLIC`).Error)
	for _, role := range []string{"source_account_runtime", "commercial_reader"} {
		require.NoError(t, owner.Exec(`CREATE ROLE `+role+` LOGIN PASSWORD 'synthetic-startup-password'; GRANT CONNECT ON DATABASE issue347 TO `+role+`; GRANT USAGE ON SCHEMA public TO `+role).Error)
		for table, privileges := range run1AllowedPrivileges[role] {
			require.NoError(t, owner.Exec(`GRANT `+strings.Join(privileges, ",")+` ON public.`+table+` TO `+role).Error)
		}
	}
	require.NoError(t, owner.Exec(`ALTER ROLE commercial_reader SET default_transaction_read_only=on`).Error)
	require.NoError(t, owner.Exec(`INSERT INTO public.source_account_resources(organization_id,id,platform,display_name,management_status,connection_status,version,created_by,updated_by,created_at,updated_at) VALUES ('org-B','01991e24-1009-7009-8009-000000000009','1688','retained','disabled','pending_connection',1,'fixture','fixture',now(),now())`).Error)
	var tables []string
	require.NoError(t, owner.Raw(`SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename`).Scan(&tables).Error)
	before := run1PermissionFacts(t, owner, tables)
	open := func(role string, trace pgx.QueryTracer) *gorm.DB {
		configuration, err := pgx.ParseConfig(fmt.Sprintf("host=127.0.0.1 port=%d dbname=issue347 user=%s password=synthetic-startup-password sslmode=disable", connection.Port, role))
		require.NoError(t, err)
		configuration.Tracer = trace
		pool := stdlib.OpenDB(*configuration)
		pool.SetMaxOpenConns(1)
		t.Cleanup(func() { require.NoError(t, pool.Close()) })
		db, err := gorm.Open(postgres.New(postgres.Config{Conn: pool}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.True(t, err == nil, "owned runtime pool failed to open")
		return db
	}
	for _, mode := range []string{"parent_cancel", "remaining_deadline"} {
		t.Run(mode, func(t *testing.T) {
			trace := &startupSchemaTrace{entered: make(chan context.Context, 1), release: make(chan struct{}, 1)}
			defer close(trace.release)
			source, commercial := open("source_account_runtime", trace), open("commercial_reader", nil)
			parent, cancel := context.WithCancel(ctx)
			if mode == "remaining_deadline" {
				cancel()
				parent, cancel = context.WithTimeout(ctx, time.Second)
			}
			defer cancel()
			var logs bytes.Buffer
			log := logrus.New()
			log.SetOutput(&logs)
			var listenCalls atomic.Int32
			closed := make(map[*gorm.DB]int)
			cfg := &currentapplication.Config{SchemaVersion: 1, Listen: currentapplication.ListenConfig{Host: "127.0.0.1", Port: 18443}, Identity: currentapplication.IdentityConfig{IssuerURL: "http://127.0.0.1:18080", AuthorizationAPIURL: "http://127.0.0.1:18080", ClientID: "run1", ClientSecret: "synthetic-identity", ProjectID: "run1"}}
			cfg.SourceAccountDatabase = currentapplication.DatabaseConfig{Host: connection.Host, Port: connection.Port, User: "source_account_runtime", Password: "synthetic-startup-password", Database: connection.Database, MaxConnections: 1}
			cfg.CommercialDatabase = cfg.SourceAccountDatabase
			cfg.CommercialDatabase.User = "commercial_reader"
			done := make(chan error, 1)
			go func() {
				done <- currentapplication.Run(parent, cfg, log, currentapplication.Dependencies{
					IdentityPreflight: func(context.Context, currentapplication.IdentityConfig) error { return nil },
					OpenSourceAccount: func(context.Context, currentapplication.DatabaseConfig) (*gorm.DB, error) { return source, nil },
					OpenCommercial:    func(context.Context, currentapplication.DatabaseConfig) (*gorm.DB, error) { return commercial, nil },
					NewApplication:    NewCurrentApplication,
					Listen: func(string, string) (net.Listener, error) {
						listenCalls.Add(1)
						return nil, fmt.Errorf("unexpected listen")
					},
					CloseDatabase: func(db *gorm.DB) error {
						closed[db]++
						pool, err := db.DB()
						if err != nil {
							return err
						}
						return pool.Close()
					},
				})
			}()
			select {
			case <-trace.entered:
				require.True(t, trace.permissions.Load(), "real permission query must precede stalled schema BeginTx")
			case err := <-done:
				t.Fatalf("startup ended before schema boundary: %v", err)
			case <-time.After(5 * time.Second):
				cancel()
				t.Fatal("schema boundary was not reached")
			}
			started := time.Now()
			if mode == "parent_cancel" {
				cancel()
			}
			var result error
			select {
			case result = <-done:
			case <-time.After(1500 * time.Millisecond):
				// Release the test gate even on RED so the real runtime can close
				// both pools; no abandoned goroutine or ten-second fixed sleep.
				trace.release <- struct{}{}
				result = <-done
				t.Error("schema verification ignored parent cancellation/deadline")
			}
			require.Error(t, result)
			require.ErrorIs(t, result, parent.Err())
			require.NotContains(t, result.Error()+logs.String(), "synthetic-startup-password")
			require.Zero(t, listenCalls.Load())
			require.Equal(t, map[*gorm.DB]int{source: 1, commercial: 1}, closed)
			for _, db := range []*gorm.DB{source, commercial} {
				pool, err := db.DB()
				require.NoError(t, err)
				require.Error(t, pool.PingContext(ctx))
			}
			t.Logf("schema boundary elapsed=%s, permission query completed, zero listen, both owned pools closed once", time.Since(started))
		})
	}
	t.Run("startup_context_not_retained_by_request_pool", func(t *testing.T) {
		db := open("source_account_runtime", nil)
		startup, cancel := context.WithCancel(ctx)
		defer cancel()
		module, err := defaultCurrentApplicationFactories(startup).buildSourceAccount(db, authz.DefaultListingKitAuthorizer())
		require.NoError(t, err)
		cancel()
		registry := kernelmodule.NewRegistry()
		require.NoError(t, module.Register(registry))
		router := gin.New()
		for _, route := range registry.Routes() {
			router.Handle(route.Method, route.Path, route.Handler)
		}
		identity := authidentity.AuthenticatedIdentity{UserID: "fixture", HomeOrganizationID: "home-A", TenantID: "org-B", EffectiveOrganizationID: "org-B", Roles: []string{"listingkit_operator"}, TokenExpiresAt: time.Now().Add(time.Hour)}
		request := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/source-accounts", nil).WithContext(authidentity.WithAuthenticatedIdentity(ctx, identity))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, 200, response.Code, response.Body.String())
		require.Contains(t, response.Body.String(), "01991e24-1009-7009-8009-000000000009")
	})
	t.Run("nil_and_already_canceled_constructor_context", func(t *testing.T) {
		db := open("source_account_runtime", nil)
		repository, err := sourceaccountstore.NewRepository(nil, db)
		require.Error(t, err)
		require.Nil(t, repository)
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		repository, err = sourceaccountstore.NewRepository(canceled, db)
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, repository)
	})
	t.Run("long_parent_keeps_ten_second_schema_cap", func(t *testing.T) {
		trace := &startupSchemaTrace{entered: make(chan context.Context, 1), release: make(chan struct{}, 1)}
		defer close(trace.release)
		db := open("source_account_runtime", trace)
		parent, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		done := make(chan error, 1)
		go func() {
			_, err := defaultCurrentApplicationFactories(parent).buildSourceAccount(db, authz.DefaultListingKitAuthorizer())
			done <- err
		}()
		select {
		case schemaContext := <-trace.entered:
			deadline, ok := schemaContext.Deadline()
			require.True(t, ok)
			require.Positive(t, time.Until(deadline))
			require.LessOrEqual(t, time.Until(deadline), 10*time.Second)
			cancel()
			require.ErrorIs(t, <-done, context.Canceled)
		case err := <-done:
			t.Fatalf("schema boundary not reached: %v", err)
		case <-time.After(5 * time.Second):
			cancel()
			t.Fatal("schema boundary not reached")
		}
	})
	require.Equal(t, before, run1PermissionFacts(t, owner, tables))
}
