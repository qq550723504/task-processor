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
	payload := map[string]any{
		"version":          2,
		"store_id":         sheinStoreID(req),
		"category_path":    normalizedSourceCategoryPath(canonical, pkg),
		"product_identity": stableProductIdentity(canonical, pkg),
	}
	return hashCachePayload(payload)
}

func attributeResolverCacheKey(req *BuildRequest, pkg *Package) string {
	if pkg == nil || categoryID(pkg) == 0 {
		return ""
	}
	payload := map[string]any{
		"version":            13,
		"store_id":           sheinStoreID(req),
		"category_id":        categoryID(pkg),
		"category_id_list":   append([]int(nil), pkg.CategoryIDList...),
		"product_attributes": normalizedAttributeInputs(buildAttributeInputs(pkg)),
	}
	return hashCachePayload(payload)
}

func saleAttributeResolverCacheKey(req *BuildRequest, canonical *canonical.Product, pkg *Package) string {
	if categoryID(pkg) == 0 {
		return ""
	}
	payload := map[string]any{
		"version":           9,
		"store_id":          sheinStoreID(req),
		"category_id":       categoryID(pkg),
		"category_id_list":  append([]int(nil), pkg.CategoryIDList...),
		"source_dimensions": normalizedSourceDimensions(canonical),
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
	dimensions := buildSourceVariantDimensions(canonical, common.BuildVariants(canonical))
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

func stableProductIdentity(canonical *canonical.Product, pkg *Package) []string {
	values := make([]string, 0, 6)
	if canonical != nil {
		values = append(values, canonical.Title)
	}
	if pkg != nil {
		values = append(values, pkg.SpuName, pkg.ProductNameEn, lookupAttributeValueInList(pkg.ProductAttributes, "sku"), lookupAttributeValueInList(pkg.ProductAttributes, "parent sku"))
	}
	for idx := range values {
		values[idx] = normalizeText(values[idx])
	}
	sort.Strings(values)
	out := values[:0]
	for _, value := range values {
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
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
