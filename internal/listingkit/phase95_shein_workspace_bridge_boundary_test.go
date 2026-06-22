package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinWorkspaceBridgePackageRemoved(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "workspace/shein/doc.go")
	assertFileAbsent(t, "workspace/shein/types_bridge.go")

	src, err := os.ReadFile("shein_workspace_types_bridge.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_workspace_types_bridge.go) error = %v", err)
	}
	content := string(src)
	for _, want := range []string{
		`sheinworkspace "task-processor/internal/marketplace/shein/workspace"`,
		`sheinpub "task-processor/internal/publishing/shein"`,
		"type SheinPackage = sheinpub.Package",
		"type SheinEditorContext = sheinworkspace.EditorContext",
		"return sheinworkspace.BuildEditorContext(pkg)",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("shein_workspace_types_bridge.go should contain %q", want)
		}
	}
	if strings.Contains(content, `task-processor/internal/listingkit/workspace/shein`) {
		t.Fatal("shein_workspace_types_bridge.go should not import ListingKit SHEIN workspace bridge")
	}
}
