package sheinmanaged

import (
	"testing"
)

func TestNewCategoryResolverReturnsSheinCategoryResolver(t *testing.T) {
	resolver := NewCategoryResolver(nil)
	if resolver == nil {
		t.Fatal("expected category resolver")
	}
}

func TestNewProductAPIBuilderReturnsSheinBuilder(t *testing.T) {
	builder := NewProductAPIBuilder(nil)
	if builder == nil {
		t.Fatal("expected product api builder")
	}
}
