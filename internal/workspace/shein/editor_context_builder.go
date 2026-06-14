package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

func BuildEditorContext(pkg *sheinpub.Package) *EditorContext {
	return sheinmarketplace.BuildEditorContext(pkg)
}

func BuildCategoryResolutionPatch(pkg *sheinpub.Package) *CategoryResolutionPatch {
	return sheinmarketplace.BuildCategoryResolutionPatch(pkg)
}

func BuildAttributeResolutionPatch(pkg *sheinpub.Package) *AttributeResolutionPatch {
	return sheinmarketplace.BuildAttributeResolutionPatch(pkg)
}

func BuildSaleAttributeResolutionPatch(pkg *sheinpub.Package) *SaleAttributeResolutionPatch {
	return sheinmarketplace.BuildSaleAttributeResolutionPatch(pkg)
}

func BuildEditorSKCPatches(pkg *sheinpub.Package) []SKCRevisionPatch {
	return sheinmarketplace.BuildEditorSKCPatches(pkg)
}
