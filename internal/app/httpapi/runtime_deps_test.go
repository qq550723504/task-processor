package httpapi

import (
	"testing"

	"github.com/sirupsen/logrus"

	appbootstrap "task-processor/internal/app/bootstrap"
	"task-processor/internal/infra/clients/management"
)

func TestRuntimeDepsManagementClientReturnsSharedClient(t *testing.T) {
	client := management.NewClientManager(nil)
	deps := &runtimeDeps{
		shared: &appbootstrap.SharedResources{
			ManagementClient: client,
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
	deps := &runtimeDeps{}

	first := deps.ensureListingKitSupport()
	if first == nil {
		t.Fatal("expected listingkit support")
	}

	second := deps.ensureListingKitSupport()
	if second != first {
		t.Fatalf("listingkit support = %p, want %p", second, first)
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

	productModule, err := buildProductModule(logger, deps)
	if err != nil {
		t.Fatalf("buildProductModule() error = %v", err)
	}
	deps.attachProductModule(productModule)
	if deps.productService == nil {
		t.Fatal("expected product service to be attached")
	}

	imageModule, err := buildImageModule(logger, deps)
	if err != nil {
		t.Fatalf("buildImageModule() error = %v", err)
	}
	deps.attachImageModule(imageModule)
	if deps.imageService == nil {
		t.Fatal("expected image service to be attached")
	}

	for i := len(deps.closers) - 1; i >= 0; i-- {
		if deps.closers[i] == nil {
			continue
		}
		if err := deps.closers[i](); err != nil {
			t.Fatalf("closer[%d]() error = %v", i, err)
		}
	}
}
