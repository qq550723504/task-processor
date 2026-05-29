package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestWorkflowStandardFileDelegatesExecutionBranchesToPhaseSeams(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("workflow_standard.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_standard.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"buildStandardWorkflowCanonicalPhase(s).run(ctx, task, result, recorder, log)",
		"buildStandardWorkflowMediaPhase(s).run(ctx, task, result, canonicalProduct, recorder, log)",
		"buildStandardWorkflowAssetPhase(s).run(",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("workflow_standard.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"s.sdsBaselineOrDefault().GetCachedBaseline(",
		"s.getCachedCanonicalProduct(",
		"s.productSvc.CreateGenerateTask(",
		"s.imageSvc.CreateProcessTask(",
		"s.syncSDSDesignFromRemote(",
		"s.assetGenerator.Plan(",
		"s.assetGenerator.Dispatch(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("workflow_standard.go should not contain %q", needle)
		}
	}
}

func TestWorkflowPlatformAdaptationFileDelegatesFinalizationToPhaseSeam(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("workflow_platform_adaptation.go")
	if err != nil {
		t.Fatalf("ReadFile(workflow_platform_adaptation.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		"buildPlatformFinalizePhase(s).run(",
		"applyStandardProductSnapshot(final, snapshot)",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_adaptation.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"s.applyDefaultSheinPricing(",
		"applySDSOfficialImagesToShein(",
		"applySheinInspectionReviewToSummary(",
		"attachPlatformImageBundles(",
		"s.assetGenerator.Dispatch(",
		"decorateListingKitResultGeneration(",
		"buildPlatformPostprocessPhase(",
		"buildPlatformAssetDispatchPhase(",
		"buildPlatformSummaryPhase(",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("workflow_platform_adaptation.go should not contain %q", needle)
		}
	}
}
