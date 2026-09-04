package httpapi

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/core/config"
	"task-processor/internal/listingkit"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	platformfeatureflag "task-processor/internal/platform/featureflag"
	platformobservability "task-processor/internal/platform/observability"
	productasset "task-processor/internal/product/asset"
	sdsadapter "task-processor/internal/sds/adapter"
	sdsclient "task-processor/internal/sds/client"
	sdsdesign "task-processor/internal/sds/design"
	sdstemplate "task-processor/internal/sds/template"
	sdsusecase "task-processor/internal/sds/usecase"
	"task-processor/internal/sdslogin"
	sdsloginbootstrap "task-processor/internal/sdslogin/bootstrap"
	"task-processor/internal/sheinlogin"
)

func TestBuildRuntimeDepsRunsEnabledSchemaMigrationBeforeRepositoryConstruction(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")
	configPath := filepath.Join(t.TempDir(), "runtime.yaml")
	contents := []byte("featureFlags:\n  flags:\n    product-listing-runtime-auto-migrate: true\ndatabase:\n  host: 127.0.0.1\n  port: 1\n  user: test\n  password: test\n  database: test\n")
	if err := os.WriteFile(configPath, contents, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	wantErr := errors.New("migration stopped bootstrap")
	called := 0
	deps, err := buildRuntimeDepsWithSchemaMigrator(logrus.New(), configPath, func(context.Context, *config.DatabaseConfig, *logrus.Logger) error {
		called++
		return wantErr
	})
	if deps != nil {
		t.Fatal("buildRuntimeDepsWithSchemaMigrator() returned deps after migration failure")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("buildRuntimeDepsWithSchemaMigrator() error = %v, want migration error", err)
	}
	if strings.Contains(err.Error(), "database connection failed") {
		t.Fatalf("migration must run before repository construction, got %v", err)
	}
	if called != 1 {
		t.Fatalf("schema migrator calls = %d, want 1", called)
	}
}

func TestBuildRuntimeDepsPropagatesTypedProductCatalogDatabaseConstructionFailure(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")
	configPath := filepath.Join(t.TempDir(), "runtime.yaml")
	contents := []byte("featureFlags:\n  flags:\n    product-listing-runtime-auto-migrate: true\ndatabase:\n  host: database.internal\n  port: 5432\n  user: test\n  password: test\n  database: test\n")
	if err := os.WriteFile(configPath, contents, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	wantErr := errors.New("catalog database unavailable")
	called := false

	deps, err := buildRuntimeDepsWithBuilders(logrus.New(), configPath, runtimeDepsBuilders{
		buildTraceRuntime: func(context.Context, platformobservability.Config) (traceRuntime, error) {
			return &stubTraceRuntime{}, nil
		},
		buildFeatureFlagRuntime: func(context.Context, platformfeatureflag.Config) (featureFlagRuntime, error) {
			return &stubFeatureFlagRuntime{enabled: true}, nil
		},
		migrateSchema: func(context.Context, *config.DatabaseConfig, *logrus.Logger) error { return nil },
		buildProductCatalogDatabase: func(got *config.DatabaseConfig, _ *logrus.Logger) (*gorm.DB, func() error, error) {
			called = true
			if got == nil || got.Host != "database.internal" {
				t.Fatalf("database config = %+v", got)
			}
			return nil, nil, wantErr
		},
	})
	if deps != nil {
		t.Fatal("buildRuntimeDepsWithBuilders() returned deps after catalog database failure")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("buildRuntimeDepsWithBuilders() error = %v, want %v", err, wantErr)
	}
	if !called {
		t.Fatal("typed product catalog database builder was not called")
	}
}

func TestMigrateProductListingSchemaIfEnabledSkipsDisabledFlag(t *testing.T) {
	previousValue, hadPreviousValue := os.LookupEnv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE")
	if err := os.Unsetenv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE"); err != nil {
		t.Fatalf("unset legacy auto-migrate environment variable: %v", err)
	}
	t.Cleanup(func() {
		if hadPreviousValue {
			_ = os.Setenv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE", previousValue)
			return
		}
		_ = os.Unsetenv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE")
	})
	evaluator := &recordingBoolEvaluator{value: false}
	called := 0
	err := migrateProductListingSchemaIfEnabled(context.Background(), evaluator, &config.DatabaseConfig{}, logrus.New(), func(context.Context, *config.DatabaseConfig, *logrus.Logger) error {
		called++
		return nil
	})
	if err != nil {
		t.Fatalf("migrateProductListingSchemaIfEnabled() error = %v", err)
	}
	if called != 0 {
		t.Fatalf("schema migrator calls = %d, want 0", called)
	}
}

func TestBuildRuntimeDepsDisabledOpenFeatureFlagSkipsMigrationAndContinuesToRepositoryConstruction(t *testing.T) {
	previousValue, hadPreviousValue := os.LookupEnv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE")
	if err := os.Unsetenv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE"); err != nil {
		t.Fatalf("unset legacy auto-migrate environment variable: %v", err)
	}
	t.Cleanup(func() {
		if hadPreviousValue {
			_ = os.Setenv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE", previousValue)
			return
		}
		_ = os.Unsetenv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE")
	})
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")
	configPath := filepath.Join(t.TempDir(), "runtime.yaml")
	contents := []byte("featureFlags:\n  flags:\n    product-listing-runtime-auto-migrate: false\ndatabase:\n  host: 127.0.0.1\n  port: 1\n  user: test\n  password: test\n  database: test\n")
	if err := os.WriteFile(configPath, contents, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	called := 0
	deps, err := buildRuntimeDepsWithSchemaMigrator(logrus.New(), configPath, func(context.Context, *config.DatabaseConfig, *logrus.Logger) error {
		called++
		return nil
	})
	if deps != nil {
		t.Fatal("buildRuntimeDepsWithSchemaMigrator() unexpectedly constructed deps with unreachable database")
	}
	if err == nil || !strings.Contains(err.Error(), "database connection failed") {
		t.Fatalf("buildRuntimeDepsWithSchemaMigrator() error = %v, want repository construction database failure", err)
	}
	if called != 0 {
		t.Fatalf("schema migrator calls = %d, want 0", called)
	}
}

func TestRuntimeDepsListingKitSupportHandlesNilDeps(t *testing.T) {
	var deps *runtimeDeps
	if deps.ensureListingKitSupport() != nil {
		t.Fatal("expected nil listingkit support for nil deps")
	}
}

func TestRuntimeDepsListingKitSupportIsStable(t *testing.T) {
	deps := &runtimeDeps{features: &featureRuntimeState{}}

	first := deps.ensureListingKitSupport()
	if first == nil {
		t.Fatal("expected listingkit support")
	}

	second := deps.ensureListingKitSupport()
	if second != first {
		t.Fatalf("listingkit support = %p, want %p", second, first)
	}
}

func TestBuildRuntimeDepsInitializesSharedRuntimeWithoutFeatureState(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")

	deps, err := buildRuntimeDeps(logger, "../../../config/config-test.yaml")
	if err != nil {
		t.Fatalf("buildRuntimeDeps() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupOwnedRuntimeResources(false, deps.constructionClosers)
	})

	if deps.shared == nil {
		t.Fatal("expected shared runtime deps")
	}
	if deps.features == nil {
		t.Fatal("expected feature runtime state")
	}
	if deps.features.listingKitSupport != nil {
		t.Fatal("expected listingkit support to be lazy")
	}
}

func TestRuntimeDepsAttachBuiltFeatureModulesOnlyMutatesFeatureState(t *testing.T) {
	deps := &runtimeDeps{
		shared:   &sharedRuntimeDeps{},
		features: &featureRuntimeState{},
	}

	deps.attachSDSLoginResult(&sdsloginbootstrap.BuildResult{
		StatusProvider: stubStatusProvider(func(context.Context) (*sdslogin.Status, error) {
			return &sdslogin.Status{}, nil
		}),
	})

	if deps.shared.openaiMgr != nil {
		t.Fatal("expected shared runtime deps to remain unchanged")
	}
	if deps.features.sdsLoginStatusProvider == nil {
		t.Fatal("expected SDS login status provider to be attached to feature state")
	}
}

func TestNewListingKitRuntimeBuildInputRoutesSDSStatusProviderThroughRuntimeSupport(t *testing.T) {
	logger := logrus.New()
	statusProvider := stubCompositionSDSStatusProvider{}
	syncService := stubRuntimeDepsSDSSyncService{}
	approvedAssets := &stubRuntimeDepsApprovedAssetReader{}
	deps := &runtimeDeps{
		shared: &sharedRuntimeDeps{cfg: &config.Config{}},
		features: &featureRuntimeState{
			sdsLoginStatusProvider: statusProvider,
			listingKitSupport: &listingKitSupport{
				approvedAssetReader: approvedAssets,
				sheinCookieStore:    &sheinlogin.RedisStore{},
			},
		},
	}
	previousFactory := newSDSSyncServiceForHTTPAPI
	t.Cleanup(func() {
		newSDSSyncServiceForHTTPAPI = previousFactory
	})
	newSDSSyncServiceForHTTPAPI = func(sdsadapter.ApprovedAssetReader, *sdsclient.Config) (sdsusecase.Service, *sdsclient.AuthState, error) {
		return syncService, &sdsclient.AuthState{AccessToken: "test-token"}, nil
	}

	input, err := newListingKitRuntimeBuildInput(logger, deps, listingkithttpapi.BuildServiceRepositories{})
	if err != nil {
		t.Fatalf("newListingKitRuntimeBuildInput() error = %v", err)
	}
	if input.Runtime.Support.SDSSyncService != syncService {
		t.Fatal("expected SDS sync service to be routed through runtime support")
	}
	if input.Runtime.Support.SDSLoginStatusProvider != statusProvider {
		t.Fatal("expected SDS login status provider to be routed through runtime support")
	}
	if input.Runtime.Support.SDSBaselineRemoteProvider == nil {
		t.Fatal("expected SDS baseline remote provider to be routed through runtime support")
	}
	if input.Runtime.Support.Repositories.Core.ApprovedAsset != approvedAssets {
		t.Fatalf("ListingKit approved asset reader = %v, want shared reader %v", input.Runtime.Support.Repositories.Core.ApprovedAsset, approvedAssets)
	}
}

func TestEnsureListingKitSheinCookieStoreFailsWithoutRedisConfig(t *testing.T) {
	deps := &runtimeDeps{
		shared:   &sharedRuntimeDeps{cfg: &config.Config{}},
		features: &featureRuntimeState{},
	}

	store, err := ensureListingKitSheinCookieStore(logrus.New(), deps)

	if store != nil {
		t.Fatal("expected nil store without redis config")
	}
	if err == nil || !strings.Contains(err.Error(), "cookie store") {
		t.Fatalf("ensureListingKitSheinCookieStore() error = %v, want explicit cookie store error", err)
	}
	if len(deps.shared.closers) != 0 {
		t.Fatalf("closers = %d, want 0", len(deps.shared.closers))
	}
}

func TestEnsureListingKitSheinCookieStoreCachesStoreAndRegistersCloser(t *testing.T) {
	redisServer := miniredis.RunT(t)
	host, portText, err := net.SplitHostPort(redisServer.Addr())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("Atoi() error = %v", err)
	}

	deps := &runtimeDeps{
		shared: &sharedRuntimeDeps{
			cfg: &config.Config{
				Platforms: config.PlatformsConfig{
					Shein: config.PlatformConfig{
						CookieRedis: config.RedisConfig{Host: host, Port: port},
					},
				},
			},
		},
		features: &featureRuntimeState{},
	}

	logger := logrus.New()
	first, err := ensureListingKitSheinCookieStore(logger, deps)
	if err != nil {
		t.Fatalf("first ensureListingKitSheinCookieStore() error = %v", err)
	}
	if first == nil {
		t.Fatal("expected redis store")
	}
	second, err := ensureListingKitSheinCookieStore(logger, deps)
	if err != nil {
		t.Fatalf("second ensureListingKitSheinCookieStore() error = %v", err)
	}
	if second != first {
		t.Fatalf("cached store = %p, want %p", second, first)
	}
	if len(deps.shared.closers) != 1 {
		t.Fatalf("closers = %d, want 1", len(deps.shared.closers))
	}
	if deps.features.listingKitSupport == nil || deps.features.listingKitSupport.sheinCookieStore != first {
		t.Fatal("expected listingkit support to cache the redis store")
	}

	if err := deps.shared.closers[0](); err != nil {
		t.Fatalf("closer() error = %v", err)
	}
}

