package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinRepairRevisionSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSrc, err := os.ReadFile("shein_workspace_repair_bridge.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_workspace_repair_bridge.go) error = %v", err)
	}
	rootContent := string(rootSrc)

	for _, needle := range []string{
		"type SheinRepairValidationPreview = sheinworkspace.RepairValidationPreview[RevisionFieldError]",
		"type SheinRepairPatchPayload = sheinworkspace.RepairPatchPayload",
	} {
		if !strings.Contains(rootContent, needle) {
			t.Fatalf("shein_workspace_repair_bridge.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func buildSheinRepairArtifacts(pkg *SheinPackage, action string, editorSection string, patch *SheinRepairPatchPayload) sheinworkspace.RepairArtifacts[SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview] {",
	} {
		if strings.Contains(rootContent, needle) {
			t.Fatalf("shein_workspace_repair_bridge.go should delegate revision support helper %q", needle)
		}
	}
	assertFileAbsent(t, "shein_repair_support.go")

	revisionSrc, err := os.ReadFile("shein_repair_revision_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_repair_revision_support.go) error = %v", err)
	}
	revisionContent := string(revisionSrc)

	for _, needle := range []string{
		"func buildSheinRepairApplyRequest(seed sheinworkspace.RepairRevisionSeed) *ApplyRevisionRequest {",
		"func buildSheinRepairArtifacts(pkg *SheinPackage, action string, editorSection string, patch *SheinRepairPatchPayload) sheinworkspace.RepairArtifacts[SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview] {",
		"func buildSheinRepairValidationPreview(pkg *SheinPackage, editorSection string, revision *ApplyRevisionRequest, skeleton *SheinEditorRevisionSkeleton) *SheinRepairValidationPreview {",
		"sheinworkspace.BuildRepairRevisionSeed(action, patch)",
		"return sheinworkspace.RepairArtifacts[SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview]{",
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
		"func buildSheinRepairRevisionBundle(action string, payload *SheinRepairPatchPayload) sheinRepairRevisionBundle {",
		"func buildSheinRepairApplyRequest(action string, payload *SheinRepairPatchPayload) *ApplyRevisionRequest {",
		"func buildSheinRepairRevisionInput(payload *SheinRepairPatchPayload) *SheinRevisionInput {",
		"func buildSheinRepairReason(action string) string {",
		"type sheinRepairArtifacts = sheinworkspace.RepairArtifacts[SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview]",
		"sheinworkspace.BuildRepairRevisionInput(payload)",
		"sheinworkspace.BuildRepairReason(action)",
	} {
		if strings.Contains(revisionContent, needle) {
			t.Fatalf("shein_repair_revision_support.go should not keep unused repair revision wrapper %q", needle)
		}
	}
	assertFileAbsent(t, "shein_repair_validation_support.go")
}
