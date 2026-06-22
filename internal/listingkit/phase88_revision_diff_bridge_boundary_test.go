package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinRevisionDiffBridgeCallsMarketplaceWorkspaceDirectly(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "workspace/shein/revision_diff_bridge.go")

	cases := []struct {
		path string
		want string
	}{
		{
			path: "revision_model.go",
			want: `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`,
		},
		{
			path: "task_revision_service.go",
			want: `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`,
		},
		{
			path: "revision_workspace_bridge.go",
			want: `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			src, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", tc.path, err)
			}
			if !strings.Contains(string(src), tc.want) {
				t.Fatalf("%s should call marketplace SHEIN workspace diff directly", tc.path)
			}
		})
	}

	restoreSource := readNamedFunctionSource(t, "revision_workspace_bridge.go", "buildRevisionRestorePreviewFromDetail")
	assertSourceContainsAll(t, restoreSource, []string{
		"sheinworkspace.BuildRevisionDiffPreviewFromInput(detail.RestorePayload.Core.Draft)",
	})
}
