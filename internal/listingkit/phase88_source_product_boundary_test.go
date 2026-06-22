package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinSourceProductBoundary(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "preview_builder_shein_source_product.go", "buildSheinSourceProductSummary")
	callNames := readNamedFunctionCallNames(t, "preview_builder_shein_source_product.go", "buildSheinSourceProductSummary")
	fileSource, err := os.ReadFile("preview_builder_shein_source_product.go")
	if err != nil {
		t.Fatalf("ReadFile(preview_builder_shein_source_product.go) error = %v", err)
	}

	assertSourceContainsAll(t, source, []string{
		"return sheinworkspace.BuildSourceProductSummary(product)",
	})
	if !strings.Contains(string(fileSource), `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`) {
		t.Fatal("preview_builder_shein_source_product.go should call marketplace SHEIN workspace directly")
	}
	assertSourceExcludesAll(t, source, []string{
		"canonical.Attributes",
		"canonical.Specifications",
		"summary.ImageURLs = uniqueStrings(summary.ImageURLs)",
	})
	assertFunctionCallsContainAll(t, callNames, []string{
		"BuildSourceProductSummary",
	})
}
