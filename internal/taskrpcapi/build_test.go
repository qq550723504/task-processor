package taskrpcapi

import (
	"testing"

	managementclient "task-processor/internal/infra/clients/management"
)

type stubClientProvider struct {
	client *managementclient.TaskRPCAPIClient
}

func (p stubClientProvider) GetTaskRPCClient() *managementclient.TaskRPCAPIClient { return p.client }

func TestBuildHandlerReturnsNilWithoutProvider(t *testing.T) {
	handler, err := BuildHandler(nil, nil)
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}
	if handler != nil {
		t.Fatalf("BuildHandler() = %v, want nil", handler)
	}
}

func TestBuildHandlerBuildsFromProvider(t *testing.T) {
	handler, err := BuildHandler(stubClientProvider{client: &managementclient.TaskRPCAPIClient{}}, nil)
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}
	if handler == nil {
		t.Fatal("BuildHandler() returned nil handler")
	}
}

func TestBuildModuleBuildsHandlerAndModule(t *testing.T) {
	result, err := BuildModule(stubClientProvider{client: &managementclient.TaskRPCAPIClient{}}, nil)
	if err != nil {
		t.Fatalf("BuildModule() error = %v", err)
	}
	if result == nil {
		t.Fatal("BuildModule() returned nil result")
	}
	if result.Handler == nil {
		t.Fatal("BuildModule() returned nil handler")
	}
	if result.Module == nil {
		t.Fatal("BuildModule() returned nil module")
	}
}
