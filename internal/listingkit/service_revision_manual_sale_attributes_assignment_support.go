package listingkit

import (
	"strings"

	sheinattribute "task-processor/internal/shein/api/attribute"
)

func backfillManualSheinSaleAttributeAssignments(pkg *SheinPackage, req *SheinRevisionInput) {
	if pkg == nil || req == nil || req.SaleAttributeResolution == nil {
		return
	}

	primaryDimension := firstNonEmptyNonBlankString(
		stringValue(req.SaleAttributeResolution.PrimarySourceDimension),
		pkgSaleAttributeSourceDimension(pkg, true),
	)
	secondaryDimension := firstNonEmptyNonBlankString(
		stringValue(req.SaleAttributeResolution.SecondarySourceDimension),
		pkgSaleAttributeSourceDimension(pkg, false),
	)
	skcAssignments := firstNonEmptyResolvedSaleAttributeMap(
		req.SaleAttributeResolution.SKCValueAssignments,
		pkgSaleAttributeAssignments(pkg, true),
	)
	skuAssignments := firstNonEmptyResolvedSaleAttributeMap(
		req.SaleAttributeResolution.SKUValueAssignments,
		pkgSaleAttributeAssignments(pkg, false),
	)

	changed := false
	for patchIndex := range req.SKCPatches {
		patch := &req.SKCPatches[patchIndex]
		if !resolvedSaleAttributeValueReady(patch.SaleAttribute) {
			sourceValue := firstNonEmptyNonBlankString(
				manualSheinSaleAttributeValue(patch.SaleAttribute),
				lookupSheinSKCSourceValue(pkg, patch.SupplierCode, primaryDimension),
			)
			if assigned, ok := resolveManualSheinSaleAttributeValueAssignment(skcAssignments, sourceValue); ok {
				assignedCopy := assigned
				patch.SaleAttribute = &assignedCopy
				changed = true
			}
		}

		for skuIndex := range patch.SKUPatches {
			skuPatch := &patch.SKUPatches[skuIndex]
			if len(skuPatch.SaleAttributes) == 0 {
				sourceValue := firstNonEmptyNonBlankString(
					lookupSKUAttributeValue(skuPatch.Attributes, secondaryDimension),
					lookupSheinSKUSourceValue(pkg, patch.SupplierCode, skuPatch.SupplierSKU, secondaryDimension),
				)
				if assigned, ok := resolveManualSheinSaleAttributeValueAssignment(skuAssignments, sourceValue); ok {
					skuPatch.SaleAttributes = []SheinResolvedSaleAttribute{assigned}
					changed = true
				}
				continue
			}
			for attrIndex := range skuPatch.SaleAttributes {
				if resolvedSaleAttributeValueReadyValue(skuPatch.SaleAttributes[attrIndex]) {
					continue
				}
				sourceValue := firstNonEmptyNonBlankString(
					skuPatch.SaleAttributes[attrIndex].Value,
					lookupSKUAttributeValue(skuPatch.Attributes, secondaryDimension),
					lookupSheinSKUSourceValue(pkg, patch.SupplierCode, skuPatch.SupplierSKU, secondaryDimension),
				)
				if assigned, ok := resolveManualSheinSaleAttributeValueAssignment(skuAssignments, sourceValue); ok {
					skuPatch.SaleAttributes[attrIndex] = assigned
					changed = true
				}
			}
		}
	}

	if changed {
		syncSheinManualSaleAttributeResolution(req)
	}
}

func syncSheinManualSaleAttributeResolution(req *SheinRevisionInput) {
	if req == nil || req.SaleAttributeResolution == nil {
		return
	}
	req.SaleAttributeResolution.SKCAttributes = nil
	req.SaleAttributeResolution.SKUAttributes = nil
	for _, patch := range req.SKCPatches {
		if patch.SaleAttribute != nil {
			req.SaleAttributeResolution.SKCAttributes = append(
				req.SaleAttributeResolution.SKCAttributes,
				*patch.SaleAttribute,
			)
		}
		for _, skuPatch := range patch.SKUPatches {
			if len(skuPatch.SaleAttributes) == 0 {
				continue
			}
			req.SaleAttributeResolution.SKUAttributes = append(
				req.SaleAttributeResolution.SKUAttributes,
				skuPatch.SaleAttributes[0],
			)
		}
	}
}

