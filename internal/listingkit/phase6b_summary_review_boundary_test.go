package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestWorkflowPlatformSummaryPhaseFileOwnsCompletionNotReviewCompatibility(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("workflow_platform_summary_phase.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_summary_phase.go) error = %v", err)
	}
	content := string(src)

	finalizeCall := "newWorkflowRecorder(final).FinalizeSummary()"
	syncPreviewCall := "syncAssetRenderPreviews(final)"

	for _, needle := range []string{
		finalizeCall,
		syncPreviewCall,
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_summary_phase.go should contain %q", needle)
		}
	}

	if strings.Index(content, finalizeCall) > strings.Index(content, syncPreviewCall) {
		t.Fatalf("workflow_platform_summary_phase.go should finalize summary before syncing asset render previews")
	}

	for _, needle := range []string{
		"addSheinReviewWorkflowIssues(",
		"applySheinInspectionReviewToSummary(",
		"applySheinVariantCoverageReviewToSummary(",
		"buildPlatformReviewPhase().run(",
		"applySheinVariantImageCoverageGuard(",
		"buildPlatformAssetDispatchPhase(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_summary_phase.go should not contain %q", needle)
		}
	}
}
