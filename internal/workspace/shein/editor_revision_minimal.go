package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
)

func BuildMinimalRevisionSkeleton(full *EditorRevisionSkeleton) *EditorRevisionSkeleton {
	return sheinmarketplace.BuildMinimalRevisionSkeleton(full)
}

func PruneRevisionInput(input *RevisionInput) *RevisionInput {
	return sheinmarketplace.PruneRevisionInput(input)
}

func IsEmptyRevisionInput(input *RevisionInput) bool {
	return sheinmarketplace.IsEmptyRevisionInput(input)
}

func PruneCategoryResolutionPatch(patch *CategoryResolutionPatch) *CategoryResolutionPatch {
	return sheinmarketplace.PruneCategoryResolutionPatch(patch)
}

func PruneAttributeResolutionPatch(patch *AttributeResolutionPatch) *AttributeResolutionPatch {
	return sheinmarketplace.PruneAttributeResolutionPatch(patch)
}

func PruneSaleAttributeResolutionPatch(patch *SaleAttributeResolutionPatch) *SaleAttributeResolutionPatch {
	return sheinmarketplace.PruneSaleAttributeResolutionPatch(patch)
}
