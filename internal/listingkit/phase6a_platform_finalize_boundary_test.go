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
		"buildPlatformReviewPhase().run(final, snapshot)",
		"buildPlatformAssetDispatchPhase(p.service).run(",
		"buildPlatformSummaryPhase().run(task, final)",
		"applySheinVariantImageCoverageGuard(final, task.Request, final.Shein)",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_finalize_phase.go should contain %q", needle)
		}
	}

	postprocessIndex := strings.Index(content, "buildPlatformPostprocessPhase(p.service).run(")
	reviewIndex := strings.Index(content, "buildPlatformReviewPhase().run(final, snapshot)")
	coverageGuardIndex := strings.Index(content, "applySheinVariantImageCoverageGuard(final, task.Request, final.Shein)")
	assetDispatchIndex := strings.Index(content, "buildPlatformAssetDispatchPhase(p.service).run(")
	summaryIndex := strings.Index(content, "buildPlatformSummaryPhase().run(task, final)")

	if !(postprocessIndex < reviewIndex &&
		reviewIndex < coverageGuardIndex &&
		coverageGuardIndex < assetDispatchIndex &&
		assetDispatchIndex < summaryIndex) {
		t.Fatalf("workflow_platform_finalize_phase.go should keep postprocess -> review prep -> coverage guard -> asset dispatch -> completion order")
	}

	for _, needle := range []string{
		"sheinpub.OptimizePackageReviewContent(",
		"applySDSOfficialImagesToShein(",
		"applySheinInspectionReviewToSummary(",
		"s.assetGenerator.Dispatch(",
		"decorateListingKitResultGeneration(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_finalize_phase.go should not contain %q", needle)
		}
	}
}

func TestWorkflowPlatformFinalizeCoverageGuardStaysInFinalizePhase(t *testing.T) {
	t.Parallel()

	const coverageGuardCall = "applySheinVariantImageCoverageGuard(final, task.Request, final.Shein)"

	for _, tc := range []struct {
		file        string
		shouldExist bool
	}{
		{file: "workflow_platform_finalize_phase.go", shouldExist: true},
		{file: "workflow_platform_postprocess_phase.go", shouldExist: false},
		{file: "workflow_platform_review_phase.go", shouldExist: false},
		{file: "workflow_platform_summary_phase.go", shouldExist: false},
	} {
		src, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", tc.file, err)
		}
		content := string(src)
		hasCall := strings.Contains(content, coverageGuardCall)
		if hasCall != tc.shouldExist {
			if tc.shouldExist {
				t.Fatalf("%s should contain %q", tc.file, coverageGuardCall)
			}
			t.Fatalf("%s should not contain %q", tc.file, coverageGuardCall)
		}
	}
}
