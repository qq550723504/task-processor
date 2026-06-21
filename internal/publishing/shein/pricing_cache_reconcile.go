package shein

import "strings"

type pricingCacheDraftReference struct {
	supplierSKU  string
	supplierCode string
	alias        string
}

// ReconcilePricingCacheReview aligns a cached pricing review with the current draft SKUs.
func ReconcilePricingCacheReview(pkg *Package, review *PricingReview) *PricingReview {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || review == nil {
		return review
	}
	refs := collectPricingCacheDraftReferences(pkg)
	if len(refs) == 0 {
		return review
	}

	refsByAlias := make(map[string][]pricingCacheDraftReference, len(refs))
	for _, ref := range refs {
		if ref.alias == "" {
			continue
		}
		refsByAlias[ref.alias] = append(refsByAlias[ref.alias], ref)
	}
	manualByAlias := pricingCacheManualOverridesByAlias(review.ManualOverrides)
	remappedManuals := make(map[string]float64, len(refs))
	for _, ref := range refs {
		if value, ok := manualByAlias[ref.alias]; ok && value > 0 {
			remappedManuals[ref.supplierSKU] = value
		}
	}

	occurrence := map[string]int{}
	for idx := range review.SKUPrices {
		price := &review.SKUPrices[idx]
		alias := pricingCacheReviewSKUKey(*price)
		if replacements := refsByAlias[alias]; len(replacements) > 0 {
			replacementIndex := occurrence[alias]
			if replacementIndex >= len(replacements) {
				replacementIndex = len(replacements) - 1
			}
			replacement := replacements[replacementIndex]
			price.SupplierSKU = replacement.supplierSKU
			if replacement.supplierCode != "" {
				price.SupplierCode = replacement.supplierCode
			}
			occurrence[alias] = replacementIndex + 1
			alias = replacement.alias
		}
		if value, ok := manualByAlias[alias]; ok && value > 0 {
			price.FinalPrice = value
			price.Manual = true
		}
	}
	if len(remappedManuals) > 0 {
		review.ManualOverrides = remappedManuals
	}
	return review
}

func collectPricingCacheDraftReferences(pkg *Package) []pricingCacheDraftReference {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return nil
	}
	refs := make([]pricingCacheDraftReference, 0)
	for _, skc := range pkg.DraftPayload.SKCList {
		for _, sku := range skc.SKUList {
			supplierSKU := strings.TrimSpace(sku.SupplierSKU)
			if supplierSKU == "" {
				continue
			}
			alias := pricingCacheDraftSKUKey(&sku)
			if alias == "" {
				alias = pricingCacheSKUAlias(supplierSKU)
			}
			if alias == "" {
				continue
			}
			refs = append(refs, pricingCacheDraftReference{
				supplierSKU:  supplierSKU,
				supplierCode: strings.TrimSpace(skc.SupplierCode),
				alias:        alias,
			})
		}
	}
	return refs
}

func pricingCacheManualOverridesByAlias(overrides map[string]float64) map[string]float64 {
	if len(overrides) == 0 {
		return nil
	}
	byAlias := make(map[string]float64, len(overrides))
	for sku, price := range overrides {
		if price <= 0 {
			continue
		}
		alias := pricingCacheSKUAlias(sku)
		if alias == "" {
			continue
		}
		byAlias[alias] = price
	}
	return byAlias
}

func pricingCacheDraftSKUKey(sku *SKUDraft) string {
	if sku == nil {
		return ""
	}
	if source := strings.TrimSpace(sku.Attributes["source_sds_sku"]); source != "" {
		return pricingCacheSKUAlias(source)
	}
	return pricingCacheSKUAlias(sku.SupplierSKU)
}

func pricingCacheReviewSKUKey(item SKUPriceReview) string {
	if key := pricingCacheSKUAlias(item.SupplierSKU); key != "" {
		return key
	}
	return pricingCacheSKUAlias(item.SupplierCode)
}

func pricingCacheSKUAlias(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return ""
	}
	if index := strings.Index(value, "-V"); index > 0 {
		return strings.TrimSpace(value[:index])
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
