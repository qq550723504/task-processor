package shein

import (
	"strings"

	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
)

// SupplierSKURename records a supplier SKU replacement produced during submit normalization.
type SupplierSKURename = sheinmarketpub.SupplierSKURename

// ApplyStudioSupplierSKURenames remaps pricing references after supplier SKU normalization.
func ApplyStudioSupplierSKURenames(pkg *Package, renames []SupplierSKURename) {
	if pkg == nil || len(renames) == 0 {
		return
	}
	state := studioPricingReferenceState(pkg)
	sheinmarketpub.ApplyStudioSupplierSKURenames(state, renames)
	applyStudioPricingReferenceState(pkg, state)
}

// ReconcileStudioPricingReferences reconciles stale pricing references against current request draft SKUs.
func ReconcileStudioPricingReferences(pkg *Package) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return false
	}
	currentSKUs := collectRequestDraftSupplierSKUs(pkg.DraftPayload)
	if len(currentSKUs) == 0 {
		return false
	}
	state := studioPricingReferenceState(pkg)
	state.CurrentSupplierSKUs = currentSKUs
	changed := sheinmarketpub.ReconcileStudioPricingReferences(state)
	if changed {
		applyStudioPricingReferenceState(pkg, state)
	}
	return changed
}

// StudioPricingSKUAlias returns a stable alias for matching stale pricing SKU references.
func StudioPricingSKUAlias(value string) string {
	return sheinmarketpub.StudioPricingSKUAlias(value)
}

func studioPricingReferenceState(pkg *Package) *sheinmarketpub.StudioPricingReferences {
	state := &sheinmarketpub.StudioPricingReferences{}
	if pkg == nil {
		return state
	}
	if pkg.FinalSubmissionDraft != nil {
		state.FinalManualPriceOverrides = pkg.FinalSubmissionDraft.ManualPriceOverrides
	}
	if pkg.Pricing != nil {
		state.ManualOverrides = pkg.Pricing.ManualOverrides
		state.SKUPrices = make([]sheinmarketpub.StudioPricingSKUReference, 0, len(pkg.Pricing.SKUPrices))
		for _, item := range pkg.Pricing.SKUPrices {
			state.SKUPrices = append(state.SKUPrices, sheinmarketpub.StudioPricingSKUReference{
				SupplierSKU: item.SupplierSKU,
			})
		}
	}
	return state
}

func applyStudioPricingReferenceState(pkg *Package, state *sheinmarketpub.StudioPricingReferences) {
	if pkg == nil || state == nil {
		return
	}
	if pkg.FinalSubmissionDraft != nil {
		pkg.FinalSubmissionDraft.ManualPriceOverrides = state.FinalManualPriceOverrides
	}
	if pkg.Pricing != nil {
		pkg.Pricing.ManualOverrides = state.ManualOverrides
		for index := range pkg.Pricing.SKUPrices {
			if index >= len(state.SKUPrices) {
				break
			}
			pkg.Pricing.SKUPrices[index].SupplierSKU = state.SKUPrices[index].SupplierSKU
		}
	}
}

func collectRequestDraftSupplierSKUs(draft *RequestDraft) []string {
	if draft == nil {
		return nil
	}
	skus := make([]string, 0)
	for _, skc := range draft.SKCList {
		for _, sku := range skc.SKUList {
			if value := strings.TrimSpace(sku.SupplierSKU); value != "" {
				skus = append(skus, value)
			}
		}
	}
	return skus
}
