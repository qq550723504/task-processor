package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestWorkflowPlatformFinalizePhaseFileDelegatesToFinalizeSubSeams(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("workflow_platform_finalize_phase.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_finalize_phase.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"buildPlatformPostprocessPhase(p.service).run(",
		"buildPlatformAssetDispatchPhase(p.service).run(",
		"buildPlatformSummaryPhase()",
		"summaryPhase.prepareReview(final, snapshot)",
		"return summaryPhase.complete(task, final)",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_finalize_phase.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"sheinpub.OptimizePackageReviewContent(",
		"applySDSOfficialImagesToShein(",
		"applySheinInspectionReviewToSummary(",
		"s.assetGenerator.Dispatch(",
		"decorateListingKitResultGeneration(",
		"syncAssetRenderPreviews(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_finalize_phase.go should not contain %q", needle)
		}
	}
}
