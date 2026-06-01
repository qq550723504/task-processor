package sdslogin

import (
	"testing"

	"task-processor/internal/core/config"
)

func TestBuildHandlerReturnsNilWithoutConfig(t *testing.T) {
	result, err := BuildHandler(nil)
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}
	if result != nil {
		t.Fatalf("BuildHandler() = %v, want nil", result)
	}
}

func TestBuildHandlerBuildsServiceAndHandler(t *testing.T) {
	result, err := BuildHandler(&config.Config{})
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}
	if result == nil {
		t.Fatal("BuildHandler() returned nil result")
	}
	if result.Service == nil {
		t.Fatal("BuildHandler() returned nil service")
	}
	if result.Handler == nil {
		t.Fatal("BuildHandler() returned nil handler")
	}
}
