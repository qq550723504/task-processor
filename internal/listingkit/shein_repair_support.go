package listingkit

import sheinworkspace "task-processor/internal/workspace/shein"

type SheinRepairValidationPreview = sheinworkspace.RepairValidationPreview[RevisionFieldError]

type SheinRepairPatchPayload struct {
	CategoryResolution      *SheinCategoryResolutionPatch      `json:"category_resolution,omitempty"`
	AttributeResolution     *SheinAttributeResolutionPatch     `json:"attribute_resolution,omitempty"`
	SaleAttributeResolution *SheinSaleAttributeResolutionPatch `json:"sale_attribute_resolution,omitempty"`
	SKCPatches              []SheinSKCRevisionPatch            `json:"skc_patches,omitempty"`
	Images                  *PlatformImageSet                  `json:"images,omitempty"`
	ReviewNotes             []string                           `json:"review_notes,omitempty"`
}

func cloneSheinRepairPatchPayload(payload *SheinRepairPatchPayload) *SheinRepairPatchPayload {
	if payload == nil {
		return nil
	}
	return &SheinRepairPatchPayload{
		CategoryResolution:      cloneSheinCategoryResolutionPatch(payload.CategoryResolution),
		AttributeResolution:     cloneSheinAttributeResolutionPatch(payload.AttributeResolution),
		SaleAttributeResolution: cloneSheinSaleAttributeResolutionPatch(payload.SaleAttributeResolution),
		SKCPatches:              cloneSheinSKCRevisionPatches(payload.SKCPatches),
		Images:                  clonePlatformImageSetForEditor(payload.Images),
		ReviewNotes:             append([]string(nil), payload.ReviewNotes...),
	}
}

func cloneSheinCategoryResolutionPatch(patch *SheinCategoryResolutionPatch) *SheinCategoryResolutionPatch {
	if patch == nil {
		return nil
	}
	cloned := *patch
	cloned.MatchedPath = append([]string(nil), patch.MatchedPath...)
	cloned.CategoryIDList = append([]int(nil), patch.CategoryIDList...)
	cloned.ReviewNotes = append([]string(nil), patch.ReviewNotes...)
	return &cloned
}

func cloneSheinAttributeResolutionPatch(patch *SheinAttributeResolutionPatch) *SheinAttributeResolutionPatch {
	if patch == nil {
		return nil
	}
	cloned := *patch
	cloned.ResolvedAttributes = append([]SheinResolvedAttribute(nil), patch.ResolvedAttributes...)
	cloned.ReviewNotes = append([]string(nil), patch.ReviewNotes...)
	return &cloned
}

func cloneSheinSaleAttributeResolutionPatch(patch *SheinSaleAttributeResolutionPatch) *SheinSaleAttributeResolutionPatch {
	if patch == nil {
		return nil
	}
	cloned := *patch
	cloned.SKCAttributes = append([]SheinResolvedSaleAttribute(nil), patch.SKCAttributes...)
	cloned.SKUAttributes = append([]SheinResolvedSaleAttribute(nil), patch.SKUAttributes...)
	cloned.SelectionSummary = append([]string(nil), patch.SelectionSummary...)
	cloned.ReviewNotes = append([]string(nil), patch.ReviewNotes...)
	return &cloned
}

func cloneSheinSKCRevisionPatches(items []SheinSKCRevisionPatch) []SheinSKCRevisionPatch {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]SheinSKCRevisionPatch, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, SheinSKCRevisionPatch{
			SupplierCode:  item.SupplierCode,
			SkcName:       cloneRepairStringPointer(item.SkcName),
			SaleName:      cloneRepairStringPointer(item.SaleName),
			MainImageURL:  cloneRepairStringPointer(item.MainImageURL),
			SaleAttribute: cloneSheinResolvedSaleAttributePointer(item.SaleAttribute),
			SKUPatches:    cloneSheinSKURevisionPatches(item.SKUPatches),
		})
	}
	return cloned
}

func cloneSheinSKURevisionPatches(items []SheinSKURevisionPatch) []SheinSKURevisionPatch {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]SheinSKURevisionPatch, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, SheinSKURevisionPatch{
			SupplierSKU:    item.SupplierSKU,
			Attributes:     cloneMap(item.Attributes),
			BasePrice:      cloneRepairStringPointer(item.BasePrice),
			CostPrice:      cloneRepairStringPointer(item.CostPrice),
			Currency:       cloneRepairStringPointer(item.Currency),
			StockCount:     cloneRepairIntPointer(item.StockCount),
			MainImage:      cloneRepairStringPointer(item.MainImage),
			Barcode:        cloneRepairStringPointer(item.Barcode),
			SaleAttributes: append([]SheinResolvedSaleAttribute(nil), item.SaleAttributes...),
			SitePriceList:  append([]SheinSitePrice(nil), item.SitePriceList...),
			StockInfoList:  append([]SheinStockInfo(nil), item.StockInfoList...),
		})
	}
	return cloned
}

