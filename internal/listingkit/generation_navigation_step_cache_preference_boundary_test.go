package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestGenerationNavigationStepCachePreferenceUsesDomainDirectly(t *testing.T) {
	source, err := os.ReadFile("generation_navigation_target_dispatch_support.go")
	if err != nil {
		t.Fatalf("read generation_navigation_target_dispatch_support.go: %v", err)
	}
	content := string(source)
	if strings.Contains(content, "func buildGenerationNavigationDispatchStepCachePreference(") {
		t.Fatal("generation_navigation_target_dispatch_support.go should not keep the forwarding wrapper")
	}
	if !strings.Contains(content, "listinggeneration.NavigationDispatchStepCachePreference(") {
		t.Fatal("generation_navigation_target_dispatch_support.go should call the generation domain directly")
	}
}
