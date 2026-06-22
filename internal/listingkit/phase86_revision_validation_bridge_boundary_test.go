package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinRevisionValidationBridgeCallsMarketplaceWorkspaceDirectly(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "workspace/shein/revision_validation_bridge.go")
	assertFileAbsent(t, "workspace/shein/revision_validation_payload_bridge.go")

	for _, path := range []string{
		"revision_validation.go",
		"revision_validate_model.go",
	} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", path, err)
			}
			content := string(src)
			if !strings.Contains(content, `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`) {
				t.Fatalf("%s should call marketplace SHEIN workspace directly", path)
			}
			if strings.Contains(content, `task-processor/internal/listingkit/workspace/shein`) {
				t.Fatalf("%s should not call ListingKit SHEIN workspace bridge", path)
			}
		})
	}
}
