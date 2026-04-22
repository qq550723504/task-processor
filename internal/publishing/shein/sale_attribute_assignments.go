package shein

import "strings"

func buildSKCAssignments(skcs []SKCPackage, candidateName string, index *templateIndex) map[string]ResolvedSaleAttribute {
	if len(skcs) == 0 || strings.TrimSpace(candidateName) == "" || index == nil {
		return nil
	}
	assignments := make(map[string]ResolvedSaleAttribute, len(skcs))
	for _, skc := range skcs {
		value := lookupAttributeValue(skc.Attributes, candidateName)
		if strings.TrimSpace(value) == "" {
			continue
		}
		resolved := toResolvedSaleAttribute(index.Match(candidateName, value), "skc")
		if resolved.AttributeID <= 0 {
			continue
		}
		assignments[skc.SupplierCode] = resolved
	}
	if len(assignments) == 0 {
		return nil
	}
	return assignments
}

func buildSKUAssignments(skcs []SKCPackage, candidateName string, index *templateIndex) map[string][]ResolvedSaleAttribute {
	if len(skcs) == 0 || strings.TrimSpace(candidateName) == "" || index == nil {
		return nil
	}
	assignments := make(map[string][]ResolvedSaleAttribute)
	for _, skc := range skcs {
		for _, sku := range skc.SKUs {
			value := lookupAttributeValue(sku.Attributes, candidateName)
			if strings.TrimSpace(value) == "" {
				continue
			}
			resolved := toResolvedSaleAttribute(index.Match(candidateName, value), "sku")
			if resolved.AttributeID <= 0 {
				continue
			}
			assignments[sku.SKU] = []ResolvedSaleAttribute{resolved}
		}
	}
	if len(assignments) == 0 {
		return nil
	}
	return assignments
}

func lookupAttributeValue(attributes map[string]string, candidateName string) string {
	if len(attributes) == 0 || strings.TrimSpace(candidateName) == "" {
		return ""
	}
	for key, value := range attributes {
		if matchesAnyName(key, []string{candidateName}) && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
