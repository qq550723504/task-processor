package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinRepairCloneSupportBoundary(t *testing.T) {
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
		"func clonePlatformImageSetForEditor(set *PlatformImageSet) *PlatformImageSet {",
		"func cloneSheinRepairPatchPayload(payload *SheinRepairPatchPayload) *SheinRepairPatchPayload {",
		"func cloneSheinRepairArtifacts(patch *SheinRepairPatchPayload, skeleton *SheinEditorRevisionSkeleton, request *ApplyRevisionRequest, validation *SheinRepairValidationPreview) sheinRepairArtifacts {",
		"func cloneSheinRepairValidationPreview(src *SheinRepairValidationPreview) *SheinRepairValidationPreview {",
		"func cloneRevisionDiffPreview(src *RevisionDiffPreview) *RevisionDiffPreview {",
		"type sheinRepairArtifacts = sheinworkspace.RepairArtifacts[SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview]",
	} {
		if strings.Contains(rootContent, needle) {
			t.Fatalf("shein_workspace_repair_bridge.go should delegate clone support helper %q", needle)
		}
	}

	assertFileAbsent(t, "shein_repair_support.go")
	assertFileAbsent(t, "shein_repair_clone_support.go")

	repairCenterSrc, err := os.ReadFile("shein_repair_center.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_repair_center.go) error = %v", err)
	}
	repairCenterContent := string(repairCenterSrc)
	for _, needle := range []string{
		"sheinworkspace.CloneRepairPatchPayload(patch)",
		"sheinworkspace.CloneEditorRevisionSkeleton(skeleton)",
		"cloneApplyRevisionRequest(request)",
		"sheinworkspace.CloneRepairValidationPreview(validation)",
	} {
		if !strings.Contains(repairCenterContent, needle) {
			t.Fatalf("shein_repair_center.go should clone artifacts directly with %q", needle)
		}
	}

	restoreSrc, err := os.ReadFile("revision_restore_request.go")
	if err != nil {
		t.Fatalf("ReadFile(revision_restore_request.go) error = %v", err)
	}
	restoreContent := string(restoreSrc)
	for _, needle := range []string{
		`common "task-processor/internal/publishing/common"`,
		"Images:           common.CloneImageSet(src.Images)",
	} {
		if !strings.Contains(restoreContent, needle) {
			t.Fatalf("revision_restore_request.go should contain %q", needle)
		}
	}
	if strings.Contains(restoreContent, "clonePlatformImageSetForEditor") {
		t.Fatal("revision_restore_request.go should use publishing/common image set clone")
	}
}
