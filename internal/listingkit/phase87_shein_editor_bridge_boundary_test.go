package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinEditorBridgeCallsMarketplaceWorkspaceDirectly(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "workspace/shein/editor_bridge.go")

	src, err := os.ReadFile("shein_workspace_editor_bridge.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_workspace_editor_bridge.go) error = %v", err)
	}
	content := string(src)
	if !strings.Contains(content, `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`) {
		t.Fatal("shein_workspace_editor_bridge.go should call marketplace SHEIN workspace directly")
	}
	if strings.Contains(content, `task-processor/internal/listingkit/workspace/shein`) {
		t.Fatal("shein_workspace_editor_bridge.go should not call ListingKit SHEIN workspace bridge")
	}

	for _, forbidden := range []string{
		"func buildSheinCategoryRecommendationMeta(",
		"func buildSheinAttributeRecommendationMeta(",
		"func buildSheinSaleRecommendationMeta(",
		"func buildSheinAttributeSuggestions(",
		"func buildSheinSaleCandidateSuggestions(",
		"func buildSheinCategoryEffects(",
		"func buildSheinAttributeEffects(",
		"func buildSheinSaleAttributeEffects(",
		"func buildSheinEditorProgress(",
		"func buildSheinEditorDirtyHints(",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("shein_workspace_editor_bridge.go should not keep unused editor wrapper %q", forbidden)
		}
	}
}
