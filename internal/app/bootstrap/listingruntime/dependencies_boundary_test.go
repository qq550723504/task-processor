package listingruntime

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestDependenciesDoNotImportLegacyCrawlerAmazon(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "dependencies.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse listing runtime dependencies imports: %v", err)
	}
	for _, imported := range file.Imports {
		importPath, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatalf("unquote import %s: %v", imported.Path.Value, err)
		}
		if importPath == "task-processor/internal/crawler/amazon" {
			t.Fatalf("dependencies.go imports legacy crawler directly: %s", importPath)
		}
	}
}

func TestDependenciesExposeListingRuntimeHealthValidatorInsteadOfSharedResources(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("dependencies.go")
	if err != nil {
		t.Fatalf("read dependencies.go: %v", err)
	}

	for _, marker := range []string{
		"func (d dependencies) SharedResources()",
		"sharedResources func() *bootstrapresources.SharedResources",
		"onSharedResources func(*bootstrapresources.SharedResources)",
		"onSharedResources(resources)",
	} {
		if strings.Contains(string(content), marker) {
			t.Fatalf("dependencies.go mentions %q; expose the listing runtime health validator as a narrow bootstrap dependency instead of leaking SharedResources", marker)
		}
	}
	if !strings.Contains(string(content), "func (d dependencies) ListingRuntimeHealthValidator()") {
		t.Fatalf("dependencies.go should expose ListingRuntimeHealthValidator as the narrow listing runtime health dependency")
	}
	if !strings.Contains(string(content), "onListingRuntimeHealthValidator func(bootstrapresources.ListingRuntimeHealthValidator)") {
		t.Fatalf("dependencies.go should pass ListingRuntimeHealthValidator through the resource-build callback instead of passing SharedResources")
	}
}

func TestListingRuntimeDoesNotImportBootstrapResources(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "..\\..\\runtime\\listing\\runtime.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse listing runtime imports: %v", err)
	}
	for _, imported := range file.Imports {
		importPath, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatalf("unquote import %s: %v", imported.Path.Value, err)
		}
		if importPath == "task-processor/internal/app/bootstrap/resources" {
			t.Fatalf("runtime.go imports bootstrap resources; consume the narrow ListingRuntimeHealthValidator dependency instead")
		}
	}
}
