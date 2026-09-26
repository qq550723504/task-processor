package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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
	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	a1688 "task-processor/internal/integration/acquisition/a1688"
	acquisitionstore "task-processor/internal/integration/persistence/product/acquisition"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/workbenchcontext"
)

type acquisitionHTTPTransport func(*http.Request) (*http.Response, error)

func (f acquisitionHTTPTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type acquisitionHTTPFixture struct {
	db, owner      *gorm.DB
	grants         *titleGrants
	provider       sourcing.PublicAcquirer
	fetches        atomic.Int32
	requestTimeout time.Duration
}

func newAcquisitionHTTPFixture(t *testing.T) *acquisitionHTTPFixture {
	t.Helper()
	dsn := os.Getenv("ISSUE398_TEST_DSN")
	if dsn == "" {
		t.Skip("result=SKIP: requires task-owned ISSUE398_TEST_DSN")
	}
	require.Contains(t, dsn, "host=127.0.0.1")
	require.Contains(t, dsn, "user=issue398_owner")
	gormConfig := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	root, err := gorm.Open(postgres.Open(dsn), gormConfig)
	require.NoError(t, err)
	rootPool, err := root.DB()
	require.NoError(t, err)
	rootPool.SetMaxOpenConns(2)
	name := "issue398_http_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE DATABASE "+name).Error)
	owner, err := gorm.Open(postgres.Open(dsn+" dbname="+name), gormConfig)
	require.NoError(t, err)
	ownerPool, err := owner.DB()
	require.NoError(t, err)
	ownerPool.SetMaxOpenConns(2)
	t.Cleanup(func() {
		require.NoError(t, ownerPool.Close())
		require.NoError(t, root.Exec("DROP DATABASE "+name+" WITH (FORCE)").Error)
		require.NoError(t, rootPool.Close())
	})
	require.NoError(t, productsourcing.InstallAcquisitionSchema(owner))
	require.NoError(t, owner.Exec(`DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='source_acquisition_runtime') THEN CREATE ROLE source_acquisition_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS; END IF; END $$`).Error)
	sum := sha256.Sum256([]byte(dsn))
	password := hex.EncodeToString(sum[:])
	require.NoError(t, owner.Exec("ALTER ROLE source_acquisition_runtime PASSWORD '"+password+"'").Error)
	require.NoError(t, acquisitionstore.GrantRuntimePermissions(context.Background(), owner))
	db, err := gorm.Open(postgres.Open(dsn+" dbname="+name+" user=source_acquisition_runtime password="+password), gormConfig)
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(8)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	f := &acquisitionHTTPFixture{db: db, owner: owner, grants: &titleGrants{}}
	body, err := os.ReadFile("../../integration/acquisition/a1688/testdata/public-product.html")
	require.NoError(t, err)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.fetches.Add(1)
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" || r.URL.RawQuery != "" {
			t.Error("provider received identity or tracking data")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(body)
	}))
	t.Cleanup(provider.Close)
	target, err := url.Parse(provider.URL)
	require.NoError(t, err)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	t.Cleanup(transport.CloseIdleConnections)
	f.provider = a1688.NewWithTransport(acquisitionHTTPTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://detail.1688.com/offer/981645030344.html" {
			t.Error("noncanonical provider request")
		}
		copy := r.Clone(r.Context())
		u := *r.URL
		u.Scheme = target.Scheme
		u.Host = target.Host
		copy.URL = &u
		return transport.RoundTrip(copy)
	}))
	return f
}

func (f *acquisitionHTTPFixture) server(t *testing.T) *httptest.Server {
	t.Helper()
	deps := newRouteAuthDependencies()
	deps.workbenchVerifier = titleVerifier{}
	deps.organizationResolver = workbenchcontext.NewResolver(f.grants, "project", "v1", nil)
	factories := currentApplicationFactories{
		buildWorkbench: func(*config.Config, *logrus.Logger) (workbenchContextBuildResult, error) {
			return workbenchContextBuildResult{module: currentApplicationTestModule{name: "workbench", routes: currentWorkbenchApplicationRoutes[:4]}, authDependencies: &deps}, nil
		},
		buildSourceAccount: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
			return currentApplicationTestModule{name: "source", routes: currentWorkbenchApplicationRoutes[5:]}, nil
		},
		buildCommercial: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
			return currentApplicationTestModule{name: "commercial", routes: currentWorkbenchApplicationRoutes[4:5]}, nil
		},
		buildAcquisition: func(auth *authz.ListingKitAuthorizer, d routeAuthDependencies) (kernelmodule.Module, error) {
			module, err := buildProductAcquisitionModule(context.Background(), f.db, d, auth, f.provider)
			if err == nil && f.requestTimeout > 0 {
				controlled := module.(productAcquisitionModule)
				for i := range controlled.routes {
					controlled.routes[i].RequestTimeout = f.requestTimeout
				}
				return controlled, nil
			}
			return module, err
		},
	}
	server, err := buildCurrentApplication(context.Background(), &gorm.DB{}, &gorm.DB{}, currentApplicationTestConfig(), logrus.New(), factories)
	require.NoError(t, err)
	result := httptest.NewServer(server.Handler)
	t.Cleanup(result.Close)
	return result
}

