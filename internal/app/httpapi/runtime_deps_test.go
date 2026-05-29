package httpapi

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"

	appbootstrap "task-processor/internal/app/bootstrap"
	"task-processor/internal/infra/clients/management"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
	"task-processor/internal/sdslogin"
	sdsloginbootstrap "task-processor/internal/sdslogin/bootstrap"
)

func TestRuntimeDepsManagementClientReturnsSharedClient(t *testing.T) {
	client := management.NewClientManager(nil)
	deps := &runtimeDeps{
		shared: &sharedRuntimeDeps{
			sharedResources: &appbootstrap.SharedResources{
				ManagementClient: client,
			},
		},
	}

	if got := deps.managementClient(); got != client {
		t.Fatalf("management client = %p, want %p", got, client)
	}
}

func TestRuntimeDepsManagementClientHandlesNilDeps(t *testing.T) {
	var deps *runtimeDeps
	if deps.managementClient() != nil {
		t.Fatal("expected nil management client for nil deps")
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
	t.Setenv("TASK_PROCESSOR_MANAGEMENT_CLIENT_SECRET", "test-secret")
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")

	deps, err := buildRuntimeDeps(logger, "../../../config/config-test.yaml")
	if err != nil {
		t.Fatalf("buildRuntimeDeps() error = %v", err)
	}

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
	deps := &runtimeDeps{
		shared:   &sharedRuntimeDeps{},
		features: &featureRuntimeState{sdsLoginStatusProvider: statusProvider},
	}

	input := newListingKitRuntimeBuildInput(logger, deps)

	if input.Runtime.SDSLoginStatusProvider != nil {
		t.Fatal("expected legacy runtime SDS login status provider to remain unset")
	}
	if input.Runtime.Support.SDSLoginStatusProvider != statusProvider {
		t.Fatal("expected SDS login status provider to be routed through runtime support")
	}
}

func TestRuntimeDepsAttachBuiltFeatureModules(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	t.Setenv("TASK_PROCESSOR_MANAGEMENT_CLIENT_SECRET", "test-secret")
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")

	deps, err := buildRuntimeDeps(logger, "../../../config/config-test.yaml")
	if err != nil {
		t.Fatalf("buildRuntimeDeps() error = %v", err)
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

	for i := len(deps.shared.closers) - 1; i >= 0; i-- {
		if deps.shared.closers[i] == nil {
			continue
		}
		if err := deps.shared.closers[i](); err != nil {
			t.Fatalf("closer[%d]() error = %v", i, err)
		}
	}
}

type stubStatusProvider func(context.Context) (*sdslogin.Status, error)

func (f stubStatusProvider) Status(ctx context.Context) (*sdslogin.Status, error) {
	return f(ctx)
}
