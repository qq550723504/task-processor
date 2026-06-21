package listingkit

import (
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

func applySheinCategoryResolutionPatch(pkg *sheinpub.Package, patch *SheinCategoryResolutionPatch) {
	sheinworkspace.ApplyCategoryResolutionPatch(pkg, patch)
}

func applySheinAttributeResolutionPatch(pkg *sheinpub.Package, patch *SheinAttributeResolutionPatch) {
	sheinworkspace.ApplyAttributeResolutionPatch(pkg, patch)
}

func applySheinSaleAttributeResolutionPatch(pkg *sheinpub.Package, patch *SheinSaleAttributeResolutionPatch) {
	sheinworkspace.ApplySaleAttributeResolutionPatch(pkg, patch)
}
