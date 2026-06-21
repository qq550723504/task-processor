package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func applySheinStudioSupplierSKURenames(pkg *sheinpub.Package, renames []sheinStudioSupplierSKURename) {
	sheinpub.ApplyStudioSupplierSKURenames(pkg, renames)
}

func reconcileSheinStudioPricingReferences(pkg *sheinpub.Package) bool {
	return sheinpub.ReconcileStudioPricingReferences(pkg)
}

func sheinStudioPricingSKUAlias(value string) string {
	return sheinpub.StudioPricingSKUAlias(value)
}