func pkgSaleAttributeSourceDimension(pkg *SheinPackage, primary bool) string {
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return ""
	}
	if primary {
		return strings.TrimSpace(pkg.SaleAttributeResolution.PrimarySourceDimension)
	}
	return strings.TrimSpace(pkg.SaleAttributeResolution.SecondarySourceDimension)
}

func pkgSaleAttributeAssignments(pkg *SheinPackage, skc bool) map[string]SheinResolvedSaleAttribute {
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return nil
	}
	if skc {
		return pkg.SaleAttributeResolution.SKCValueAssignments
	}
	return pkg.SaleAttributeResolution.SKUValueAssignments
}

func firstNonEmptyResolvedSaleAttributeMap(values ...map[string]SheinResolvedSaleAttribute) map[string]SheinResolvedSaleAttribute {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}

func resolveManualSheinSaleAttributeValueAssignment(assignments map[string]SheinResolvedSaleAttribute, sourceValue string) (SheinResolvedSaleAttribute, bool) {
	if len(assignments) == 0 || strings.TrimSpace(sourceValue) == "" {
		return SheinResolvedSaleAttribute{}, false
	}
	assigned, ok := assignments[normalizeManualSheinSaleAttributeText(sourceValue)]
	return assigned, ok
}

func normalizeManualSheinSaleAttributeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("_", " ", "-", " ", "/", " ")
	return strings.Join(strings.Fields(replacer.Replace(value)), " ")
}

func manualSheinSaleAttributeValue(attr *SheinResolvedSaleAttribute) string {
	if attr == nil {
		return ""
	}
	return strings.TrimSpace(attr.Value)
}

func resolvedSaleAttributeValueReady(attr *SheinResolvedSaleAttribute) bool {
	return attr != nil && resolvedSaleAttributeValueReadyValue(*attr)
}

func resolvedSaleAttributeValueReadyValue(attr SheinResolvedSaleAttribute) bool {
	return attr.AttributeID > 0 && attr.AttributeValueID != nil && *attr.AttributeValueID > 0
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func manualSheinSaleAttributesNeedRemoteResolution(req *SheinRevisionInput) bool {
	if req == nil {
		return false
	}
	secondaryAttributeID := manualSheinSecondaryAttributeID(req)
	for _, patch := range req.SKCPatches {
		if patch.SaleAttribute != nil && patch.SaleAttribute.AttributeID > 0 && patch.SaleAttribute.AttributeValueID == nil {
			return true
		}
		for _, skuPatch := range patch.SKUPatches {
			if len(skuPatch.SaleAttributes) == 0 && secondaryAttributeID > 0 {
				return true
			}
			for _, attr := range skuPatch.SaleAttributes {
				if attr.AttributeID > 0 && attr.AttributeValueID == nil {
					return true
				}
			}
		}
	}
	return false
}

func manualSheinSecondaryAttributeID(req *SheinRevisionInput) int {
	if req == nil || req.SaleAttributeResolution == nil || req.SaleAttributeResolution.SecondaryAttributeID == nil {
		return 0
	}
	return *req.SaleAttributeResolution.SecondaryAttributeID
}

func manualSheinAttributeName(attr sheinattribute.AttributeInfo, fallback string) string {
	if strings.TrimSpace(attr.AttributeNameEn) != "" {
		return strings.TrimSpace(attr.AttributeNameEn)
	}
	if strings.TrimSpace(attr.AttributeName) != "" {
		return strings.TrimSpace(attr.AttributeName)
	}
	return fallback
}

func manualSheinSaleAttributeCategoryID(pkg *SheinPackage, req *SheinRevisionInput) int {
	if req != nil {
		if req.CategoryResolution != nil && req.CategoryResolution.CategoryID != nil && *req.CategoryResolution.CategoryID > 0 {
			return *req.CategoryResolution.CategoryID
		}
		if req.CategoryID != nil && *req.CategoryID > 0 {
			return *req.CategoryID
		}
	}
	if pkg != nil && pkg.CategoryID > 0 {
		return pkg.CategoryID
	}
	return 0
}

func flattenSheinAttributeTemplatesByID(info *sheinattribute.AttributeTemplateInfo) map[int]sheinattribute.AttributeInfo {
	if info == nil {
		return nil
	}
	result := make(map[int]sheinattribute.AttributeInfo)
	for _, template := range info.Data {
		for _, attr := range template.AttributeInfos {
			if attr.AttributeID <= 0 {
				continue
			}
			result[attr.AttributeID] = attr
		}
	}
	return result
}
