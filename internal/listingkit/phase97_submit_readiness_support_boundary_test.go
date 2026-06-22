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

	typesSrc, err := os.ReadFile("shein_submit_readiness_types.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_readiness_types.go) error = %v", err)
	}
	typesContent := string(typesSrc)
	for _, needle := range []string{
		"type SheinReadinessReason = sheinworkspace.ReadinessReason",
		"type SheinRepairHint = sheinworkspace.RepairHint[SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview]",
	} {
		if !strings.Contains(typesContent, needle) {
			t.Fatalf("shein_submit_readiness_types.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"type SheinReadinessReason struct {",
		"type SheinRepairHint struct {",
	} {
		if strings.Contains(typesContent, needle) {
			t.Fatalf("shein_submit_readiness_types.go should keep readiness DTOs in workspace, found %q", needle)
		}
	}

	guidanceSrc, err := os.ReadFile("shein_submit_readiness_guidance_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_readiness_guidance_support.go) error = %v", err)
	}
	guidanceContent := string(guidanceSrc)

	for _, needle := range []string{
		"func buildSheinReadinessGuidance(pkg *SheinPackage, key string, fieldPaths []string, suggestedAction string, warningOnly bool) sheinReadinessGuidance {",
		"reason: sheinworkspace.BuildReadinessReason(spec.Reason)",
		"patch := sheinworkspace.BuildReadinessPatchPayload(pkg, key)",
		"func cloneSheinRepairHints(items []SheinRepairHint) []SheinRepairHint {",
	} {
		if !strings.Contains(guidanceContent, needle) {
			t.Fatalf("shein_submit_readiness_guidance_support.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"&SheinReadinessReason{",
		"cloned := *reason",
		"func buildSheinReadinessPatchPayload(pkg *SheinPackage, key string) *SheinRepairPatchPayload {",
		"return sheinworkspace.BuildReadinessPatchPayload(pkg, key)",
		"func buildSheinReadinessReason(spec *sheinworkspace.ReadinessReasonSpec) *SheinReadinessReason {",
		"func cloneSheinReadinessReason(reason *SheinReadinessReason) *SheinReadinessReason {",
	} {
		if strings.Contains(guidanceContent, needle) {
			t.Fatalf("shein_submit_readiness_guidance_support.go should not keep patch payload wrapper %q", needle)
		}
	}
	for _, needle := range []string{
		`case "category", "category_review":`,
		`case "attributes", "attribute_review":`,
		`case "sale_attributes", "variants":`,
		`clonePlatformImageSetForEditor(pkg.Images)`,
		`ReviewNotes: append([]string(nil), pkg.ReviewNotes...)`,
	} {
		if strings.Contains(guidanceContent, needle) {
			t.Fatalf("shein_submit_readiness_guidance_support.go should delegate patch payload construction, found %q", needle)
		}
	}

	assertFileAbsent(t, "shein_submit_readiness_status_support.go")

	publishingStatusSrc, err := os.ReadFile("../publishing/shein/submit_readiness_status.go")
	if err != nil {
		t.Fatalf("ReadFile(../publishing/shein/submit_readiness_status.go) error = %v", err)
	}
	publishingStatusContent := string(publishingStatusSrc)
	for _, needle := range []string{
		"func HasAnySubmitSKU(pkg *Package) bool {",
		"func FinalSubmitImagesReady(pkg *Package, action string) (bool, string) {",
		"func HasSubmitImage(pkg *Package) bool {",
		"func ProductImageInfoHasImage(info *sheinproduct.ImageInfo) bool {",
		"func SubmitPricingReady(pkg *Package) bool {",
	} {
		if !strings.Contains(publishingStatusContent, needle) {
			t.Fatalf("publishing submit_readiness_status.go should contain %q", needle)
		}
	}
	if strings.Contains(publishingStatusContent, "func sheinSourceFactsReady(") {
		t.Fatal("shein_submit_readiness_status_support.go should not keep a source-facts wrapper; call SHEIN workspace readiness from checks assembly")
	}
	assertFileAbsent(t, "workspace/shein/source_facts_bridge.go")

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
	for _, needle := range []string{
		"func sheinHasBlockingPendingAttributes(",
		"func sheinCategoryReviewPending(",
		"func sheinSaleAttributeReviewPending(",
		"func sheinSaleAttributeStatusResolved(",
		"func sheinSaleAttributesReadyForSubmit(",
		"func sheinSaleAttributesReadinessFailureReasons(",
		"func sheinResolvedSaleAttributeReady(",
		"func sheinResolvedSaleAttributeValueReady(",
	} {
		if strings.Contains(buildValidationContent, needle) {
			t.Fatalf("shein_build_validation.go should not keep publishing-owned wrapper %q", needle)
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
		"sheinworkspace.BuildSubmitReadinessCheck(",
	} {
		if !strings.Contains(checksContent, needle) {
			t.Fatalf("shein_submit_readiness_checks_support.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"func sheinSubmitReadinessCheck(",
		"func sheinReadinessTaxonomyForKey(",
		"func sheinSubmitReadinessFinalDraftReady(pkg *SheinPackage, action string) bool {",
		"func sheinSubmitReadinessFinalReviewMessage(action string) string {",
		"return sheinpub.FinalReviewReady(pkg, action)",
		"return sheinpub.FinalReviewMessage(action)",
	} {
		if strings.Contains(checksContent, needle) {
			t.Fatalf("shein_submit_readiness_checks_support.go should not keep readiness wrapper %q", needle)
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
