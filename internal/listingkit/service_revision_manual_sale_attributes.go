package listingkit

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

var manualSheinParenContentPattern = regexp.MustCompile(`[(（]([^()（）]+)[)）]`)

func (s *service) resolveManualSheinSaleAttributeValueIDs(
	ctx context.Context,
	task *Task,
	req *ApplyRevisionRequest,
) error {
	if task == nil || task.Result == nil || task.Result.Shein == nil || req == nil || req.Shein == nil {
		return nil
	}
	backfillManualSheinSaleAttributeAssignments(task.Result.Shein, req.Shein)
	if !manualSheinSaleAttributesNeedRemoteResolution(req.Shein) {
		return nil
	}

	api, err := s.buildSheinAttributeAPI(ctx, task)
	if err != nil {
		return err
	}
	categoryID := manualSheinSaleAttributeCategoryID(task.Result.Shein, req.Shein)
	if categoryID <= 0 {
		return fmt.Errorf("shein category id is unavailable for manual sale attribute resolution")
	}
	templates, err := api.GetAttributeTemplates(categoryID)
	if err != nil {
		return err
	}
	attrByID := flattenSheinAttributeTemplatesByID(templates)
	if len(attrByID) == 0 {
		return fmt.Errorf("shein attribute templates are unavailable for manual sale attribute resolution")
	}

	relations, notes, err := resolveManualSheinSaleAttributeValueIDs(
		task.Result.Shein,
		req.Shein,
		api,
		categoryID,
		attrByID,
	)
	if err != nil {
		return err
	}
	if len(relations) > 0 {
		if req.Shein.SaleAttributeResolution == nil {
			req.Shein.SaleAttributeResolution = &SheinSaleAttributeResolutionPatch{}
		}
		req.Shein.SaleAttributeResolution.CustomAttributeRelation = append(
			req.Shein.SaleAttributeResolution.CustomAttributeRelation,
			relations...,
		)
	}
	if len(notes) > 0 {
		if req.Shein.SaleAttributeResolution == nil {
			req.Shein.SaleAttributeResolution = &SheinSaleAttributeResolutionPatch{}
		}
		req.Shein.SaleAttributeResolution.ReviewNotes = uniqueStrings(append(
			append([]string(nil), req.Shein.SaleAttributeResolution.ReviewNotes...),
			notes...,
		))
	}
	return nil
}

