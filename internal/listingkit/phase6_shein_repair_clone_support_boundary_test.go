package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinRepairCloneSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSrc, err := os.ReadFile("shein_repair_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_repair_support.go) error = %v", err)
	}
	rootContent := string(rootSrc)

	for _, needle := range []string{
		"type SheinRepairValidationPreview = sheinworkspace.RepairValidationPreview[RevisionFieldError]",
		"type SheinRepairPatchPayload = sheinworkspace.RepairPatchPayload",
		"type sheinRepairRevisionBundle struct {",
		"type sheinRepairArtifacts struct {",
	} {
		if !strings.Contains(rootContent, needle) {
			t.Fatalf("shein_repair_support.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func clonePlatformImageSetForEditor(set *PlatformImageSet) *PlatformImageSet {",
		"func cloneSheinRepairPatchPayload(payload *SheinRepairPatchPayload) *SheinRepairPatchPayload {",
		"func cloneSheinRepairArtifacts(patch *SheinRepairPatchPayload, skeleton *SheinEditorRevisionSkeleton, request *ApplyRevisionRequest, validation *SheinRepairValidationPreview) sheinRepairArtifacts {",
		"func cloneSheinRepairValidationPreview(src *SheinRepairValidationPreview) *SheinRepairValidationPreview {",
		"func cloneRevisionDiffPreview(src *RevisionDiffPreview) *RevisionDiffPreview {",
	} {
		if strings.Contains(rootContent, needle) {
			t.Fatalf("shein_repair_support.go should delegate clone support helper %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("shein_repair_clone_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_repair_clone_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func clonePlatformImageSetForEditor(set *PlatformImageSet) *PlatformImageSet {",
		"func cloneSheinRepairArtifacts(patch *SheinRepairPatchPayload, skeleton *SheinEditorRevisionSkeleton, request *ApplyRevisionRequest, validation *SheinRepairValidationPreview) sheinRepairArtifacts {",
		"sheinworkspace.CloneRepairPatchPayload(patch)",
		"func cloneSheinRepairValidationPreview(src *SheinRepairValidationPreview) *SheinRepairValidationPreview {",
		"func cloneRevisionDiffPreview(src *RevisionDiffPreview) *RevisionDiffPreview {",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("shein_repair_clone_support.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"func cloneSheinRepairPatchPayload(payload *SheinRepairPatchPayload) *SheinRepairPatchPayload {",
		"sheinworkspace.CloneRepairPatchPayload(payload)",
	} {
		if strings.Contains(supportContent, needle) {
			t.Fatalf("shein_repair_clone_support.go should not keep patch clone wrapper %q", needle)
		}
	}
}
