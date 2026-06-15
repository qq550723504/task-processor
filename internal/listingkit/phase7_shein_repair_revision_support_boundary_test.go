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
		"type SheinRepairValidationPreview = listingworkspace.RepairValidationPreview[RevisionFieldError]",
		"type SheinRepairPatchPayload struct {",
		"type sheinRepairRevisionBundle struct {",
		"type sheinRepairArtifacts struct {",
	} {
		if !strings.Contains(rootContent, needle) {
			t.Fatalf("shein_repair_support.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func buildSheinRepairRevisionBundle(action string, payload *SheinRepairPatchPayload) sheinRepairRevisionBundle {",
		"func buildSheinRepairRevisionSkeleton(action string, payload *SheinRepairPatchPayload) *SheinEditorRevisionSkeleton {",
		"func buildSheinRepairApplyRequest(action string, payload *SheinRepairPatchPayload) *ApplyRevisionRequest {",
		"func buildSheinRepairRevisionInput(payload *SheinRepairPatchPayload) *SheinRevisionInput {",
		"func buildSheinRepairReason(action string) string {",
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
		"func buildSheinRepairRevisionSkeleton(action string, payload *SheinRepairPatchPayload) *SheinEditorRevisionSkeleton {",
		"func buildSheinRepairApplyRequest(action string, payload *SheinRepairPatchPayload) *ApplyRevisionRequest {",
		"func buildSheinRepairRevisionInput(payload *SheinRepairPatchPayload) *SheinRevisionInput {",
		"func buildSheinRepairReason(action string) string {",
		"func buildSheinRepairArtifacts(pkg *SheinPackage, action string, editorSection string, patch *SheinRepairPatchPayload) sheinRepairArtifacts {",
	} {
		if !strings.Contains(revisionContent, needle) {
			t.Fatalf("shein_repair_revision_support.go should contain %q", needle)
		}
	}
}
