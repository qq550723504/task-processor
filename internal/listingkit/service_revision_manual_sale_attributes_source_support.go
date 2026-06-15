package listingkit

import (
	"fmt"
	"strings"

	sheinattribute "task-processor/internal/shein/api/attribute"
)

func manualSheinComparableSourceValues(sourceValue string) []string {
	sourceValue = strings.TrimSpace(sourceValue)
	if sourceValue == "" {
		return nil
	}
	candidates := []string{sourceValue}
	if matches := manualSheinParenContentPattern.FindAllStringSubmatch(sourceValue, -1); len(matches) > 0 {
		outer := strings.TrimSpace(manualSheinParenContentPattern.ReplaceAllString(sourceValue, " "))
		if outer != "" {
			candidates = append(candidates, outer)
		}
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			inner := strings.TrimSpace(match[1])
			if inner != "" {
				candidates = append(candidates, inner)
			}
		}
	}
	return manualSheinUniqueNonEmptyStrings(candidates)
}

func manualSheinUniqueNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

func lookupSheinSKCSourceValue(pkg *SheinPackage, supplierCode, dimension string) string {
	if pkg == nil || dimension == "" {
		return ""
	}
	for _, skc := range pkg.SkcList {
		if strings.TrimSpace(skc.SupplierCode) != strings.TrimSpace(supplierCode) {
			continue
		}
		return lookupSKUAttributeValue(skc.Attributes, dimension)
	}
	return ""
}

func lookupSheinSKUSourceValue(pkg *SheinPackage, supplierCode, supplierSKU, dimension string) string {
	if pkg == nil || dimension == "" {
		return ""
	}
	for _, skc := range pkg.SkcList {
		if strings.TrimSpace(skc.SupplierCode) != strings.TrimSpace(supplierCode) {
			continue
		}
		for _, sku := range skc.SKUs {
			if strings.TrimSpace(sku.SKU) != strings.TrimSpace(supplierSKU) {
				continue
			}
			return lookupSKUAttributeValue(sku.Attributes, dimension)
		}
	}
	return ""
}

func lookupSKUAttributeValue(attributes map[string]string, dimension string) string {
	if len(attributes) == 0 || strings.TrimSpace(dimension) == "" {
		return ""
	}
	return strings.TrimSpace(attributes[dimension])
}

func firstNonEmptyNonBlankString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func dedupeCustomAttributeRelations(relations []sheinattribute.CustomAttributeRelation) []sheinattribute.CustomAttributeRelation {
	if len(relations) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(relations))
	out := make([]sheinattribute.CustomAttributeRelation, 0, len(relations))
	for _, relation := range relations {
		key := fmt.Sprintf("%d:%d", relation.PreAttributeValueID, relation.AttributeValueID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, relation)
	}
	return out
}
