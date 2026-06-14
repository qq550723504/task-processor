package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

func BuildEditorRevisionSkeleton(
	pkg *sheinpub.Package,
	categoryPatch *CategoryResolutionPatch,
	attributePatch *AttributeResolutionPatch,
	salePatch *SaleAttributeResolutionPatch,
	skcPatches []SKCRevisionPatch,
) *EditorRevisionSkeleton {
	return sheinmarketplace.BuildEditorRevisionSkeleton(pkg, categoryPatch, attributePatch, salePatch, skcPatches)
}

func CloneCategoryResolutionPatch(src *CategoryResolutionPatch) *CategoryResolutionPatch {
	return sheinmarketplace.CloneCategoryResolutionPatch(src)
}

func CloneAttributeResolutionPatch(src *AttributeResolutionPatch) *AttributeResolutionPatch {
	return sheinmarketplace.CloneAttributeResolutionPatch(src)
}

func CloneSaleAttributeResolutionPatch(src *SaleAttributeResolutionPatch) *SaleAttributeResolutionPatch {
	return sheinmarketplace.CloneSaleAttributeResolutionPatch(src)
}

func CloneSKCRevisionPatches(items []SKCRevisionPatch) []SKCRevisionPatch {
	return sheinmarketplace.CloneSKCRevisionPatches(items)
}
