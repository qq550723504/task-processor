package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinSubmitReadinessSupportFilesOwnHelperFamilies(t *testing.T) {
	t.Parallel()

	homeSrc, err := os.ReadFile("shein_submit_readiness.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_readiness.go) error = %v", err)
	}
	homeContent := string(homeSrc)

	for _, needle := range []string{
		"func buildSheinSubmitReadinessWithPodForAction(pkg *SheinPackage, pod *PodExecutionSummary, action string) *SheinSubmitReadiness {",
		"func buildSheinSubmitReadinessGuidanceResolver(",
		"func shapeSheinSubmitReadinessSummary(",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("shein_submit_readiness.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func buildSheinReadinessGuidance(",
		"func sheinHasSubmitImage(pkg *SheinPackage) bool {",
		"func sheinFinalImagesReadyForAction(pkg *SheinPackage, action string) (bool, string) {",
		"func buildSheinSubmitReadinessChecks(",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("shein_submit_readiness.go should delegate helper seam %q", needle)
		}
	}

	guidanceSrc, err := os.ReadFile("shein_submit_readiness_guidance_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_readiness_guidance_support.go) error = %v", err)
	}
	guidanceContent := string(guidanceSrc)

	for _, needle := range []string{
		"func buildSheinReadinessReason(spec *sheinworkspace.ReadinessReasonSpec) *SheinReadinessReason {",
		"func buildSheinReadinessGuidance(pkg *SheinPackage, key string, fieldPaths []string, suggestedAction string, warningOnly bool) sheinReadinessGuidance {",
		"func cloneSheinRepairHints(items []SheinRepairHint) []SheinRepairHint {",
	} {
		if !strings.Contains(guidanceContent, needle) {
			t.Fatalf("shein_submit_readiness_guidance_support.go should contain %q", needle)
		}
	}

	statusSrc, err := os.ReadFile("shein_submit_readiness_status_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_readiness_status_support.go) error = %v", err)
	}
	statusContent := string(statusSrc)

	for _, needle := range []string{
		"func sheinHasAnySKU(pkg *SheinPackage) bool {",
		"func sheinFinalImagesReadyForAction(pkg *SheinPackage, action string) (bool, string) {",
		"func sheinHasSubmitImage(pkg *SheinPackage) bool {",
		"func sheinProductImageInfoHasImage(info *sheinproduct.ImageInfo) bool {",
	} {
		if !strings.Contains(statusContent, needle) {
			t.Fatalf("shein_submit_readiness_status_support.go should contain %q", needle)
		}
	}
	if strings.Contains(statusContent, "func sheinSourceFactsReady(") {
		t.Fatal("shein_submit_readiness_status_support.go should not keep a source-facts wrapper; call SHEIN workspace readiness from checks assembly")
	}

	checksSrc, err := os.ReadFile("shein_submit_readiness_checks_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_readiness_checks_support.go) error = %v", err)
	}
	checksContent := string(checksSrc)

	for _, needle := range []string{
		"func buildSheinSubmitReadinessChecks(pkg *SheinPackage, pod *PodExecutionSummary, action string, validation sheinBuildValidation) []sheinworkspace.ReadinessCheckSpec {",
		"func appendSheinPodReadinessChecks(",
		"func appendSheinTemplateReadinessChecks(",
		"func appendSheinPayloadReadinessChecks(",
		"func sheinSubmitReadinessFinalDraftReady(pkg *SheinPackage, action string) bool {",
		"func sheinSubmitReadinessFinalReviewMessage(action string) string {",
		"sourceFactsReady, sourceFactsMessage := sheinworkspace.SourceFactsReady(sourceMetadata)",
	} {
		if !strings.Contains(checksContent, needle) {
			t.Fatalf("shein_submit_readiness_checks_support.go should contain %q", needle)
		}
	}
}
