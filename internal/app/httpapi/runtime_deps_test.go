package httpapi

import (
	"testing"

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
