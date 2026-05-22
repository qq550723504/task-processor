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
