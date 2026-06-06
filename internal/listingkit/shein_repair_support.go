package listingkit

import (
	listingworkspace "task-processor/internal/listingkit/workspace/shein"
	common "task-processor/internal/publishing/common"
)

type SheinRepairValidationPreview = listingworkspace.RepairValidationPreview[RevisionFieldError]

type SheinRepairPatchPayload struct {
	CategoryResolution      *SheinCategoryResolutionPatch      `json:"category_resolution,omitempty"`
	AttributeResolution     *SheinAttributeResolutionPatch     `json:"attribute_resolution,omitempty"`
	SaleAttributeResolution *SheinSaleAttributeResolutionPatch `json:"sale_attribute_resolution,omitempty"`
	SKCPatches              []SheinSKCRevisionPatch            `json:"skc_patches,omitempty"`
	Images                  *PlatformImageSet                  `json:"images,omitempty"`
	ReviewNotes             []string                           `json:"review_notes,omitempty"`
}

type sheinRepairRevisionBundle struct {
	input    *SheinRevisionInput
	skeleton *SheinEditorRevisionSkeleton
	request  *ApplyRevisionRequest
}

type sheinRepairArtifacts struct {
	patch      *SheinRepairPatchPayload
	skeleton   *SheinEditorRevisionSkeleton
	request    *ApplyRevisionRequest
	validation *SheinRepairValidationPreview
}

type sheinRepairClonedFields struct {
	categoryResolution      *SheinCategoryResolutionPatch
	attributeResolution     *SheinAttributeResolutionPatch
	saleAttributeResolution *SheinSaleAttributeResolutionPatch
	skcPatches              []SheinSKCRevisionPatch
	images                  *PlatformImageSet
	reviewNotes             []string
}

func cloneSheinRepairFields(payload *SheinRepairPatchPayload) sheinRepairClonedFields {
	if payload == nil {
		return sheinRepairClonedFields{}
	}
	return sheinRepairClonedFields{
		categoryResolution:      cloneSheinCategoryResolutionPatch(payload.CategoryResolution),
		attributeResolution:     cloneSheinAttributeResolutionPatch(payload.AttributeResolution),
		saleAttributeResolution: cloneSheinSaleAttributeResolutionPatch(payload.SaleAttributeResolution),
		skcPatches:              cloneSheinSKCRevisionPatches(payload.SKCPatches),
		images:                  clonePlatformImageSetForEditor(payload.Images),
		reviewNotes:             append([]string(nil), payload.ReviewNotes...),
	}
}

func cloneSheinRepairPatchPayload(payload *SheinRepairPatchPayload) *SheinRepairPatchPayload {
	if payload == nil {
		return nil
	}
	fields := cloneSheinRepairFields(payload)
	return &SheinRepairPatchPayload{
		CategoryResolution:      fields.categoryResolution,
		AttributeResolution:     fields.attributeResolution,
		SaleAttributeResolution: fields.saleAttributeResolution,
		SKCPatches:              fields.skcPatches,
		Images:                  fields.images,
		ReviewNotes:             fields.reviewNotes,
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
	cloned.PendingAttributes = append([]common.Attribute(nil), patch.PendingAttributes...)
	cloned.PendingAttributeCandidates = clonePendingAttributeCandidates(patch.PendingAttributeCandidates)
	cloned.RecommendedAttributeCandidates = clonePendingAttributeCandidates(patch.RecommendedAttributeCandidates)
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

func buildSheinRepairRevisionBundle(action string, payload *SheinRepairPatchPayload) sheinRepairRevisionBundle {
	input := buildSheinRepairRevisionInput(payload)
	if input == nil {
		return sheinRepairRevisionBundle{}
	}
	minimal := pruneSheinRevisionInput(input)
	if minimal == nil || isEmptySheinRevisionInput(minimal) {
		return sheinRepairRevisionBundle{}
	}
	skeleton := &SheinEditorRevisionSkeleton{
		Platform: "shein",
		Actor:    "desktop-client",
		Reason:   buildSheinRepairReason(action),
		Shein:    minimal,
	}
	return sheinRepairRevisionBundle{
		input:    input,
		skeleton: skeleton,
		request: &ApplyRevisionRequest{
			Platform: skeleton.Platform,
			Actor:    skeleton.Actor,
			Reason:   skeleton.Reason,
			Shein:    cloneHistorySheinRevisionInput(skeleton.Shein),
		},
	}
}

func buildSheinRepairRevisionSkeleton(action string, payload *SheinRepairPatchPayload) *SheinEditorRevisionSkeleton {
	return buildSheinRepairRevisionBundle(action, payload).skeleton
}

func buildSheinRepairApplyRequest(action string, payload *SheinRepairPatchPayload) *ApplyRevisionRequest {
	return buildSheinRepairRevisionBundle(action, payload).request
}

func buildSheinRepairRevisionInput(payload *SheinRepairPatchPayload) *SheinRevisionInput {
	if payload == nil {
		return nil
	}
	fields := cloneSheinRepairFields(payload)
	input := &SheinRevisionInput{
		CategoryResolution:      fields.categoryResolution,
		AttributeResolution:     fields.attributeResolution,
		SaleAttributeResolution: fields.saleAttributeResolution,
		SKCPatches:              fields.skcPatches,
		Images:                  fields.images,
		ReviewNotes:             fields.reviewNotes,
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

func buildSheinRepairArtifacts(pkg *SheinPackage, action string, editorSection string, patch *SheinRepairPatchPayload) sheinRepairArtifacts {
	bundle := buildSheinRepairRevisionBundle(action, patch)
	return sheinRepairArtifacts{
		patch:      cloneSheinRepairPatchPayload(patch),
		skeleton:   bundle.skeleton,
		request:    bundle.request,
		validation: buildSheinRepairValidationPreview(pkg, editorSection, bundle.request, bundle.skeleton),
	}
}

func cloneSheinRepairArtifacts(patch *SheinRepairPatchPayload, skeleton *SheinEditorRevisionSkeleton, request *ApplyRevisionRequest, validation *SheinRepairValidationPreview) sheinRepairArtifacts {
	return sheinRepairArtifacts{
		patch:      cloneSheinRepairPatchPayload(patch),
		skeleton:   cloneSheinEditorRevisionSkeleton(skeleton),
		request:    cloneApplyRevisionRequest(request),
		validation: cloneSheinRepairValidationPreview(validation),
	}
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
	return listingworkspace.BuildRepairValidationPreview(pkg, editorSection, skeleton, valid, fieldErrors)
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
