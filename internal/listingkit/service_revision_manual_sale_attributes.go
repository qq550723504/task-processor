package listingkit

import (
	"context"
	"fmt"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheinclient "task-processor/internal/shein/client"
)

func (s *service) resolveManualSheinSaleAttributeValueIDs(
	ctx context.Context,
	task *Task,
	req *ApplyRevisionRequest,
) error {
	if task == nil || task.Result == nil || task.Result.Shein == nil || req == nil || req.Shein == nil {
		return nil
	}
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

func (s *service) buildSheinAttributeAPI(ctx context.Context, task *Task) (sheinpub.AttributeAPI, error) {
	storeID, err := s.resolveSheinStoreID(ctx, task)
	if err != nil || storeID <= 0 {
		return nil, fmt.Errorf("shein store id is unavailable for attribute resolution")
	}
	if s.sheinManagementClient == nil {
		return nil, fmt.Errorf("shein management client is unavailable for attribute resolution")
	}

	apiClient := sheinclient.NewAPIClient(storeID, s.sheinManagementClient)
	if !apiClient.HasCookies() {
		if err := apiClient.ForceRefreshCookies(); err != nil {
			return nil, fmt.Errorf("shein store cookies are unavailable for attribute resolution: %w", err)
		}
	}
	if !apiClient.HasCookies() {
		return nil, fmt.Errorf("shein store cookies are unavailable for attribute resolution")
	}

	baseAPI := sheinclient.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		storeID,
		apiClient.GetHTTPClient(),
	)
	baseAPI.SetAuthRefreshFunc(apiClient.ForceRefreshCookies)
	return sheinattribute.NewClient(baseAPI), nil
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
				resolved, customRelations, resolveNotes, matched := sheinpub.ResolveSingleSaleAttributeValue(
					attr,
					secondaryDimension,
					sourceValue,
					"sku",
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

func manualSheinSaleAttributesNeedRemoteResolution(req *SheinRevisionInput) bool {
	if req == nil {
		return false
	}
	for _, patch := range req.SKCPatches {
		if patch.SaleAttribute != nil && patch.SaleAttribute.AttributeID > 0 && patch.SaleAttribute.AttributeValueID == nil {
			return true
		}
		for _, skuPatch := range patch.SKUPatches {
			for _, attr := range skuPatch.SaleAttributes {
				if attr.AttributeID > 0 && attr.AttributeValueID == nil {
					return true
				}
			}
		}
	}
	return false
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
