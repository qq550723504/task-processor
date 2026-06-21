package workspace

import common "task-processor/internal/publishing/common"

// RepairPatchPayload carries direct repair patches that can be converted into a revision request seed.
type RepairPatchPayload struct {
	CategoryResolution      *CategoryResolutionPatch      `json:"category_resolution,omitempty"`
	AttributeResolution     *AttributeResolutionPatch     `json:"attribute_resolution,omitempty"`
	SaleAttributeResolution *SaleAttributeResolutionPatch `json:"sale_attribute_resolution,omitempty"`
	SKCPatches              []SKCRevisionPatch            `json:"skc_patches,omitempty"`
	Images                  *common.ImageSet              `json:"images,omitempty"`
	ReviewNotes             []string                      `json:"review_notes,omitempty"`
}

// RepairRevisionSeed is a platform-owned repair revision draft before app-layer request wrapping.
type RepairRevisionSeed struct {
	Input    *RevisionInput
	Skeleton *EditorRevisionSkeleton
}

// BuildRepairRevisionSeed builds a minimal SHEIN revision skeleton from a repair patch payload.
func BuildRepairRevisionSeed(action string, payload *RepairPatchPayload) RepairRevisionSeed {
	input := BuildRepairRevisionInput(payload)
	if input == nil {
		return RepairRevisionSeed{}
	}
	minimal := PruneRevisionInput(input)
	if minimal == nil || IsEmptyRevisionInput(minimal) {
		return RepairRevisionSeed{}
	}
	return RepairRevisionSeed{
		Input: input,
		Skeleton: &EditorRevisionSkeleton{
			Platform: "shein",
			Actor:    "desktop-client",
			Reason:   BuildRepairReason(action),
			Shein:    minimal,
		},
	}
}

// BuildRepairRevisionInput converts a repair patch payload into a SHEIN revision input.
func BuildRepairRevisionInput(payload *RepairPatchPayload) *RevisionInput {
	if payload == nil {
		return nil
	}
	input := &RevisionInput{
		CategoryResolution:      cloneCategoryPatch(payload.CategoryResolution),
		AttributeResolution:     cloneAttributePatch(payload.AttributeResolution),
		SaleAttributeResolution: cloneSalePatch(payload.SaleAttributeResolution),
		SKCPatches:              cloneSKCPatches(payload.SKCPatches),
		Images:                  cloneImageSet(payload.Images),
		ReviewNotes:             append([]string(nil), payload.ReviewNotes...),
	}
	if IsEmptyRevisionInput(input) {
		return nil
	}
	return input
}

// BuildRepairReason builds a stable revision reason for a repair action.
func BuildRepairReason(action string) string {
	if action == "" {
		return "repair suggested issue"
	}
	return "repair: " + action
}
