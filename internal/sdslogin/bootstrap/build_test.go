package bootstrap

import (
	"testing"

	"task-processor/internal/core/config"
)

func TestBuildHandlerReturnsNilWithoutConfig(t *testing.T) {
	t.Parallel()

	result, err := BuildHandler(nil)
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}
	if result != nil {
		t.Fatalf("BuildHandler() = %#v, want nil", result)
	}
}

func TestBuildHandlerReturnsHandlerAndStatusProvider(t *testing.T) {
	t.Parallel()

	result, err := BuildHandler(&config.Config{})
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}
	if result == nil {
		t.Fatal("BuildHandler() returned nil result")
	}
	if result.Handler == nil {
		t.Fatal("BuildHandler() returned nil handler")
	}
	if result.Module == nil {
		t.Fatal("BuildHandler() returned nil module")
	}
	if result.StatusProvider == nil {
		t.Fatal("BuildHandler() returned nil status provider")
	}
}
