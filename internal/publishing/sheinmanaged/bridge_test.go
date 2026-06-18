package sheinmanaged

import (
	"os"
	"path/filepath"
	"strings"
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

func TestSheinManagedBridgeKeepsAPIBuildersInDedicatedFile(t *testing.T) {
	bridgeSource := readSheinManagedTestFile(t, "bridge.go")
	if strings.Contains(bridgeSource, "type productAPIBuilder struct") ||
		strings.Contains(bridgeSource, "func NewProductAPIBuilder") ||
		strings.Contains(bridgeSource, "type imageAPIBuilder struct") ||
		strings.Contains(bridgeSource, "type translateAPIBuilder struct") {
		t.Fatal("bridge.go should keep resolver bridge wiring only; move API builders to api_builders.go")
	}

	builderSource := readSheinManagedTestFile(t, "api_builders.go")
	for _, marker := range []string{
		"func NewProductAPIBuilder",
		"func NewImageAPIBuilder",
		"func NewTranslateAPIBuilder",
	} {
		if !strings.Contains(builderSource, marker) {
			t.Fatalf("api_builders.go missing %s", marker)
		}
	}
}

func readSheinManagedTestFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(".", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
