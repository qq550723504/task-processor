package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestGenerationNavigationStopOnNotModifiedUsesDomainDirectly(t *testing.T) {
	source, err := os.ReadFile("generation_navigation_target_dispatch_support.go")
	if err != nil {
		t.Fatalf("read generation_navigation_target_dispatch_support.go: %v", err)
	}
	content := string(source)
	if strings.Contains(content, "func buildGenerationNavigationDispatchStopOnNotModified(") {
		t.Fatal("generation_navigation_target_dispatch_support.go should not keep the forwarding wrapper")
	}
	if !strings.Contains(content, "listinggeneration.NavigationDispatchStopOnNotModified(") {
		t.Fatal("generation_navigation_target_dispatch_support.go should call the generation domain directly")
	}
}
