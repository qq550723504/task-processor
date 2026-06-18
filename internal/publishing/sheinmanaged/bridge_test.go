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

func TestSheinManagedBridgeKeepsResolversInDedicatedFiles(t *testing.T) {
	bridgeSource := readSheinManagedTestFile(t, "bridge.go")
	for _, marker := range []string{
		"type categoryResolver struct",
		"func NewCategoryResolver",
		"type attributeResolver struct",
		"func NewAttributeResolver",
		"type saleAttributeResolver struct",
		"func NewSaleAttributeResolver",
	} {
		if strings.Contains(bridgeSource, marker) {
			t.Fatalf("bridge.go should keep package-level wiring only; move %s to a dedicated resolver file", marker)
		}
	}

	resolverFiles := map[string][]string{
		"category_resolver.go": {
			"type categoryResolver struct",
			"func NewCategoryResolver",
		},
		"attribute_resolver.go": {
			"type attributeResolver struct",
			"func NewAttributeResolver",
		},
		"sale_attribute_resolver.go": {
			"type saleAttributeResolver struct",
			"func NewSaleAttributeResolver",
		},
	}
	for fileName, markers := range resolverFiles {
		source := readSheinManagedTestFile(t, fileName)
		for _, marker := range markers {
			if !strings.Contains(source, marker) {
				t.Fatalf("%s missing %s", fileName, marker)
			}
		}
	}
}

func TestSheinManagedAttributeAPIConstructionStaysInDedicatedFactory(t *testing.T) {
	for _, fileName := range []string{
		"attribute_resolver.go",
		"sale_attribute_resolver.go",
	} {
		source := readSheinManagedTestFile(t, fileName)
		if strings.Contains(source, `"task-processor/internal/shein/api/attribute"`) {
			t.Fatalf("%s should use attribute_api_factory.go instead of importing the SHEIN attribute API directly", fileName)
		}
	}

	factorySource := readSheinManagedTestFile(t, "attribute_api_factory.go")
	for _, marker := range []string{
		`"task-processor/internal/shein/api/attribute"`,
		"func buildAttributeAPI",
	} {
		if !strings.Contains(factorySource, marker) {
			t.Fatalf("attribute_api_factory.go missing %s", marker)
		}
	}
}

func TestSheinManagedCategoryAPIConstructionStaysInDedicatedFactory(t *testing.T) {
	resolverSource := readSheinManagedTestFile(t, "category_resolver.go")
	if strings.Contains(resolverSource, `"task-processor/internal/shein/api/category"`) {
		t.Fatal("category_resolver.go should use category_api_factory.go instead of importing the SHEIN category API directly")
	}

	factorySource := readSheinManagedTestFile(t, "category_api_factory.go")
	for _, marker := range []string{
		`"task-processor/internal/shein/api/category"`,
		"func buildCategoryAPI",
	} {
		if !strings.Contains(factorySource, marker) {
			t.Fatalf("category_api_factory.go missing %s", marker)
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
