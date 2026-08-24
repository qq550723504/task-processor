package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformMappersUseCommonBrandHelpers(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"temu_mapper.go", "walmart_mapper.go"} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(source)
		if !strings.Contains(content, "common.WithBrandHint(") {
			t.Fatalf("%s should use common.WithBrandHint", path)
		}
		if strings.Contains(content, "withBrandHint(") {
			t.Fatalf("%s should not use the ListingKit duplicate withBrandHint helper", path)
		}
	}

	walmartSource, err := os.ReadFile("walmart_mapper.go")
	if err != nil {
		t.Fatalf("read walmart_mapper.go: %v", err)
	}
	if !strings.Contains(string(walmartSource), "common.ResolveBrand(") {
		t.Fatal("walmart_mapper.go should use common.ResolveBrand")
	}
	if strings.Contains(string(walmartSource), "resolveBrand(") {
		t.Fatal("walmart_mapper.go should not use the ListingKit duplicate resolveBrand helper")
	}

	helperSource, err := os.ReadFile("platform_helpers.go")
	if err != nil {
		t.Fatalf("read platform_helpers.go: %v", err)
	}
	for _, signature := range []string{"func resolveBrand(", "func withBrandHint("} {
		if strings.Contains(string(helperSource), signature) {
			t.Fatalf("platform_helpers.go should not duplicate %s", signature)
		}
	}
}