func acquisitionHTTPRequest(server *httptest.Server, method, path, actor, org, key, body string) (int, []byte, error) {
	r, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	if actor != "" {
		r.Header.Set("Authorization", "Bearer "+actor)
	}
	if org != "" {
		r.Header.Set("X-Requested-Organization-ID", org)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	r.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(r)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	return response.StatusCode, raw, err
}

func acquisitionHTTPCall(t *testing.T, server *httptest.Server, method, path, actor, org, key, body string, status int) acquisitionResultDTO {
	t.Helper()
	code, raw, err := acquisitionHTTPRequest(server, method, path, actor, org, key, body)
	require.NoError(t, err)
	require.Equal(t, status, code, string(raw))
	var result acquisitionResultDTO
	if status == 200 {
		require.NoError(t, json.Unmarshal(raw, &result))
	}
	return result
}

func TestAcquisitionMountedCurrentApplicationFullChainAndLiveAuthorization(t *testing.T) {
	f := newAcquisitionHTTPFixture(t)
	server := f.server(t)
	body := `{"source":"981645030344"}`
	acquisitionHTTPCall(t, server, "POST", productAcquisitionBase, "", "B", uuid.NewString(), body, 401)
	acquisitionHTTPCall(t, server, "POST", productAcquisitionBase, "bad", "B", uuid.NewString(), body, 401)
	acquisitionHTTPCall(t, server, "POST", productAcquisitionBase, "viewer", "B", uuid.NewString(), body, 403)
	require.Zero(t, f.fetches.Load())
	key := uuid.NewString()
	first := acquisitionHTTPCall(t, server, "POST", productAcquisitionBase, "operator", "B", key, body, 200)
	require.Equal(t, "1", first.CatalogVersion)
	require.Equal(t, "crawler:1688:981645030344", first.ProductKey)
	require.NotEmpty(t, first.MissingFacts)
	replay := acquisitionHTTPCall(t, server, "POST", productAcquisitionBase, "operator", "B", key, `{"source":"http://detail.1688.com/offer/981645030344.html?discard=1"}`, 200)
	require.True(t, replay.Replayed)
	require.Equal(t, first.PublicationID, replay.PublicationID)
	require.EqualValues(t, 1, f.fetches.Load())
	acquisitionHTTPCall(t, server, "POST", productAcquisitionBase, "operator", "B", key, `{"source":"12345"}`, 409)
	acquisitionHTTPCall(t, server, "GET", productAcquisitionBase+"/"+first.OperationID, "operator", "A", "", "", 404)
	acquisitionHTTPCall(t, server, "GET", productAcquisitionBase+"/"+first.OperationID, "operator2", "B", "", "", 404)
	var home, effective int64
	require.NoError(t, f.owner.Raw("SELECT count(*) FROM product_snapshot_versions WHERE tenant_id='A'").Scan(&home).Error)
	require.NoError(t, f.owner.Raw("SELECT count(*) FROM product_snapshot_versions WHERE tenant_id='B'").Scan(&effective).Error)
	require.Zero(t, home)
	require.EqualValues(t, 1, effective)
	require.Zero(t, f.grants.cached.Load(), "acquisition never substitutes cached roles")
	f.grants.failed.Store(true)
	acquisitionHTTPCall(t, server, "POST", productAcquisitionBase, "operator", "B", uuid.NewString(), body, 503)
	f.grants.failed.Store(false)
	f.grants.revoked.Store(true)
	acquisitionHTTPCall(t, server, "POST", productAcquisitionBase, "operator", "B", key, body, 403)
	acquisitionHTTPCall(t, server, "POST", productAcquisitionBase+"/verify", "operator", "B", key, body, 403)
	acquisitionHTTPCall(t, server, "GET", productAcquisitionBase+"/"+first.OperationID, "operator", "B", "", "", 403)
	require.EqualValues(t, 1, f.fetches.Load())
}

func TestAcquisitionMountedHTTPResponseLossAndApplicationRestart(t *testing.T) {
	f := newAcquisitionHTTPFixture(t)
	normal := f.server(t)
	lost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := httptest.NewRecorder()
		normal.Config.Handler.ServeHTTP(response, r)
		if response.Code != 200 {
			w.WriteHeader(response.Code)
			_, _ = w.Write(response.Body.Bytes())
			return
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	t.Cleanup(lost.Close)
	key := uuid.NewString()
	body := `{"source":"981645030344"}`
	_, _, err := acquisitionHTTPRequest(lost, "POST", productAcquisitionBase, "operator", "B", key, body)
	require.Error(t, err)
	rebuilt := f.server(t)
	verified := acquisitionHTTPCall(t, rebuilt, "POST", productAcquisitionBase+"/verify", "operator", "B", key, body, 200)
	require.Equal(t, "published", verified.Outcome)
	require.Equal(t, "1", verified.CatalogVersion)
	require.EqualValues(t, 1, f.fetches.Load())
	var versions, receipts int64
	require.NoError(t, f.owner.Table("product_snapshot_versions").Count(&versions).Error)
	require.NoError(t, f.owner.Table("product_source_publication_receipts").Count(&receipts).Error)
	require.EqualValues(t, 1, versions)
	require.Equal(t, versions, receipts)
}

type acquisitionHTTPProviderFunc func(context.Context, sourcing.AcquisitionSource) (sourcing.AcquisitionEvidence, error)

func (f acquisitionHTTPProviderFunc) Acquire(ctx context.Context, source sourcing.AcquisitionSource) (sourcing.AcquisitionEvidence, error) {
	return f(ctx, source)
}

func TestAcquisitionMountedCancellationDeadlineAndPostCaptureRevocation(t *testing.T) {
	for _, mode := range []string{"cancel", "deadline", "revoke-after-capture"} {
		t.Run(mode, func(t *testing.T) {
			f := newAcquisitionHTTPFixture(t)
			entered, finished := make(chan struct{}), make(chan struct{})
			original := f.provider
			f.provider = acquisitionHTTPProviderFunc(func(ctx context.Context, source sourcing.AcquisitionSource) (sourcing.AcquisitionEvidence, error) {
				close(entered)
				defer close(finished)
				if mode == "revoke-after-capture" {
					e, err := original.Acquire(ctx, source)
					f.grants.revoked.Store(true)
					return e, err
				}
				<-ctx.Done()
				return sourcing.AcquisitionEvidence{}, ctx.Err()
			})
			if mode == "deadline" {
				f.requestTimeout = 100 * time.Millisecond
			}
			server := f.server(t)
			key := uuid.NewString()
			body := `{"source":"981645030344"}`
			if mode == "cancel" {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				request, err := http.NewRequestWithContext(ctx, "POST", server.URL+productAcquisitionBase, strings.NewReader(body))
				require.NoError(t, err)
				request.Header.Set("Authorization", "Bearer operator")
				request.Header.Set("X-Requested-Organization-ID", "B")
				request.Header.Set("Idempotency-Key", key)
				request.Header.Set("Content-Type", "application/json")
				result := make(chan error, 1)
				go func() {
					response, e := server.Client().Do(request)
					if response != nil {
						response.Body.Close()
					}
					result <- e
				}()
				select {
				case <-entered:
				case <-time.After(3 * time.Second):
					t.Fatal("provider not entered")
				}
				cancel()
				require.Error(t, <-result)
			} else {
				status := 403
				if mode == "deadline" {
					status = 504
				}
				acquisitionHTTPCall(t, server, "POST", productAcquisitionBase, "operator", "B", key, body, status)
			}
			select {
			case <-finished:
			case <-time.After(3 * time.Second):
				t.Fatal("provider did not stop")
			}
			var versions, publications int64
			require.NoError(t, f.owner.Table("product_snapshot_versions").Count(&versions).Error)
			require.NoError(t, f.owner.Table("product_source_publications").Count(&publications).Error)
			require.Zero(t, versions)
			require.Zero(t, publications)
		})
	}
}
