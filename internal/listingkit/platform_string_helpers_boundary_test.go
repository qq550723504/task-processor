package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformStringAdaptersHaveDedicatedFile(t *testing.T) {
	source, err := os.ReadFile("platform_string_helpers.go")
	if err != nil {
		t.Fatalf("read platform_string_helpers.go: %v", err)
	}
	content := string(source)
	for _, signature := range []string{"func firstNonEmpty(", "func uniqueStrings("} {
		if !strings.Contains(content, signature) {
			t.Fatalf("platform_string_helpers.go should own %s", signature)
		}
	}

	helperSource, err := os.ReadFile("platform_helpers.go")
	if err != nil {
		t.Fatalf("read platform_helpers.go: %v", err)
	}
	if strings.Contains(string(helperSource), "func firstNonEmpty(") || strings.Contains(string(helperSource), "func uniqueStrings(") {
		t.Fatal("platform_helpers.go should not own string adapters")
	}
}
