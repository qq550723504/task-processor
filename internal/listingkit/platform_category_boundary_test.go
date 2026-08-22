package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformMappersUseCommonLastCategory(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"temu_mapper.go", "walmart_mapper.go"} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(source)
		if !strings.Contains(content, "common.LastCategory(") {
			t.Fatalf("%s should use common.LastCategory", path)
		}
		if strings.Contains(content, "lastCategory(") {
			t.Fatalf("%s should not use the ListingKit duplicate lastCategory helper", path)
		}
	}

	helperSource, err := os.ReadFile("platform_helpers.go")
	if err != nil {
		t.Fatalf("read platform_helpers.go: %v", err)
	}
	if strings.Contains(string(helperSource), "func lastCategory(") {
		t.Fatal("platform_helpers.go should not duplicate common category selection")
	}
}
