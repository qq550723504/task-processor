package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformFirstNonEmptyDelegatesToCommon(t *testing.T) {
	source, err := os.ReadFile("platform_helpers.go")
	if err != nil {
		t.Fatalf("read platform_helpers.go: %v", err)
	}
	content := string(source)
	start := strings.Index(content, "func firstNonEmpty(")
	if start < 0 {
		t.Fatal("platform_helpers.go should keep a compatibility adapter for firstNonEmpty")
	}
	body := content[start:]
	if !strings.Contains(body, "common.FirstNonEmpty(values...)") {
		t.Fatal("firstNonEmpty should delegate to common.FirstNonEmpty")
	}
	if strings.Contains(body, "strings.TrimSpace(value)") {
		t.Fatal("firstNonEmpty should not duplicate common fallback logic")
	}
}
