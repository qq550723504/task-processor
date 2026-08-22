package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformMappersUseCommonAttributeFlattening(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"temu_mapper.go", "walmart_mapper.go"} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(source)
		if !strings.Contains(content, "common.FlattenAttributes(") {
			t.Fatalf("%s should use common.FlattenAttributes", path)
		}
		if strings.Contains(content, "flattenAttributes(") {
			t.Fatalf("%s should not use the ListingKit duplicate flattenAttributes helper", path)
		}
	}

	helperSource, err := os.ReadFile("platform_helpers.go")
	if err != nil {
		t.Fatalf("read platform_helpers.go: %v", err)
	}
	if strings.Contains(string(helperSource), "func flattenAttributes(") {
		t.Fatal("platform_helpers.go should not duplicate common attribute flattening")
	}
}
