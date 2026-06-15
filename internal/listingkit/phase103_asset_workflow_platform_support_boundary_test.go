package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestAssetWorkflowPlatformSupportOwnsBundleAndPendingTaskHelpers(t *testing.T) {
	t.Parallel()

	rootSrc, err := os.ReadFile("asset_workflow.go")
	if err != nil {
		t.Fatalf("ReadFile(asset_workflow.go) error = %v", err)
	}
	rootContent := string(rootSrc)

	for _, needle := range []string{
		"func attachPlatformImageBundles(",
		"func platformGenerationTasks(",
		"func collectPlatformGenerationTasks(",
		"func assetbundleRequest(",
	} {
		if strings.Contains(rootContent, needle) {
			t.Fatalf("asset_workflow.go should delegate platform helper seam %q", needle)
		}
	}

	platformSrc, err := os.ReadFile("asset_workflow_platform_support.go")
	if err != nil {
		t.Fatalf("ReadFile(asset_workflow_platform_support.go) error = %v", err)
	}
	platformContent := string(platformSrc)

	for _, needle := range []string{
		"func attachPlatformImageBundles(",
		"func platformGenerationTasks(",
		"func collectPlatformGenerationTasks(",
		"func assetbundleRequest(",
	} {
		if !strings.Contains(platformContent, needle) {
			t.Fatalf("asset_workflow_platform_support.go should contain %q", needle)
		}
	}
}
