package workspace

import (
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

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

type RepairRevisionBundle[I any, S any, Q any] struct {
	Input    *I
	Skeleton *S
	Request  *Q
}

type RepairArtifacts[P any, S any, Q any, V any] struct {
	Patch      *P
	Skeleton   *S
	Request    *Q
	Validation *V
}

// CloneRepairPatchPayload returns a deep copy of repair patches before they are cached or reused.
func CloneRepairPatchPayload(payload *RepairPatchPayload) *RepairPatchPayload {
	if payload == nil {
		return nil
	}
	return &RepairPatchPayload{
		CategoryResolution:      cloneCategoryPatch(payload.CategoryResolution),
		AttributeResolution:     cloneAttributePatch(payload.AttributeResolution),
		SaleAttributeResolution: cloneSalePatch(payload.SaleAttributeResolution),
		SKCPatches:              cloneRepairSKCPatches(payload.SKCPatches),
		Images:                  cloneImageSet(payload.Images),
		ReviewNotes:             append([]string(nil), payload.ReviewNotes...),
	}
}

func cloneRepairSKCPatches(items []SKCRevisionPatch) []SKCRevisionPatch {
	if len(items) == 0 {
		return nil
	}
	out := make([]SKCRevisionPatch, 0, len(items))
	for _, item := range items {
		patch := item
		patch.SkcName = cloneStringPointer(item.SkcName)
		patch.SaleName = cloneStringPointer(item.SaleName)
		patch.MainImageURL = cloneStringPointer(item.MainImageURL)
		if item.SaleAttribute != nil {
			attr := *item.SaleAttribute
			patch.SaleAttribute = &attr
		}
		patch.SKUPatches = cloneRepairSKUPatches(item.SKUPatches)
		out = append(out, patch)
	}
	return out
}

func cloneRepairSKUPatches(items []SKURevisionPatch) []SKURevisionPatch {
	if len(items) == 0 {
		return nil
	}
	out := make([]SKURevisionPatch, 0, len(items))
	for _, item := range items {
		patch := item
		patch.Attributes = cloneMap(item.Attributes)
		patch.BasePrice = cloneStringPointer(item.BasePrice)
		patch.CostPrice = cloneStringPointer(item.CostPrice)
		patch.Currency = cloneStringPointer(item.Currency)
		patch.StockCount = cloneIntPointer(item.StockCount)
		patch.MainImage = cloneStringPointer(item.MainImage)
		patch.Barcode = cloneStringPointer(item.Barcode)
		patch.SaleAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), item.SaleAttributes...)
		patch.SitePriceList = append([]sheinpub.SitePrice(nil), item.SitePriceList...)
		patch.StockInfoList = append([]sheinpub.StockInfo(nil), item.StockInfoList...)
		out = append(out, patch)
	}
	return out
}

func cloneIntPointer(in *int) *int {
	if in == nil {
		return nil
	}
	out := *in
	return &out
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
