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
