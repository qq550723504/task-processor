//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	podapp "task-processor/internal/app/pod"
	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authz"
	coreconfig "task-processor/internal/core/config"
	acquisitionstore "task-processor/internal/integration/persistence/product/acquisition"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	"task-processor/internal/integration/sds"
	"task-processor/internal/product/supplymarket"
	"task-processor/internal/workbenchcontext"
	"testing"
	"time"
)

type nativeMarketStorage struct{}

func (nativeMarketStorage) PutImmutable(context.Context, supplymarket.PrivateFile, []byte) error {
	return nil
}
func (nativeMarketStorage) Inspect(context.Context, supplymarket.PrivateFile) (supplymarket.PrivateObject, error) {
	return supplymarket.PrivateObject{}, nil
}
func (nativeMarketStorage) ReadBounded(context.Context, supplymarket.PrivateFile) ([]byte, error) {
	return nil, nil
}

type nativePODCredentials struct{}

func (nativePODCredentials) Current(context.Context) (sds.Credentials, error) {
	return sds.Credentials{}, nil
}

type nativePODStarter struct{}

func (nativePODStarter) Ensure(context.Context, podapp.Execution) error { return nil }

type nativePODWorker struct{}

func (nativePODWorker) Start() error { return nil }
func (nativePODWorker) Stop()        {}

func TestSupplyMarketNativeCompositionPostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, e := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("native_product"), tcpostgres.WithUsername("native_owner"), tcpostgres.WithPassword("isolated-owner"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, e)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, e := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, e)
	u, e := url.Parse(dsn)
	require.NoError(t, e)
	open := func(v *url.URL) *gorm.DB {
		db, e := gorm.Open(postgres.Open(v.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, e)
		pool, e := db.DB()
		require.NoError(t, e)
		pool.SetMaxOpenConns(4)
		t.Cleanup(func() { require.NoError(t, pool.Close()) })
		return db
	}
	owner := open(u)
	require.NoError(t, owner.Exec("CREATE ROLE source_acquisition_runtime LOGIN PASSWORD 'isolated-runtime'").Error)
	require.NoError(t, owner.Exec("CREATE ROLE supply_asset_runtime LOGIN PASSWORD 'isolated-runtime'").Error)
	port, e := strconv.Atoi(u.Port())
	require.NoError(t, e)
	raw, e := json.Marshal(map[string]any{"schemaVersion": 1, "collections": true, "supplyMarket": true, "pod": true, "database": map[string]any{"host": "127.0.0.1", "port": port, "user": "native_owner", "password": "isolated-owner", "database": "native_product"}})
	require.NoError(t, e)
	path := filepath.Join(t.TempDir(), "fresh-product.json")
	require.NoError(t, os.WriteFile(path, raw, 0600))
	require.NoError(t, productsourcing.InitializeAcquisitionDatabase(ctx, path, "native_product"))
	require.Error(t, productsourcing.InitializeAcquisitionDatabase(ctx, path, "native_product"), "existing data/schema is never repaired by startup or initializer")
	runtimeURL := *u
	runtimeURL.User = url.UserPassword(acquisitionstore.RuntimeRole, "isolated-runtime")
	product := open(&runtimeURL)
	cap := acquisitionstore.RuntimeCapabilities{Collections: true, SupplyMarket: true, POD: true}
	require.NoError(t, acquisitionstore.VerifyRuntimePermissions(ctx, product, cap))
	require.Error(t, acquisitionstore.VerifyRuntimePermissions(ctx, product, acquisitionstore.RuntimeCapabilities{Collections: true}))
	require.NoError(t, product.Exec("SELECT operation_id FROM product_pod_fences FOR UPDATE").Error, "the fence needs its row-lock privilege")
	require.NoError(t, owner.Exec("CREATE DATABASE native_assets OWNER native_owner").Error)
	assetURL := *u
	assetURL.Path = "/native_assets"
	assetOwner := open(&assetURL)
	require.NoError(t, assetstore.InstallSourceApprovalSchema(assetOwner))
	require.NoError(t, assetstore.GrantSourceRuntimePermissions(ctx, assetOwner))
	assetURL.User = url.UserPassword(assetstore.SourceRuntimeRole, "isolated-runtime")
	assets := open(&assetURL)
	cfg := &coreconfig.Config{}
	cfg.ListingKit.Zitadel.TenantDirectoryToken = "fixture"
	cfg.ListingKit.Zitadel.ProjectID = "fixture"
	cfg.ListingKit.Zitadel.AuthorizationAPIURL = "http://127.0.0.1:1"
	var worker podapp.Worker
	pod := &PODDependencies{AssetDB: assets, Credentials: nativePODCredentials{}, HTTP: &http.Client{Timeout: time.Second}, OSSHosts: []string{"fixture.oss-cn-hangzhou.aliyuncs.com"}, Starter: nativePODStarter{}, Worker: &worker, NewWorker: func(p *podapp.Processor) (podapp.Worker, error) {
		require.NotNil(t, p.Repository)
		require.NotNil(t, p.Kernel)
		return nativePODWorker{}, nil
	}}
	module, e := buildSupplyMarketModule(ctx, product, SupplyMarketDependencies{Storage: nativeMarketStorage{}}, pod, routeAuthDependencies{organizationResolver: &workbenchcontext.Resolver{}}, authz.DefaultListingKitAuthorizer(), cfg)
	require.NoError(t, e)
	require.Len(t, module.routes, 32)
	require.NotNil(t, module.worker)
	for _, r := range module.routes {
		require.NoError(t, validateMarketPODDescriptor(r))
	}
	require.NoError(t, owner.Exec("GRANT DELETE ON supply_market_records TO source_acquisition_runtime").Error)
	require.Error(t, acquisitionstore.VerifyRuntimePermissions(ctx, product, cap), "extra destructive permission refuses startup")
}
