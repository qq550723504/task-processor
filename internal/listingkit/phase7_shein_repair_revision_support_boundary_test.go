package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinRepairRevisionSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSrc, err := os.ReadFile("shein_repair_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_repair_support.go) error = %v", err)
	}
	rootContent := string(rootSrc)

	for _, needle := range []string{
		"type SheinRepairValidationPreview = sheinworkspace.RepairValidationPreview[RevisionFieldError]",
		"type SheinRepairPatchPayload = sheinworkspace.RepairPatchPayload",
		"type sheinRepairRevisionBundle = sheinworkspace.RepairRevisionBundle[SheinRevisionInput, SheinEditorRevisionSkeleton, ApplyRevisionRequest]",
		"type sheinRepairArtifacts = sheinworkspace.RepairArtifacts[SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview]",
	} {
		if !strings.Contains(rootContent, needle) {
			t.Fatalf("shein_repair_support.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func buildSheinRepairRevisionBundle(action string, payload *SheinRepairPatchPayload) sheinRepairRevisionBundle {",
		"func buildSheinRepairArtifacts(pkg *SheinPackage, action string, editorSection string, patch *SheinRepairPatchPayload) sheinRepairArtifacts {",
	} {
		if strings.Contains(rootContent, needle) {
			t.Fatalf("shein_repair_support.go should delegate revision support helper %q", needle)
		}
	}

	revisionSrc, err := os.ReadFile("shein_repair_revision_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_repair_revision_support.go) error = %v", err)
	}
	revisionContent := string(revisionSrc)

	for _, needle := range []string{
		"func buildSheinRepairRevisionBundle(action string, payload *SheinRepairPatchPayload) sheinRepairRevisionBundle {",
		"func buildSheinRepairArtifacts(pkg *SheinPackage, action string, editorSection string, patch *SheinRepairPatchPayload) sheinRepairArtifacts {",
		"func buildSheinRepairValidationPreview(pkg *SheinPackage, editorSection string, revision *ApplyRevisionRequest, skeleton *SheinEditorRevisionSkeleton) *SheinRepairValidationPreview {",
		"sheinworkspace.BuildRepairRevisionSeed(action, payload)",
		"Input:    seed.Input",
		"Request: &ApplyRevisionRequest{",
		"sheinworkspace.BuildRepairValidationPreview(pkg, editorSection, skeleton, valid, fieldErrors)",
	} {
		if !strings.Contains(revisionContent, needle) {
			t.Fatalf("shein_repair_revision_support.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"cloneSheinCategoryResolutionPatch(payload.CategoryResolution)",
		"pruneSheinRevisionInput(input)",
		`Reason:   buildSheinRepairReason(action),`,
	} {
		if strings.Contains(revisionContent, needle) {
			t.Fatalf("shein_repair_revision_support.go should delegate repair revision detail %q", needle)
		}
	}
	for _, needle := range []string{
		"func buildSheinRepairRevisionSkeleton(action string, payload *SheinRepairPatchPayload) *SheinEditorRevisionSkeleton {",
		"func buildSheinRepairApplyRequest(action string, payload *SheinRepairPatchPayload) *ApplyRevisionRequest {",
		"func buildSheinRepairRevisionInput(payload *SheinRepairPatchPayload) *SheinRevisionInput {",
		"func buildSheinRepairReason(action string) string {",
		"sheinworkspace.BuildRepairRevisionInput(payload)",
		"sheinworkspace.BuildRepairReason(action)",
	} {
		if strings.Contains(revisionContent, needle) {
			t.Fatalf("shein_repair_revision_support.go should not keep unused repair revision wrapper %q", needle)
		}
	}
	assertFileAbsent(t, "shein_repair_validation_support.go")
}
