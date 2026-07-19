package shein

import (
	"sort"
	"strings"

	"task-processor/internal/catalog/canonical"
)

func stablePackageIdentity(canonical *canonical.Product, pkg *Package) []string {
	identifiers := stableCanonicalSDSIdentifiers(canonical)
	identifiers = append(identifiers, stablePackageIdentifiers(pkg)...)
	identifiers = normalizeStableIdentity(identifiers)
	if len(identifiers) > 0 {
		return identifiers
	}

	values := make([]string, 0, 6)
	if canonical != nil {
		values = append(values, canonical.Title)
	}
	if pkg != nil {
		values = append(values, pkg.SpuName, pkg.ProductNameEn, lookupAttributeValueInList(pkg.ProductAttributes, "sku"), lookupAttributeValueInList(pkg.ProductAttributes, "parent sku"))
	}
	return normalizeStableIdentity(values)
}

func stablePackageAttributeIdentity(pkg *Package) []string {
	identifiers := stablePackageIdentifiers(pkg)
	if len(identifiers) > 0 {
		return identifiers
	}

	values := make([]string, 0, 4)
	if pkg != nil {
		spuName := pkg.SpuName
		sku := lookupAttributeValueInList(pkg.ProductAttributes, "sku")
		parentSKU := lookupAttributeValueInList(pkg.ProductAttributes, "parent sku")
		values = append(values, spuName, sku, parentSKU)
		if strings.TrimSpace(spuName) == "" && strings.TrimSpace(sku) == "" && strings.TrimSpace(parentSKU) == "" {
			values = append(values, pkg.ProductNameEn)
		}
	}
	return normalizeStableIdentity(values)
}

func stableCanonicalSDSIdentifiers(canonical *canonical.Product) []string {
	if canonical == nil {
		return nil
	}
	values := make([]string, 0, len(canonical.Attributes)+len(canonical.Variants)*2)
	for _, key := range []string{"sku", "product_sku", "variant_sku"} {
		if attr, ok := canonical.Attributes[key]; ok {
			values = append(values, attr.Value)
		}
	}
	for _, variant := range canonical.Variants {
		if attr, ok := variant.Attributes["source_sds_sku"]; ok && strings.TrimSpace(attr.Value) != "" {
			values = append(values, attr.Value)
			continue
		}
		if attr, ok := variant.Attributes["variant_sku"]; ok {
			values = append(values, attr.Value)
		}
	}
	identifiers := normalizeStableSDSIdentity(values)
	if len(identifiers) > 0 {
		return identifiers
	}

	fallback := make([]string, 0, len(canonical.Variants)*2)
	for _, variant := range canonical.Variants {
		fallback = append(fallback, variant.SKU)
		if attr, ok := variant.Attributes["supplier_sku"]; ok {
			fallback = append(fallback, attr.Value)
		}
	}
	return normalizeStableIdentity(fallback)
}

func stablePackageIdentifiers(pkg *Package) []string {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return nil
	}
	primary := make([]string, 0, len(pkg.ProductAttributes)+4)
	primary = append(primary,
		lookupAttributeValueInList(pkg.ProductAttributes, "product_sku"),
		lookupAttributeValueInList(pkg.ProductAttributes, "variant_sku"),
		lookupAttributeValueInList(pkg.ProductAttributes, "source_sds_sku"),
		lookupAttributeValueInList(pkg.ProductAttributes, "sku"),
	)
	identifiers := normalizeStableSDSIdentity(primary)
	if len(identifiers) > 0 {
		return identifiers
	}

	requestSKCCount := 0
	if pkg.DraftPayload != nil {
		requestSKCCount = len(pkg.DraftPayload.SKCList)
	}
	secondary := make([]string, 0, len(pkg.SkcList)+requestSKCCount)
	for _, skc := range pkg.SkcList {
		for _, sku := range skc.SKUs {
			secondary = append(secondary, sku.Attributes["source_sds_sku"])
		}
	}
	if pkg.DraftPayload != nil {
		for _, skc := range pkg.DraftPayload.SKCList {
			for _, sku := range skc.SKUList {
				secondary = append(secondary, sku.Attributes["source_sds_sku"])
			}
		}
	}
	identifiers = normalizeStableSDSIdentity(secondary)
	if len(identifiers) > 0 {
		return identifiers
	}

	fallback := make([]string, 0, len(pkg.ProductAttributes)+len(pkg.SkcList)+requestSKCCount*2)
	fallback = append(fallback,
		lookupAttributeValueInList(pkg.ProductAttributes, "supplier_sku"),
		lookupAttributeValueInList(pkg.ProductAttributes, "parent sku"),
	)
	for _, skc := range pkg.SkcList {
		fallback = append(fallback, normalizedSourceLikeSKU(skc.SupplierCode))
		for _, sku := range skc.SKUs {
			fallback = append(fallback, normalizedSourceLikeSKU(sku.SKU))
		}
	}
	if pkg.DraftPayload != nil {
		for _, skc := range pkg.DraftPayload.SKCList {
			fallback = append(fallback, normalizedSourceLikeSKU(skc.SupplierCode))
			for _, sku := range skc.SKUList {
				fallback = append(fallback, normalizedSourceLikeSKU(sku.SupplierSKU))
			}
		}
	}
	return normalizeStableIdentity(fallback)
}

func normalizedSourceLikeSKU(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if index := strings.Index(value, "-v"); index > 0 {
		value = value[:index]
	}
	if index := strings.LastIndex(value, "-"); index > 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}

func normalizeStableIdentity(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeText(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func normalizeStableSDSIdentity(values []string) []string {
	expanded := make([]string, 0, len(values)*2)
	for _, value := range values {
		expanded = append(expanded, value)
		if family := deriveStableSDSFamilyIdentifier(value); family != "" {
			expanded = append(expanded, family)
		}
	}
	return normalizeStableIdentity(expanded)
}

func deriveStableSDSFamilyIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 10 || strings.ContainsAny(value, "-_/ ") {
		return ""
	}
	hasLetter := false
	for _, ch := range value {
		switch {
		case ch >= 'A' && ch <= 'Z':
			hasLetter = true
		case ch >= 'a' && ch <= 'z':
			hasLetter = true
		case ch >= '0' && ch <= '9':
		default:
			return ""
		}
	}
	if !hasLetter {
		return ""
	}
	for _, ch := range value[len(value)-3:] {
		if ch < '0' || ch > '9' {
			return ""
		}
	}
	return value[:len(value)-3]
}

func StablePricingPackageIdentity(pkg *Package) []string {
	if pkg == nil {
		return nil
	}
	identifiers := stablePackageIdentifiers(pkg)
	if len(identifiers) > 0 {
		return identifiers
	}
	values := []string{
		strings.TrimSpace(pkg.SpuName),
		strings.TrimSpace(pkg.ProductNameEn),
		strings.TrimSpace(lookupAttributeValueInList(pkg.ProductAttributes, "sku")),
		strings.TrimSpace(lookupAttributeValueInList(pkg.ProductAttributes, "parent sku")),
	}
	return normalizeStableIdentity(values)
}
