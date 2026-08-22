package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestTemuMapperUsesCommonFloatFormatting(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("temu_mapper.go")
	if err != nil {
		t.Fatalf("read temu_mapper.go: %v", err)
	}
	content := string(source)
	if !strings.Contains(content, "common.FormatFloat(") {
		t.Fatal("temu_mapper.go should use common.FormatFloat")
	}
	if strings.Contains(content, "formatFloat(") {
		t.Fatal("temu_mapper.go should not use the ListingKit duplicate formatFloat helper")
	}

	helperSource, err := os.ReadFile("platform_helpers.go")
	if err != nil {
		t.Fatalf("read platform_helpers.go: %v", err)
	}
	if strings.Contains(string(helperSource), "func formatFloat(") {
		t.Fatal("platform_helpers.go should not duplicate common float formatting")
	}
}