func cloneSheinResolvedSaleAttributePointer(attr *SheinResolvedSaleAttribute) *SheinResolvedSaleAttribute {
	if attr == nil {
		return nil
	}
	cloned := *attr
	return &cloned
}

func cloneRepairStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneRepairIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func buildSheinRepairRevisionSkeleton(action string, payload *SheinRepairPatchPayload) *SheinEditorRevisionSkeleton {
	input := buildSheinRepairRevisionInput(payload)
	if input == nil {
		return nil
	}
	minimal := pruneSheinRevisionInput(input)
	if minimal == nil || isEmptySheinRevisionInput(minimal) {
		return nil
	}
	return &SheinEditorRevisionSkeleton{
		Platform: "shein",
		Actor:    "desktop-client",
		Reason:   buildSheinRepairReason(action),
		Shein:    minimal,
	}
}

func buildSheinRepairApplyRequest(action string, payload *SheinRepairPatchPayload) *ApplyRevisionRequest {
	skeleton := buildSheinRepairRevisionSkeleton(action, payload)
	if skeleton == nil {
		return nil
	}
	return &ApplyRevisionRequest{
		Platform: skeleton.Platform,
		Actor:    skeleton.Actor,
		Reason:   skeleton.Reason,
		Shein:    cloneHistorySheinRevisionInput(skeleton.Shein),
	}
}

func buildSheinRepairRevisionInput(payload *SheinRepairPatchPayload) *SheinRevisionInput {
	if payload == nil {
		return nil
	}
	input := &SheinRevisionInput{
		CategoryResolution:      cloneSheinCategoryResolutionPatch(payload.CategoryResolution),
		AttributeResolution:     cloneSheinAttributeResolutionPatch(payload.AttributeResolution),
		SaleAttributeResolution: cloneSheinSaleAttributeResolutionPatch(payload.SaleAttributeResolution),
		SKCPatches:              cloneSheinSKCRevisionPatches(payload.SKCPatches),
		Images:                  clonePlatformImageSetForEditor(payload.Images),
		ReviewNotes:             append([]string(nil), payload.ReviewNotes...),
	}
	if isEmptySheinRevisionInput(input) {
		return nil
	}
	return input
}

func buildSheinRepairReason(action string) string {
	if action == "" {
		return "repair suggested issue"
	}
	return "repair: " + action
}

func buildSheinRepairValidationPreview(pkg *SheinPackage, editorSection string, revision *ApplyRevisionRequest, skeleton *SheinEditorRevisionSkeleton) *SheinRepairValidationPreview {
	if revision == nil || skeleton == nil || skeleton.Shein == nil {
		return nil
	}
	valid := true
	var fieldErrors []RevisionFieldError
	if validationErr, ok := validateApplyRevisionRequest(revision).(*RevisionValidationError); ok {
		valid = false
		fieldErrors = append([]RevisionFieldError(nil), validationErr.Fields...)
	}
	return sheinworkspace.BuildRepairValidationPreview(pkg, editorSection, skeleton, valid, fieldErrors)
}

func cloneSheinRepairValidationPreview(src *SheinRepairValidationPreview) *SheinRepairValidationPreview {
	if src == nil {
		return nil
	}
	return &SheinRepairValidationPreview{
		Valid:                       src.Valid,
		Status:                      src.Status,
		FieldErrors:                 append([]RevisionFieldError(nil), src.FieldErrors...),
		RevisionDiffPreview:         cloneRevisionDiffPreview(src.RevisionDiffPreview),
		AffectedSections:            append([]string(nil), src.AffectedSections...),
		CategoryPreviewEffects:      append([]SheinEditorEffect(nil), src.CategoryPreviewEffects...),
		AttributePreviewEffects:     append([]SheinEditorEffect(nil), src.AttributePreviewEffects...),
		SaleAttributePreviewEffects: append([]SheinEditorEffect(nil), src.SaleAttributePreviewEffects...),
	}
}

func cloneRevisionDiffPreview(src *RevisionDiffPreview) *RevisionDiffPreview {
	if src == nil {
		return nil
	}
	cloned := &RevisionDiffPreview{
		ChangeCount: src.ChangeCount,
	}
	if len(src.Changes) > 0 {
		cloned.Changes = append([]RevisionFieldChange(nil), src.Changes...)
	}
	return cloned
}

func safeRepairActionCount(center *SheinRepairCenter) int {
	if center == nil || center.Stats == nil {
		return 0
	}
	return center.Stats.TotalActions
}

func safeRepairDirectApplyCount(center *SheinRepairCenter) int {
	if center == nil || center.Stats == nil {
		return 0
	}
	return center.Stats.DirectApplyActions
}

func safeRepairPlanStatus(center *SheinRepairCenter) string {
	if center == nil || center.PrimaryPlan == nil {
		return ""
	}
	return center.PrimaryPlan.Status
}

func safeRepairSessionStatus(center *SheinRepairCenter) string {
	if center == nil || center.Session == nil {
		return ""
	}
	return center.Session.Status
}
