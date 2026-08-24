package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformHelpersDoNotKeepUnusedCloneMap(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("platform_helpers.go")
	if err != nil {
		t.Fatalf("read platform_helpers.go: %v", err)
	}
	if strings.Contains(string(source), "func cloneMap(") {
		t.Fatal("platform_helpers.go should not keep the unused cloneMap duplicate")
	}
}
