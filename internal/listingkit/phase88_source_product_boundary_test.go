package listingkit

import "testing"

func TestSheinSourceProductBoundary(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "preview_builder_shein_source_product.go", "buildSheinSourceProductSummary")
	callNames := readNamedFunctionCallNames(t, "preview_builder_shein_source_product.go", "buildSheinSourceProductSummary")

	assertSourceContainsAll(t, source, []string{
		"return sheinworkspace.BuildSourceProductSummary(product)",
	})
	assertSourceExcludesAll(t, source, []string{
		"canonical.Attributes",
		"canonical.Specifications",
		"summary.ImageURLs = uniqueStrings(summary.ImageURLs)",
	})
	assertFunctionCallsContainAll(t, callNames, []string{
		"BuildSourceProductSummary",
	})
}
