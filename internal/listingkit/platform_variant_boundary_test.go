package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformMappersUseCommonVariantBuilding(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"temu_mapper.go", "walmart_mapper.go"} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(source)
		if !strings.Contains(content, "common.BuildVariants(") {
			t.Fatalf("%s should use common.BuildVariants", path)
		}
		if strings.Contains(content, "buildPlatformVariants(") {
			t.Fatalf("%s should not use the ListingKit duplicate buildPlatformVariants helper", path)
		}
	}

	helperSource, err := os.ReadFile("platform_helpers.go")
	if err != nil {
		t.Fatalf("read platform_helpers.go: %v", err)
	}
	if strings.Contains(string(helperSource), "func buildPlatformVariants(") {
		t.Fatal("platform_helpers.go should not duplicate common variant building")
	}
}
