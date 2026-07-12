package shein

import (
	"fmt"
	"strings"
)

// ReconcilePublishedSaleAttributeResolution merges publish-confirmed per-value
// assignments into a cloned resolution suitable for reuse by later tasks.
func ReconcilePublishedSaleAttributeResolution(pkg *Package, resolution *SaleAttributeResolution) *SaleAttributeResolution {
	result := cloneSaleAttributeResolution(resolution)
	pkg = NormalizePackageSemanticFields(pkg)
	if result == nil || pkg == nil || pkg.DraftPayload == nil {
		return result
	}
	if result.SKCValueAssignments == nil {
		result.SKCValueAssignments = map[string]ResolvedSaleAttribute{}
	}
	if result.SKUValueAssignments == nil {
		result.SKUValueAssignments = map[string]ResolvedSaleAttribute{}
	}
	for _, skc := range pkg.DraftPayload.SKCList {
		mergePublishedSaleAssignment(result.SKCValueAssignments, map[string]string{result.PrimarySourceDimension: skc.SaleName}, result.PrimarySourceDimension, skc.SaleAttribute)
		for _, sku := range skc.SKUList {
			for i := range sku.SaleAttributes {
				assignment := sku.SaleAttributes[i]
				mergePublishedSaleAssignment(result.SKUValueAssignments, sku.Attributes, result.SecondarySourceDimension, &assignment)
			}
		}
	}
	result.skcValueAssignments = cloneResolvedSaleAttributeMap(result.SKCValueAssignments)
	result.skuValueAssignments = cloneResolvedSaleAttributeMap(result.SKUValueAssignments)
	return result
}

func mergePublishedSaleAssignment(target map[string]ResolvedSaleAttribute, source map[string]string, dimension string, assignment *ResolvedSaleAttribute) {
	if assignment == nil || assignment.AttributeID <= 0 || assignment.AttributeValueID == nil || *assignment.AttributeValueID <= 0 {
		return
	}
	sourceValue := sourceDimensionValue(source, dimension)
	if sourceValue == "" {
		return
	}
	target[normalizeText(sourceValue)] = *assignment
}

func sourceDimensionValue(attributes map[string]string, dimension string) string {
	want := normalizeText(dimension)
	for name, value := range attributes {
		if normalizeText(name) == want {
			return value
		}
	}
	return ""
}

// SaleAttributeResolutionApplicable verifies that every current source value
// has a concrete cached SHEIN value assignment.
func SaleAttributeResolutionApplicable(resolution *SaleAttributeResolution) (bool, string) {
	if resolution == nil || strings.TrimSpace(resolution.Status) != "resolved" {
		return false, "resolution is not resolved"
	}
	for _, dimension := range resolution.SourceDimensions {
		var scope string
		var assignments map[string]ResolvedSaleAttribute
		switch normalizeText(dimension.Name) {
		case normalizeText(resolution.PrimarySourceDimension):
			scope, assignments = "skc", resolution.SKCValueAssignments
		case normalizeText(resolution.SecondarySourceDimension):
			scope, assignments = "sku", resolution.SKUValueAssignments
		default:
			continue
		}
		for _, value := range dimension.Values {
			key := normalizeText(value)
			assignment, ok := assignments[key]
			if !ok || assignment.AttributeID <= 0 || assignment.AttributeValueID == nil || *assignment.AttributeValueID <= 0 {
				return false, fmt.Sprintf("missing %s assignment for %s=%s", scope, normalizeText(dimension.Name), key)
			}
		}
	}
	return true, ""
}
