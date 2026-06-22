package shein

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"task-processor/internal/catalog/canonical"
	common "task-processor/internal/publishing/common"
)

func shortResolutionCacheKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 12 {
		return key
	}
	return key[:12]
}

func categoryResolverCacheKey(req *BuildRequest, canonical *canonical.Product, pkg *Package) string {
	categoryPath := normalizedCategoryCachePath(canonical, pkg)
	return categoryResolverCacheKeyForPath(req, categoryResolverIdentity(canonical, pkg, categoryPath), categoryPath)
}

func categoryResolverCacheKeyForPath(req *BuildRequest, identity []string, categoryPath []string) string {
	normalizedPath := append([]string(nil), categoryPath...)
	if len(normalizedPath) == 0 {
		normalizedPath = []string{}
	}
	payload := map[string]any{
		"version":              3,
		"store_id":             sheinStoreID(req),
		"target_category_hint": normalizeText(targetCategoryHint(req)),
		"category_path":        normalizedPath,
		"product_identity":     identity,
	}
	return hashCachePayload(payload)
}

func categoryResolverUnresolvedAliasKey(req *BuildRequest, canonical *canonical.Product, pkg *Package) string {
	if !canUseCategoryResolverUnresolvedAlias(canonical, pkg) {
		return ""
	}
	return categoryResolverCacheKeyForPath(req, categoryResolverIdentity(canonical, pkg, nil), nil)
}

func canUseCategoryResolverUnresolvedAlias(canonical *canonical.Product, pkg *Package) bool {
	return len(stableCanonicalSDSIdentifiers(canonical)) > 0 || len(stablePackageIdentifiers(pkg)) > 0
}

func categoryResolverIdentity(canonical *canonical.Product, pkg *Package, categoryPath []string) []string {
	if len(categoryPath) == 0 {
		if identifiers := stableCanonicalSDSIdentifiers(canonical); len(identifiers) > 0 {
			return identifiers
		}
		if identifiers := stablePackageIdentifiers(pkg); len(identifiers) > 0 {
			return identifiers
		}
	}
	return stableProductIdentity(canonical, pkg)
}

func targetCategoryHint(req *BuildRequest) string {
	if req == nil {
		return ""
	}
	return req.TargetCategoryHint
}

func attributeResolverCacheKey(req *BuildRequest, canonical *canonical.Product, pkg *Package) string {
	return attributeResolverCacheKeyWithIdentity(req, pkg, stableProductIdentity(canonical, pkg), pkgProductAttributes(pkg))
}

func attributeResolverCacheKeyWithIdentity(req *BuildRequest, pkg *Package, identity []string, productAttributes []common.Attribute) string {
	if pkg == nil || categoryID(pkg) == 0 {
		return ""
	}
	payload := map[string]any{
		"version":               14,
		"store_id":              sheinStoreID(req),
		"category_id":           categoryID(pkg),
		"category_id_list":      append([]int(nil), pkg.CategoryIDList...),
		"category_path":         normalizedSourceCategoryPath(nil, pkg),
		"product_identity":      identity,
		"product_attributes":    normalizedAttributeInputs(productAttributes),
		"supplemental_attrs":    normalizedStringMapInputs(pkg.Attributes),
		"structured_attr_hints": normalizedStructuredAttributeHints(productAttributes),
	}
	return hashCachePayload(payload)
}

