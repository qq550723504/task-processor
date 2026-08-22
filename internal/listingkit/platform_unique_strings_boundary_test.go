package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformStringDeduplicationDelegatesToCommon(t *testing.T) {
	source, err := os.ReadFile("platform_helpers.go")
	if err != nil {
		t.Fatalf("read platform_helpers.go: %v", err)
	}
	content := string(source)
	start := strings.Index(content, "func uniqueStrings(")
	if start < 0 {
		t.Fatal("platform_helpers.go should keep a compatibility adapter for uniqueStrings")
	}
	body := content[start:]
	if !strings.Contains(body, "common.UniqueStrings(values)") {
		t.Fatal("uniqueStrings should delegate to common.UniqueStrings")
	}
	if strings.Contains(body, "seen := map[string]struct{}") {
		t.Fatal("uniqueStrings should not duplicate common deduplication logic")
	}
}
