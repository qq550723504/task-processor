package listingkit

import (
	listingworkspace "task-processor/internal/listingkit/workspace/shein"
)

type SheinRepairValidationPreview = listingworkspace.RepairValidationPreview[RevisionFieldError]

type SheinRepairPatchPayload struct {
	CategoryResolution      *SheinCategoryResolutionPatch      `json:"category_resolution,omitempty"`
	AttributeResolution     *SheinAttributeResolutionPatch     `json:"attribute_resolution,omitempty"`
	SaleAttributeResolution *SheinSaleAttributeResolutionPatch `json:"sale_attribute_resolution,omitempty"`
	SKCPatches              []SheinSKCRevisionPatch            `json:"skc_patches,omitempty"`
	Images                  *PlatformImageSet                  `json:"images,omitempty"`
	ReviewNotes             []string                           `json:"review_notes,omitempty"`
}

type sheinRepairRevisionBundle struct {
	input    *SheinRevisionInput
	skeleton *SheinEditorRevisionSkeleton
	request  *ApplyRevisionRequest
}

type sheinRepairArtifacts struct {
	patch      *SheinRepairPatchPayload
	skeleton   *SheinEditorRevisionSkeleton
	request    *ApplyRevisionRequest
	validation *SheinRepairValidationPreview
}
