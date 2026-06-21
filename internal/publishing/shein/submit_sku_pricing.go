package shein

import "strings"

// SupplierSKURename records a supplier SKU replacement produced during submit normalization.
type SupplierSKURename struct {
	Old string
	New string
}

// ApplyStudioSupplierSKURenames remaps pricing references after supplier SKU normalization.
func ApplyStudioSupplierSKURenames(pkg *Package, renames []SupplierSKURename) {
	if pkg == nil || len(renames) == 0 {
		return
	}
	renameMap := make(map[string][]string, len(renames))
	for _, item := range renames {
		oldKey := strings.TrimSpace(item.Old)
		newKey := strings.TrimSpace(item.New)
		if newKey == "" {
			continue
		}
		existing := renameMap[oldKey]
		duplicate := false
		for _, candidate := range existing {
			if strings.EqualFold(candidate, newKey) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			renameMap[oldKey] = append(renameMap[oldKey], newKey)
		}
	}

	if pkg.FinalSubmissionDraft != nil {
		pkg.FinalSubmissionDraft.ManualPriceOverrides = remapPriceOverrides(pkg.FinalSubmissionDraft.ManualPriceOverrides, renameMap)
	}
	if pkg.Pricing != nil {
		pkg.Pricing.ManualOverrides = remapPriceOverrides(pkg.Pricing.ManualOverrides, renameMap)
		occurrence := map[string]int{}
		for i := range pkg.Pricing.SKUPrices {
			oldKey := strings.TrimSpace(pkg.Pricing.SKUPrices[i].SupplierSKU)
			replacements := renameMap[oldKey]
			switch {
			case len(replacements) == 0:
				continue
			case len(replacements) == 1:
				pkg.Pricing.SKUPrices[i].SupplierSKU = replacements[0]
			default:
				index := occurrence[oldKey]
				if index >= len(replacements) {
					index = len(replacements) - 1
				}
				pkg.Pricing.SKUPrices[i].SupplierSKU = replacements[index]
				occurrence[oldKey] = index + 1
			}
		}
	}
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
	currentSKUSet := make(map[string]struct{}, len(currentSKUs))
	aliasMap := make(map[string][]string, len(currentSKUs))
	for _, sku := range currentSKUs {
		currentSKUSet[sku] = struct{}{}
		aliasKey := StudioPricingSKUAlias(sku)
		if aliasKey == "" {
			continue
		}
		duplicate := false
		for _, existing := range aliasMap[aliasKey] {
			if strings.EqualFold(existing, sku) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			aliasMap[aliasKey] = append(aliasMap[aliasKey], sku)
		}
	}

	changed := false
	if pkg.FinalSubmissionDraft != nil {
		remapped, updated := reconcilePriceOverrideAliases(pkg.FinalSubmissionDraft.ManualPriceOverrides, currentSKUSet, aliasMap)
		if updated {
			pkg.FinalSubmissionDraft.ManualPriceOverrides = remapped
			changed = true
		}
	}
	if pkg.Pricing != nil {
		remapped, updated := reconcilePriceOverrideAliases(pkg.Pricing.ManualOverrides, currentSKUSet, aliasMap)
		if updated {
			pkg.Pricing.ManualOverrides = remapped
			changed = true
		}
		occurrence := map[string]int{}
		for index := range pkg.Pricing.SKUPrices {
			oldSKU := strings.TrimSpace(pkg.Pricing.SKUPrices[index].SupplierSKU)
			if oldSKU == "" {
				continue
			}
			if _, ok := currentSKUSet[oldSKU]; ok {
				continue
			}
			replacements := aliasMap[StudioPricingSKUAlias(oldSKU)]
			if len(replacements) == 0 {
				continue
			}
			replaceIndex := occurrence[oldSKU]
			if replaceIndex >= len(replacements) {
				replaceIndex = len(replacements) - 1
			}
			if !strings.EqualFold(oldSKU, replacements[replaceIndex]) {
				pkg.Pricing.SKUPrices[index].SupplierSKU = replacements[replaceIndex]
				changed = true
			}
			occurrence[oldSKU] = replaceIndex + 1
		}
	}
	return changed
}

// StudioPricingSKUAlias returns a stable alias for matching stale pricing SKU references.
func StudioPricingSKUAlias(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "-")
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if looksLikeSubmitRequestToken(part) || looksLikeSubmitTaskToken(part) {
			continue
		}
		filtered = append(filtered, part)
	}
	alias := strings.Join(filtered, "-")
	if prefix, ok := trimPricingStyleLikeSuffix(alias); ok {
		return prefix
	}
	return alias
}

func remapPriceOverrides(input map[string]float64, renameMap map[string][]string) map[string]float64 {
	if len(input) == 0 {
		return input
	}
	output := make(map[string]float64, len(input))
	for key, value := range input {
		replacements := renameMap[strings.TrimSpace(key)]
		if len(replacements) == 0 {
			output[key] = value
			continue
		}
		for _, replacement := range replacements {
			output[replacement] = value
		}
	}
	return output
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

func reconcilePriceOverrideAliases(
	input map[string]float64,
	currentSKUSet map[string]struct{},
	aliasMap map[string][]string,
) (map[string]float64, bool) {
	if len(input) == 0 {
		return input, false
	}
	output := make(map[string]float64, len(input))
	changed := false
	for key, value := range input {
		trimmed := strings.TrimSpace(key)
		if _, ok := currentSKUSet[trimmed]; ok {
			output[trimmed] = value
			continue
		}
		replacements := aliasMap[StudioPricingSKUAlias(trimmed)]
		if len(replacements) == 0 {
			output[key] = value
			continue
		}
		changed = true
		for _, replacement := range replacements {
			output[replacement] = value
		}
	}
	return output, changed
}

func trimPricingStyleLikeSuffix(value string) (string, bool) {
	index := strings.LastIndex(value, "-")
	if index <= 0 {
		return "", false
	}
	suffix := strings.TrimSpace(value[index+1:])
	if len(suffix) != 8 {
		return "", false
	}
	for _, r := range suffix {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		default:
			return "", false
		}
	}
	prefix := strings.TrimSpace(value[:index])
	if prefix == "" || !strings.ContainsAny(prefix, "0123456789") {
		return "", false
	}
	return prefix, true
}

func looksLikeSubmitRequestToken(token string) bool {
	token = strings.TrimSpace(strings.ToUpper(token))
	if len(token) < 6 || len(token) > 9 || !strings.HasPrefix(token, "R") {
		return false
	}
	for _, r := range token[1:] {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

func looksLikeSubmitTaskToken(token string) bool {
	token = strings.TrimSpace(strings.ToUpper(token))
	if len(token) != 9 || !strings.HasPrefix(token, "T") {
		return false
	}
	for _, r := range token[1:] {
		switch {
		case r >= '0' && r <= '9', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
