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
		"return sheinpub.HasAnySubmitSKU(pkg)",
		"return sheinpub.FinalSubmitImagesReady(pkg, action)",
		"return sheinpub.HasSubmitImage(pkg)",
		"return sheinpub.ProductImageInfoHasImage(info)",
	} {
		if !strings.Contains(statusContent, needle) {
			t.Fatalf("shein_submit_readiness_status_support.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"for _, skc := range pkg.DraftPayload.SKCList",
		"parseMoney(",
		"strings.TrimSpace(image.ImageURL)",
	} {
		if strings.Contains(statusContent, needle) {
			t.Fatalf("shein_submit_readiness_status_support.go should delegate readiness status logic, found %q", needle)
		}
	}
	if strings.Contains(statusContent, "func sheinSourceFactsReady(") {
		t.Fatal("shein_submit_readiness_status_support.go should not keep a source-facts wrapper; call SHEIN workspace readiness from checks assembly")
	}

	buildValidationSrc, err := os.ReadFile("shein_build_validation.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_build_validation.go) error = %v", err)
	}
	buildValidationContent := string(buildValidationSrc)
	for _, needle := range []string{
		"func appendSheinBuildValidationChecks(",
		"return append(checks, sheinworkspace.BuildSubmitPayloadValidationReadinessChecks(sheinworkspace.SubmitPayloadValidationReadinessInput{",
	} {
		if !strings.Contains(buildValidationContent, needle) {
			t.Fatalf("shein_build_validation.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		`"发布载荷结构"`,
		`"shein.preview_product", "shein.request_draft.skc_list"`,
	} {
		if strings.Contains(buildValidationContent, needle) {
			t.Fatalf("shein_build_validation.go should delegate payload validation check construction, found %q", needle)
		}
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
		"return append(checks, sheinworkspace.BuildSubmitTemplateReadinessChecks(sheinworkspace.SubmitTemplateReadinessInput{",
		"return append(checks, sheinworkspace.BuildSubmitPayloadReadinessChecks(pkg, action)...)",
		"func sheinSubmitReadinessFinalDraftReady(pkg *SheinPackage, action string) bool {",
		"func sheinSubmitReadinessFinalReviewMessage(action string) string {",
		"return sheinpub.FinalReviewReady(pkg, action)",
		"return sheinpub.FinalReviewMessage(action)",
	} {
		if !strings.Contains(checksContent, needle) {
			t.Fatalf("shein_submit_readiness_checks_support.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		`"request_draft",`,
		`"preview_product",`,
		`"variant_image_coverage",`,
		`"pricing",`,
		`"category_review",`,
		`"attribute_review",`,
		`"sale_attributes",`,
	} {
		if strings.Contains(checksContent, needle) {
			t.Fatalf("shein_submit_readiness_checks_support.go should delegate check construction, found %q", needle)
		}
	}

	workspacePayloadSrc, err := os.ReadFile("../marketplace/shein/workspace/submit_payload_readiness_checks.go")
	if err != nil {
		t.Fatalf("ReadFile(../marketplace/shein/workspace/submit_payload_readiness_checks.go) error = %v", err)
	}
	workspacePayloadContent := string(workspacePayloadSrc)
	for _, needle := range []string{
		"func BuildSubmitPayloadReadinessChecks(pkg *sheinpub.Package, action string) []ReadinessCheckSpec {",
		"checks = append(checks, BuildManualNotesReadinessCheck(pkg))",
		"checks = append(checks, BuildSourceFactsReadinessCheck(pkg))",
	} {
		if !strings.Contains(workspacePayloadContent, needle) {
			t.Fatalf("workspace payload readiness checks should contain %q", needle)
		}
	}

	workspacePayloadValidationSrc, err := os.ReadFile("../marketplace/shein/workspace/submit_payload_validation_readiness_checks.go")
	if err != nil {
		t.Fatalf("ReadFile(../marketplace/shein/workspace/submit_payload_validation_readiness_checks.go) error = %v", err)
	}
	workspacePayloadValidationContent := string(workspacePayloadValidationSrc)
	for _, needle := range []string{
		"func BuildSubmitPayloadValidationReadinessChecks(input SubmitPayloadValidationReadinessInput) []ReadinessCheckSpec {",
		`"发布载荷结构"`,
		`"shein.preview_product", "shein.request_draft.skc_list"`,
	} {
		if !strings.Contains(workspacePayloadValidationContent, needle) {
			t.Fatalf("workspace payload validation readiness checks should contain %q", needle)
		}
	}

	workspaceTemplateSrc, err := os.ReadFile("../marketplace/shein/workspace/submit_template_readiness_checks.go")
	if err != nil {
		t.Fatalf("ReadFile(../marketplace/shein/workspace/submit_template_readiness_checks.go) error = %v", err)
	}
	workspaceTemplateContent := string(workspaceTemplateSrc)
	for _, needle := range []string{
		"func BuildSubmitTemplateReadinessChecks(input SubmitTemplateReadinessInput) []ReadinessCheckSpec {",
		`"category_review",`,
		`"attribute_review",`,
		`"sale_attributes",`,
	} {
		if !strings.Contains(workspaceTemplateContent, needle) {
			t.Fatalf("workspace template readiness checks should contain %q", needle)
		}
	}
}