func attributeResolverLegacyVariantOnlyCacheKeys(req *BuildRequest, canonical *canonical.Product, pkg *Package) []string {
	variantIDs := legacyVariantOnlySDSIdentifiers(canonical, pkg)
	if len(variantIDs) == 0 {
		return nil
	}
	legacyInputs := legacyVariantOnlyAttributeInputs(pkgProductAttributes(pkg))
	keys := make([]string, 0, len(variantIDs))
	seen := map[string]struct{}{}
	for _, variantID := range variantIDs {
		key := attributeResolverCacheKeyWithIdentity(req, pkg, normalizeStableIdentity([]string{variantID}), legacyInputs)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}

func legacyVariantOnlySDSIdentifiers(canonical *canonical.Product, pkg *Package) []string {
	values := []string{
		lookupAttributeValueInList(pkgProductAttributes(pkg), "variant_sku"),
		lookupAttributeValueInList(pkgProductAttributes(pkg), "source_sds_sku"),
	}
	if canonical != nil {
		if attr, ok := canonical.Attributes["variant_sku"]; ok {
			values = append(values, attr.Value)
		}
		for _, variant := range canonical.Variants {
			for _, key := range []string{"source_sds_sku", "variant_sku"} {
				if attr, ok := variant.Attributes[key]; ok {
					values = append(values, attr.Value)
				}
			}
		}
	}
	return normalizeStableIdentity(values)
}

func legacyVariantOnlyAttributeInputs(inputs []common.Attribute) []common.Attribute {
	if len(inputs) == 0 {
		return nil
	}
	result := make([]common.Attribute, 0, len(inputs))
	productSKU := normalizeText(lookupAttributeValueInList(inputs, "product_sku"))
	for _, item := range inputs {
		name := normalizeText(item.Name)
		value := normalizeText(item.Value)
		switch name {
		case "product_sku":
			continue
		case "sku":
			if productSKU == "" || value == productSKU {
				continue
			}
		}
		result = append(result, item)
	}
	return result
}

func pkgProductAttributes(pkg *Package) []common.Attribute {
	if pkg == nil {
		return nil
	}
	return pkg.ProductAttributes
}

func saleAttributeResolverCacheKey(req *BuildRequest, canonical *canonical.Product, pkg *Package) string {
	if categoryID(pkg) == 0 {
		return ""
	}
	payload := map[string]any{
		"version":            19,
		"store_id":           sheinStoreID(req),
		"category_id":        categoryID(pkg),
		"category_id_list":   append([]int(nil), pkg.CategoryIDList...),
		"source_dimensions":  normalizedSourceDimensions(canonical),
		"source_variant_ids": stableCanonicalSDSIdentifiers(canonical),
	}
	return hashCachePayload(payload)
}

func shouldCacheCategoryResolution(resolution *CategoryResolution) bool {
	return resolution != nil && resolution.CategoryID > 0 && strings.TrimSpace(resolution.Status) != "unresolved"
}

func shouldCacheAttributeResolution(resolution *AttributeResolution) bool {
	return resolution != nil && resolution.TemplateCount > 0 && resolution.ResolvedCount > 0
}

func shouldCacheSaleAttributeResolution(resolution *SaleAttributeResolution) bool {
	if resolution == nil {
		return false
	}
	return resolution.PrimaryAttributeID > 0 || resolution.SecondaryAttributeID > 0 || len(resolution.SKCAttributes) > 0 || len(resolution.SKUAttributes) > 0
}

func normalizedSourceDimensions(canonical *canonical.Product) []string {
	dimensions := saleAttributeSourceDimensions(buildSourceVariantDimensions(canonical, common.BuildVariants(canonical)))
	if len(dimensions) == 0 {
		return nil
	}
	result := make([]string, 0, len(dimensions))
	for _, dimension := range dimensions {
		name := normalizeText(dimension.Name)
		if name == "" {
			continue
		}
		values := make([]string, 0, len(dimension.Values))
		for _, value := range dimension.Values {
			value = normalizeText(value)
			if value != "" {
				values = append(values, value)
			}
		}
		sort.Strings(values)
		result = append(result, name+"="+strings.Join(values, "|"))
	}
	sort.Strings(result)
	return result
}

func normalizedSourceCategoryPath(canonical *canonical.Product, pkg *Package) []string {
	var path []string
	if canonical != nil && len(canonical.CategoryPath) > 0 {
		path = canonical.CategoryPath
	} else {
		path = resolveCategoryPath(canonical, pkg)
	}
	result := make([]string, 0, len(path))
	for _, item := range path {
		item = normalizeText(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func normalizedCategoryCachePath(canonical *canonical.Product, pkg *Package) []string {
	if canonical != nil && len(canonical.CategoryPath) > 0 {
		return normalizedCategoryPathItems(canonical.CategoryPath)
	}
	if pkg != nil && pkg.Metadata != nil {
		if raw := strings.TrimSpace(pkg.Metadata["source_category_path"]); raw != "" {
			return normalizedCategoryPathItems(strings.Split(raw, ">"))
		}
	}
	return nil
}

func normalizedCategoryPathItems(path []string) []string {
	result := make([]string, 0, len(path))
	for _, item := range path {
		item = normalizeText(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func stableProductIdentity(canonical *canonical.Product, pkg *Package) []string {
	return stablePackageIdentity(canonical, pkg)
}

func stableAttributeProductIdentity(pkg *Package) []string {
	return stablePackageAttributeIdentity(pkg)
}

func normalizedAttributeInputs(inputs []common.Attribute) []string {
	if len(inputs) == 0 {
		return nil
	}
	result := make([]string, 0, len(inputs))
	for _, item := range inputs {
		name := normalizeText(item.Name)
		value := normalizeText(item.Value)
		if name == "" || value == "" {
			continue
		}
		result = append(result, name+"="+value)
	}
	sort.Strings(result)
	return result
}

func normalizedStringMapInputs(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	inputs := make([]common.Attribute, 0, len(values))
	for name, value := range values {
		inputs = append(inputs, common.Attribute{Name: name, Value: value})
	}
	return normalizedAttributeInputs(inputs)
}

func normalizedStructuredAttributeHints(inputs []common.Attribute) []string {
	if len(inputs) == 0 {
		return nil
	}
	pool := newDisplayAttributeEvidencePoolFromInputs(inputs)
	if pool == nil {
		return nil
	}
	structured := pool.StructuredItems()
	if len(structured) == 0 {
		return nil
	}
	result := make([]string, 0, len(structured))
	for _, item := range structured {
		field := normalizeText(item.Field)
		value := normalizeText(item.RawValue)
		if field == "" || value == "" {
			continue
		}
		result = append(result, field+"="+value)
	}
	sort.Strings(result)
	return result
}

func lookupAttributeValueInList(inputs []common.Attribute, name string) string {
	name = normalizeText(name)
	for _, item := range inputs {
		if normalizeText(item.Name) == name {
			return item.Value
		}
	}
	return ""
}

func sheinStoreID(req *BuildRequest) string {
	if req == nil || req.SheinStoreID == 0 {
		return ""
	}
	return strconv.FormatInt(req.SheinStoreID, 10)
}

func hashCachePayload(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil || len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
