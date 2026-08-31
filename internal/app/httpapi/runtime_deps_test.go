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

	"task-processor/internal/core/config"
	"task-processor/internal/listingkit"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	"task-processor/internal/productimage"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
	sdsadapter "task-processor/internal/sds/adapter"
	sdsclient "task-processor/internal/sds/client"
	sdsdesign "task-processor/internal/sds/design"
	sdstemplate "task-processor/internal/sds/template"
	sdsusecase "task-processor/internal/sds/usecase"
	sdsworkflow "task-processor/internal/sds/workflow"
	"task-processor/internal/sdslogin"
	sdsloginbootstrap "task-processor/internal/sdslogin/bootstrap"
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

func TestMigrateProductListingSchemaIfEnabledSkipsDisabledFlag(t *testing.T) {
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
	if deps.features.productService != nil {
		t.Fatal("expected product service to be unset before feature attachment")
	}
	if deps.features.imageService != nil {
		t.Fatal("expected image service to be unset before feature attachment")
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
	deps := &runtimeDeps{
		shared: &sharedRuntimeDeps{},
		features: &featureRuntimeState{
			sdsLoginStatusProvider: statusProvider,
			imageService:           stubRuntimeDepsImageService{},
		},
	}
	previousFactory := newSDSSyncServiceForHTTPAPI
	t.Cleanup(func() {
		newSDSSyncServiceForHTTPAPI = previousFactory
	})
	newSDSSyncServiceForHTTPAPI = func(productimage.Service, *sdsclient.Config) (sdsusecase.Service, *sdsclient.AuthState, error) {
		return syncService, &sdsclient.AuthState{AccessToken: "test-token"}, nil
	}

	input := newListingKitRuntimeBuildInput(logger, deps)

	if input.Runtime.SDSSyncService != nil {
		t.Fatal("expected legacy runtime SDS sync service to remain unset")
	}
	if input.Runtime.SDSLoginStatusProvider != nil {
		t.Fatal("expected legacy runtime SDS login status provider to remain unset")
	}
	if input.Runtime.SDSBaselineRemoteProvider != nil {
		t.Fatal("expected legacy runtime SDS baseline remote provider to remain unset")
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
}

func TestEnsureListingKitSheinCookieStoreReturnsNilWithoutRedisConfig(t *testing.T) {
	deps := &runtimeDeps{
		shared:   &sharedRuntimeDeps{cfg: &config.Config{}},
		features: &featureRuntimeState{},
	}

	store := ensureListingKitSheinCookieStore(logrus.New(), deps)

	if store != nil {
		t.Fatal("expected nil store without redis config")
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
	first := ensureListingKitSheinCookieStore(logger, deps)
	if first == nil {
		t.Fatal("expected redis store")
	}
	second := ensureListingKitSheinCookieStore(logger, deps)
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

func TestRuntimeDepsAttachBuiltFeatureModules(t *testing.T) {
	runtimePaths := configureProductImageRuntimePaths(t)

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")

	deps, err := buildRuntimeDeps(logger, "../../../config/config-test.yaml")
	if err != nil {
		t.Fatalf("buildRuntimeDeps() error = %v", err)
	}
	if deps.shared.imageWorkDir != runtimePaths.workDir {
		t.Fatalf("image work dir = %q, want %q", deps.shared.imageWorkDir, runtimePaths.workDir)
	}
	if deps.shared.cfg.ProductImage.Publisher.OutputDir != runtimePaths.publisherOutputDir {
		t.Fatalf("publisher output dir = %q, want %q", deps.shared.cfg.ProductImage.Publisher.OutputDir, runtimePaths.publisherOutputDir)
	}

	productModule, err := productenrichhttpapi.BuildRuntimeModule(productenrichhttpapi.RuntimeBuildInput{
		Logger:        logger,
		Config:        deps.shared.cfg,
		LLMManager:    deps.shared.llmMgr,
		InputParser:   deps.shared.inputParser,
		Understanding: deps.shared.understanding,
	})
	if err != nil {
		t.Fatalf("BuildRuntimeModule() product error = %v", err)
	}
	deps.attachProductModule(productModule)
	if deps.features.productService == nil {
		t.Fatal("expected product service to be attached")
	}

	imageModule, err := productimagehttpapi.BuildRuntimeModule(productimagehttpapi.RuntimeBuildInput{
		Logger:        logger,
		Config:        deps.shared.cfg,
		LLMManager:    deps.shared.llmMgr,
		OpenAIManager: deps.shared.openaiMgr,
		InputParser:   deps.shared.inputParser,
		Understanding: deps.shared.understanding,
		ImageWorkDir:  deps.shared.imageWorkDir,
	})
	if err != nil {
		t.Fatalf("BuildRuntimeModule() image error = %v", err)
	}
	deps.attachImageModule(imageModule)
	if deps.features.imageService == nil {
		t.Fatal("expected image service to be attached")
	}
	assertRuntimeDirectory(t, runtimePaths.workDir)

	cleanupOwnedRuntimeResources(false, deps.constructionClosers)
}

type stubStatusProvider func(context.Context) (*sdslogin.Status, error)

func (f stubStatusProvider) Status(ctx context.Context) (*sdslogin.Status, error) {
	return f(ctx)
}

var _ productimage.Service = stubRuntimeDepsImageService{}
var _ sdsusecase.Service = stubRuntimeDepsSDSSyncService{}
var _ listingkit.SDSBaselineRemoteProvider = stubRuntimeDepsSDSBaselineProvider{}

type stubRuntimeDepsImageService struct{}

func (stubRuntimeDepsImageService) CreateProcessTask(context.Context, *productimage.ImageProcessRequest) (*productimage.Task, error) {
	return nil, nil
}

func (stubRuntimeDepsImageService) GetTaskResult(context.Context, string) (*productimage.TaskResult, error) {
	return nil, nil
}

func (stubRuntimeDepsImageService) ReviewTask(context.Context, string, *productimage.ReviewTaskRequest) (*productimage.TaskResult, error) {
	return nil, nil
}

func (stubRuntimeDepsImageService) ProcessImages(context.Context, *productimage.Task) (*productimage.ImageProcessResult, error) {
	return nil, nil
}

func (stubRuntimeDepsImageService) SetTaskSubmitter(productimage.TaskSubmitter) {}

type stubRuntimeDepsSDSSyncService struct{}

func (stubRuntimeDepsSDSSyncService) SyncFromRemoteImage(context.Context, sdsusecase.RemoteImageInput) (*sdsworkflow.SyncResult, error) {
	return nil, nil
}

func (stubRuntimeDepsSDSSyncService) SyncFromLocalFile(context.Context, sdsusecase.LocalFileInput) (*sdsworkflow.SyncResult, error) {
	return nil, nil
}

func (stubRuntimeDepsSDSSyncService) SyncFromImageResult(context.Context, sdsusecase.ImageResultInput) (*sdsadapter.SyncResult, error) {
	return nil, nil
}

func (stubRuntimeDepsSDSSyncService) SyncFromImageRequest(context.Context, sdsusecase.ImageRequestInput) (*sdsadapter.SyncResult, error) {
	return nil, nil
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
