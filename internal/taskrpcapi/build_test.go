package taskrpcapi

import (
	"testing"
)

func TestBuildHandlerReturnsNilWithoutLocalStatusProvider(t *testing.T) {
	handler, err := BuildHandler(nil)
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}
	if handler != nil {
		t.Fatalf("BuildHandler() = %v, want nil", handler)
	}
}

func TestBuildHandlerBuildsFromLocalStatusProvider(t *testing.T) {
	handler, err := BuildHandler(func() map[string]any {
		return map[string]any{"status": "ok"}
	})
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}
	if handler == nil {
		t.Fatal("BuildHandler() returned nil handler")
	}
}

func TestBuildModuleBuildsHandlerAndModule(t *testing.T) {
	result, err := BuildModule(func() map[string]any {
		return map[string]any{"status": "ok"}
	})
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