type stubStatusProvider func(context.Context) (*sdslogin.Status, error)

func (f stubStatusProvider) Status(ctx context.Context) (*sdslogin.Status, error) {
	return f(ctx)
}

var _ sdsusecase.Service = stubRuntimeDepsSDSSyncService{}
var _ listingkit.SDSBaselineRemoteProvider = stubRuntimeDepsSDSBaselineProvider{}

type stubRuntimeDepsSDSSyncService struct{}

func (stubRuntimeDepsSDSSyncService) SyncFromApprovedAssets(context.Context, sdsusecase.ApprovedAssetsInput) (*sdsadapter.SyncResult, error) {
	return nil, nil
}

type stubRuntimeDepsApprovedAssetReader struct{}

func (stubRuntimeDepsApprovedAssetReader) GetApprovedInventory(context.Context, productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
	return productasset.ApprovedAssetInventory{}, productasset.ErrApprovedAssetsNotReady
}

type stubRuntimeDepsSDSBaselineProvider struct{}

func (stubRuntimeDepsSDSBaselineProvider) GetProductDetail(context.Context, int64) (*sdstemplate.ProductDetail, error) {
	return nil, nil
}

func (stubRuntimeDepsSDSBaselineProvider) GetDesignProduct(context.Context, int64) (*sdsdesign.DesignProductPage, error) {
	return nil, nil
}

func (stubRuntimeDepsSDSBaselineProvider) GetPrototypeGroups(context.Context, int64) ([]sdsdesign.PrototypeGroup, error) {
	return nil, nil
}
