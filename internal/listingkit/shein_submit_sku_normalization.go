package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

type sheinStudioSupplierSKURename = sheinpub.SupplierSKURename

func normalizeSheinStudioSubmitSupplierSKUs(task *Task, pkg *sheinpub.Package, submitRequestID string) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if task == nil || task.Request == nil || task.Request.Options == nil || pkg == nil || pkg.DraftPayload == nil {
		return false
	}
	sds := task.Request.Options.SDS
	if sds == nil {
		return false
	}
	taskDiscriminator := combineStudioSubmitDiscriminators(
		studioSubmitTaskDiscriminator(task.ID),
		studioSubmitRequestDiscriminator(submitRequestID),
	)
	styleID := resolveStudioSubmitStyleSuffix(task)
	if styleID == "" && strings.TrimSpace(sds.ProductSKU) == "" && strings.TrimSpace(sds.VariantSKU) == "" && len(sds.Variants) == 0 {
		return false
	}

	seen := map[string]int{}
	renames := make([]sheinStudioSupplierSKURename, 0)
	changed := false
	globalIndex := 0

	for skcIndex := range pkg.DraftPayload.SKCList {
		draftSKC := &pkg.DraftPayload.SKCList[skcIndex]
		for skuIndex := range draftSKC.SKUList {
			draftSKU := &draftSKC.SKUList[skuIndex]
			oldSKU := strings.TrimSpace(draftSKU.SupplierSKU)
			match, matchedIndex := matchStudioSubmitVariantOption(sds, draftSKC, draftSKU, globalIndex)
			baseSKU := resolveStudioSubmitBaseSKU(sds, draftSKU, match, oldSKU)
			requireVariantDiscriminator := studioSubmitRequiresVariantDiscriminator(sds, baseSKU) || taskDiscriminator != ""
			discriminator := resolveStudioSubmitVariantDiscriminator(sds, draftSKU, match, matchedIndex, globalIndex, taskDiscriminator)
			newSKU := buildStudioVariantSKU(baseSKU, styleID, discriminator, requireVariantDiscriminator, seen)
			if strings.TrimSpace(newSKU) == "" {
				globalIndex++
				continue
			}
			if oldSKU == "" || !strings.EqualFold(oldSKU, newSKU) {
				draftSKU.SupplierSKU = newSKU
				changed = true
			}
			renames = append(renames, sheinStudioSupplierSKURename{Old: oldSKU, New: newSKU})

			if skcIndex < len(pkg.SkcList) && skuIndex < len(pkg.SkcList[skcIndex].SKUs) {
				pkg.SkcList[skcIndex].SKUs[skuIndex].SKU = newSKU
			}
			if pkg.PreviewPayload != nil && skcIndex < len(pkg.PreviewPayload.SKCList) && skuIndex < len(pkg.PreviewPayload.SKCList[skcIndex].SKUS) {
				pkg.PreviewPayload.SKCList[skcIndex].SKUS[skuIndex].SupplierSKU = newSKU
				sheinpub.SetPreviewPayload(pkg, pkg.PreviewPayload)
			}
			globalIndex++
		}
	}

	if changed {
		applySheinStudioSupplierSKURenames(pkg, renames)
	}
	reconciled := reconcileSheinStudioPricingReferences(pkg)
	return changed || reconciled
}
