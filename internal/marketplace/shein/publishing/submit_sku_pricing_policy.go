package publishing

import (
	"strconv"
	"strings"
)

// SupplierSKURename records a supplier SKU replacement produced during submit normalization.
type SupplierSKURename struct {
	Old string
	New string
}

// StudioPricingSKUReference is the neutral SKU reference shape used by pricing policy.
type StudioPricingSKUReference struct {
	SupplierSKU string
}

// SubmitPricingSKUInput is the neutral submit SKU pricing shape used for readiness checks.
type SubmitPricingSKUInput struct {
	BasePrice      string
	SiteBasePrices []string
}

// StudioPricingReferences is the neutral pricing state needed for studio submit SKU reconciliation.
type StudioPricingReferences struct {
	CurrentSupplierSKUs       []string
	FinalManualPriceOverrides map[string]float64
	ManualOverrides           map[string]float64
	SKUPrices                 []StudioPricingSKUReference
}

// SubmitPricingReady reports whether draft SKU prices are complete and positive.
func SubmitPricingReady(skus []SubmitPricingSKUInput) bool {
	hasSKU := false
	for _, sku := range skus {
		hasSKU = true
		if parseSubmitMoney(sku.BasePrice) <= 0 {
			return false
		}
		if len(sku.SiteBasePrices) == 0 {
			return false
		}
		for _, sitePrice := range sku.SiteBasePrices {
			if parseSubmitMoney(sitePrice) <= 0 {
				return false
			}
		}
	}
	return hasSKU
}

// ApplyStudioSupplierSKURenames remaps pricing references after supplier SKU normalization.
func ApplyStudioSupplierSKURenames(state *StudioPricingReferences, renames []SupplierSKURename) {
	if state == nil || len(renames) == 0 {
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

	state.FinalManualPriceOverrides = remapPriceOverrides(state.FinalManualPriceOverrides, renameMap)
	state.ManualOverrides = remapPriceOverrides(state.ManualOverrides, renameMap)
	occurrence := map[string]int{}
	for i := range state.SKUPrices {
		oldKey := strings.TrimSpace(state.SKUPrices[i].SupplierSKU)
		replacements := renameMap[oldKey]
		switch {
		case len(replacements) == 0:
			continue
		case len(replacements) == 1:
			state.SKUPrices[i].SupplierSKU = replacements[0]
		default:
			index := occurrence[oldKey]
			if index >= len(replacements) {
				index = len(replacements) - 1
			}
			state.SKUPrices[i].SupplierSKU = replacements[index]
			occurrence[oldKey] = index + 1
		}
	}
}

// ReconcileStudioPricingReferences reconciles stale pricing references against current request draft SKUs.
func ReconcileStudioPricingReferences(state *StudioPricingReferences) bool {
	if state == nil || len(state.CurrentSupplierSKUs) == 0 {
		return false
	}
	currentSKUSet := make(map[string]struct{}, len(state.CurrentSupplierSKUs))
	aliasMap := make(map[string][]string, len(state.CurrentSupplierSKUs))
	for _, sku := range state.CurrentSupplierSKUs {
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
	if remapped, updated := reconcilePriceOverrideAliases(state.FinalManualPriceOverrides, currentSKUSet, aliasMap); updated {
		state.FinalManualPriceOverrides = remapped
		changed = true
	}
	if remapped, updated := reconcilePriceOverrideAliases(state.ManualOverrides, currentSKUSet, aliasMap); updated {
		state.ManualOverrides = remapped
		changed = true
	}
	occurrence := map[string]int{}
	for index := range state.SKUPrices {
		oldSKU := strings.TrimSpace(state.SKUPrices[index].SupplierSKU)
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
			state.SKUPrices[index].SupplierSKU = replacements[replaceIndex]
			changed = true
		}
		occurrence[oldSKU] = replaceIndex + 1
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
		if LooksLikeSubmitRequestToken(part) || LooksLikeSubmitTaskToken(part) {
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

func parseSubmitMoney(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}
