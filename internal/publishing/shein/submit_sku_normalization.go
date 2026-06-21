package shein

import "strings"

// StudioSubmitSKUContext carries task-scoped defaults for studio submit SKU normalization.
type StudioSubmitSKUContext struct {
	StyleID           string
	TaskDiscriminator string
	Variant           *SubmitVariantContext
}

// NormalizeStudioSubmitSupplierSKUs normalizes studio supplier SKUs across draft, preview, package, and pricing references.
func NormalizeStudioSubmitSupplierSKUs(pkg *Package, input StudioSubmitSKUContext) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || input.Variant == nil {
		return false
	}
	if strings.TrimSpace(input.StyleID) == "" &&
		strings.TrimSpace(input.Variant.ProductSKU) == "" &&
		strings.TrimSpace(input.Variant.VariantSKU) == "" &&
		len(input.Variant.Variants) == 0 {
		return false
	}

	seen := map[string]int{}
	renames := make([]SupplierSKURename, 0)
	changed := false
	globalIndex := 0

	for skcIndex := range pkg.DraftPayload.SKCList {
		draftSKC := &pkg.DraftPayload.SKCList[skcIndex]
		draftSKCValue := submitDraftSKCSaleAttributeValue(draftSKC)
		for skuIndex := range draftSKC.SKUList {
			draftSKU := &draftSKC.SKUList[skuIndex]
			oldSKU := strings.TrimSpace(draftSKU.SupplierSKU)
			matchedIndex := MatchSubmitVariantOptionIndex(input.Variant, draftSKCValue, draftSKU, globalIndex)
			match := submitVariantAt(input.Variant, matchedIndex)
			baseSKU := ResolveSubmitBaseSKU(input.Variant, draftSKU, match, oldSKU)
			requireVariantDiscriminator := SubmitRequiresVariantDiscriminator(input.Variant, baseSKU) || input.TaskDiscriminator != ""
			discriminator := ResolveSubmitVariantDiscriminator(input.Variant, draftSKU, match, matchedIndex, globalIndex, input.TaskDiscriminator)
			newSKU := BuildStudioVariantSKU(baseSKU, input.StyleID, discriminator, requireVariantDiscriminator, seen)
			if strings.TrimSpace(newSKU) == "" {
				globalIndex++
				continue
			}
			if oldSKU == "" || !strings.EqualFold(oldSKU, newSKU) {
				draftSKU.SupplierSKU = newSKU
				changed = true
			}
			renames = append(renames, SupplierSKURename{Old: oldSKU, New: newSKU})
			setPackageSKU(pkg, skcIndex, skuIndex, newSKU)
			globalIndex++
		}
	}

	if changed {
		ApplyStudioSupplierSKURenames(pkg, renames)
	}
	reconciled := ReconcileStudioPricingReferences(pkg)
	return changed || reconciled
}

// BuildStudioVariantSKU builds the final studio submit SKU from base, variant discriminator, and style suffix.
func BuildStudioVariantSKU(baseSKU, styleID, variantDiscriminator string, requireVariantDiscriminator bool, seen map[string]int) string {
	baseSKU = strings.TrimSpace(baseSKU)
	styleSuffix := NormalizeSubmitStyleSuffix(styleID)
	variantDiscriminator = normalizeSubmitVariantDiscriminator(variantDiscriminator)

	parts := make([]string, 0, 2)
	if baseSKU != "" {
		parts = append(parts, baseSKU)
	}
	if styleSuffix != "" {
		parts = append(parts, styleSuffix)
	}
	baseCandidate := strings.Join(parts, "-")
	if baseCandidate == "" {
		baseCandidate = "SDS-STUDIO-001"
	}
	if !requireVariantDiscriminator && seen == nil {
		return baseCandidate
	}
	if !requireVariantDiscriminator {
		if _, exists := seen[baseCandidate]; !exists {
			seen[baseCandidate] = 1
			return baseCandidate
		}
	}
	parts = parts[:0]
	if baseSKU != "" {
		parts = append(parts, baseSKU)
	}
	if variantDiscriminator != "" {
		parts = append(parts, variantDiscriminator)
	}
	if styleSuffix != "" {
		parts = append(parts, styleSuffix)
	}
	candidate := strings.Join(parts, "-")
	if candidate == "" {
		candidate = baseCandidate
	}
	if seen == nil {
		return candidate
	}
	if _, exists := seen[candidate]; !exists {
		seen[candidate] = 1
		return candidate
	}
	seen[candidate]++
	return candidate + "-" + strconvItoa(seen[candidate])
}

func submitDraftSKCSaleAttributeValue(draft *SKCRequestDraft) string {
	if draft == nil || draft.SaleAttribute == nil {
		return ""
	}
	return draft.SaleAttribute.Value
}

func submitVariantAt(input *SubmitVariantContext, index int) *SubmitVariantOption {
	if input == nil || index < 0 || index >= len(input.Variants) {
		return nil
	}
	return &input.Variants[index]
}

func setPackageSKU(pkg *Package, skcIndex, skuIndex int, newSKU string) {
	if skcIndex < len(pkg.SkcList) && skuIndex < len(pkg.SkcList[skcIndex].SKUs) {
		pkg.SkcList[skcIndex].SKUs[skuIndex].SKU = newSKU
	}
	if pkg.PreviewPayload != nil && skcIndex < len(pkg.PreviewPayload.SKCList) && skuIndex < len(pkg.PreviewPayload.SKCList[skcIndex].SKUS) {
		pkg.PreviewPayload.SKCList[skcIndex].SKUS[skuIndex].SupplierSKU = newSKU
		SetPreviewPayload(pkg, pkg.PreviewPayload)
	}
}

func strconvItoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := [20]byte{}
	index := len(digits)
	v := value
	for v > 0 {
		index--
		digits[index] = byte('0' + v%10)
		v /= 10
	}
	return string(digits[index:])
}