func resolveManualSheinSaleAttributeValueIDs(
	pkg *SheinPackage,
	req *SheinRevisionInput,
	api sheinpub.AttributeAPI,
	categoryID int,
	attrByID map[int]sheinattribute.AttributeInfo,
) ([]sheinattribute.CustomAttributeRelation, []string, error) {
	if pkg == nil || req == nil || api == nil {
		return nil, nil, nil
	}
	backfillManualSheinSaleAttributeAssignments(pkg, req)
	primaryDimension := ""
	secondaryDimension := ""
	if pkg.SaleAttributeResolution != nil {
		primaryDimension = strings.TrimSpace(pkg.SaleAttributeResolution.PrimarySourceDimension)
		secondaryDimension = strings.TrimSpace(pkg.SaleAttributeResolution.SecondarySourceDimension)
	}
	spuName := strings.TrimSpace(pkg.SpuName)
	if spuName == "" {
		spuName = strings.TrimSpace(pkg.ProductNameEn)
	}

	var relations []sheinattribute.CustomAttributeRelation
	var notes []string

	for patchIndex := range req.SKCPatches {
		patch := &req.SKCPatches[patchIndex]
		if patch.SaleAttribute != nil && patch.SaleAttribute.AttributeID > 0 && patch.SaleAttribute.AttributeValueID == nil {
			sourceValue := firstNonEmptyNonBlankString(
				patch.SaleAttribute.Value,
				lookupSheinSKCSourceValue(pkg, patch.SupplierCode, primaryDimension),
			)
			if sourceValue == "" {
				return nil, nil, fmt.Errorf("missing source value for shein skc %q", patch.SupplierCode)
			}
			attr, ok := attrByID[patch.SaleAttribute.AttributeID]
			if !ok {
				return nil, nil, fmt.Errorf("shein template attribute %d is unavailable", patch.SaleAttribute.AttributeID)
			}
			resolved, customRelations, resolveNotes, matched := sheinpub.ResolveSingleSaleAttributeValue(
				attr,
				primaryDimension,
				sourceValue,
				"skc",
				api,
				categoryID,
				spuName,
			)
			if !matched || resolved.AttributeValueID == nil || *resolved.AttributeValueID <= 0 {
				return nil, nil, fmt.Errorf("resolve shein skc sale attribute %q failed: %s", sourceValue, strings.Join(resolveNotes, "; "))
			}
			patch.SaleAttribute = &resolved
			relations = append(relations, customRelations...)
			notes = append(notes, resolveNotes...)
		}

		for skuIndex := range patch.SKUPatches {
			skuPatch := &patch.SKUPatches[skuIndex]
			if len(skuPatch.SaleAttributes) == 0 {
				secondaryAttributeID := manualSheinSecondaryAttributeID(req)
				sourceValue := firstNonEmptyNonBlankString(
					lookupSKUAttributeValue(skuPatch.Attributes, secondaryDimension),
					lookupSheinSKUSourceValue(pkg, patch.SupplierCode, skuPatch.SupplierSKU, secondaryDimension),
				)
				if secondaryAttributeID > 0 && sourceValue != "" {
					attrName := manualSheinAttributeName(attrByID[secondaryAttributeID], "Size")
					skuPatch.SaleAttributes = []SheinResolvedSaleAttribute{{
						Scope:       "sku",
						Name:        attrName,
						Value:       sourceValue,
						AttributeID: secondaryAttributeID,
					}}
				}
			}
			for attrIndex := range skuPatch.SaleAttributes {
				if skuPatch.SaleAttributes[attrIndex].AttributeID <= 0 || skuPatch.SaleAttributes[attrIndex].AttributeValueID != nil {
					continue
				}
				sourceValue := firstNonEmptyNonBlankString(
					skuPatch.SaleAttributes[attrIndex].Value,
					lookupSKUAttributeValue(skuPatch.Attributes, secondaryDimension),
					lookupSheinSKUSourceValue(pkg, patch.SupplierCode, skuPatch.SupplierSKU, secondaryDimension),
				)
				if sourceValue == "" {
					return nil, nil, fmt.Errorf("missing source value for shein sku %q", skuPatch.SupplierSKU)
				}
				attr, ok := attrByID[skuPatch.SaleAttributes[attrIndex].AttributeID]
				if !ok {
					return nil, nil, fmt.Errorf("shein template attribute %d is unavailable", skuPatch.SaleAttributes[attrIndex].AttributeID)
				}
				resolved, customRelations, resolveNotes, matched := resolveManualSheinSKUAttributeValueWithVariants(
					attr,
					secondaryDimension,
					sourceValue,
					api,
					categoryID,
					spuName,
				)
				if !matched || resolved.AttributeValueID == nil || *resolved.AttributeValueID <= 0 {
					return nil, nil, fmt.Errorf("resolve shein sku sale attribute %q failed: %s", sourceValue, strings.Join(resolveNotes, "; "))
				}
				skuPatch.SaleAttributes[attrIndex] = resolved
				relations = append(relations, customRelations...)
				notes = append(notes, resolveNotes...)
			}
		}
	}

	syncSheinManualSaleAttributeResolution(req)
	return dedupeCustomAttributeRelations(relations), uniqueStrings(notes), nil
}

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

func resolveManualSheinSKUAttributeValueWithVariants(
	attr sheinattribute.AttributeInfo,
	sourceDimension string,
	sourceValue string,
	api sheinpub.AttributeAPI,
	categoryID int,
	spuName string,
) (SheinResolvedSaleAttribute, []sheinattribute.CustomAttributeRelation, []string, bool) {
	var lastRelations []sheinattribute.CustomAttributeRelation
	var lastNotes []string
	for _, candidate := range manualSheinComparableSourceValues(sourceValue) {
		resolved, relations, notes, matched := sheinpub.ResolveSingleSaleAttributeValue(
			attr,
			sourceDimension,
			candidate,
			"sku",
			api,
			categoryID,
			spuName,
		)
		if matched && resolved.AttributeValueID != nil && *resolved.AttributeValueID > 0 {
			return resolved, relations, notes, true
		}
		lastRelations = relations
		lastNotes = notes
	}
	return SheinResolvedSaleAttribute{}, lastRelations, lastNotes, false
}

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
