package listingkit

import (
	"strings"
	"testing"
)

func TestWorkflowPlatformSummaryPhaseFileOwnsCompletionNotReviewCompatibility(t *testing.T) {
	t.Parallel()

	content := readExactMethodSource(t, "workflow_platform_finalize_phase.go", "func (p *platformSummaryPhase) run(")

	finalizeCall := "newWorkflowRecorder(final).FinalizeSummary()"
	syncPreviewCall := "syncAssetRenderPreviews(final)"

	for _, needle := range []string{
		finalizeCall,
		syncPreviewCall,
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("platform summary phase should contain %q", needle)
		}
	}

	if orderedNeedlesOutOfSequence(content, finalizeCall, syncPreviewCall) {
		t.Fatalf("platform summary phase should finalize summary before syncing asset render previews")
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
			t.Fatalf("platform summary phase should not contain %q", needle)
		}
	}
}

func TestWorkflowPlatformReviewPhaseOwnsReviewCompatibility(t *testing.T) {
	t.Parallel()

	content := readExactMethodSource(t, "workflow_platform_finalize_phase.go", "func (p *platformReviewPhase) run(")

	for _, needle := range []string{
		"newWorkflowRecorder(final).Start(\"shein_review\", \"\")",
		"applySheinInspectionReviewToSummary(final)",
		"applySheinVariantCoverageReviewToSummary(final)",
		"addSheinReviewWorkflowIssues(final)",
		"sheinReviewStage.Complete()",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("platform review phase should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"sheinpub.OptimizePackageReviewContent(",
		"applySDSOfficialImagesToShein(",
		"applySheinVariantImageCoverageGuard(",
		"buildPlatformAssetDispatchPhase(",
		"buildPlatformSummaryPhase(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("platform review phase should not contain %q", needle)
		}
	}
}

func orderedNeedlesOutOfSequence(source string, first string, second string) bool {
	firstIndex := strings.Index(source, first)
	secondIndex := strings.Index(source, second)
	return firstIndex < 0 || secondIndex < 0 || firstIndex > secondIndex
}
