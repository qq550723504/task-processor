package listingkit

import "fmt"

func buildRevisionHistoryRestoreDraft(record *ListingKitRevisionRecord) *SheinEditorRevisionSkeleton {
	if record == nil {
		return nil
	}
	switch record.Platform {
	case "shein":
		return buildSheinRestoreDraft(record)
	default:
		return nil
	}
}

func buildSheinRestoreDraft(record *ListingKitRevisionRecord) *SheinEditorRevisionSkeleton {
	if record == nil || record.EditorContext == nil {
		return nil
	}
	if record.EditorContext.RevisionSkeleton != nil {
		restore := cloneSheinEditorRevisionSkeleton(record.EditorContext.RevisionSkeleton)
		restore.Actor = "desktop-client"
		restore.Reason = buildRevisionHistoryRestoreReason(record)
		return restore
	}

	restore := &SheinEditorRevisionSkeleton{
		Platform: "shein",
		Actor:    "desktop-client",
		Reason:   buildRevisionHistoryRestoreReason(record),
		Shein:    buildSheinRevisionInputFromEditorContext(record.EditorContext),
	}
	if restore.Shein == nil {
		return nil
	}
	return restore
}

func buildRevisionHistoryRestoreReason(record *ListingKitRevisionRecord) string {
	if record == nil {
		return "restore from revision history"
	}
	if record.Reason != "" {
		return fmt.Sprintf("restore: %s", record.Reason)
	}
	return "restore from revision history"
}

func cloneSheinEditorRevisionSkeleton(src *SheinEditorRevisionSkeleton) *SheinEditorRevisionSkeleton {
	if src == nil {
		return nil
	}
	cloned := &SheinEditorRevisionSkeleton{
		Platform: src.Platform,
		Actor:    src.Actor,
		Reason:   src.Reason,
	}
	if src.Shein != nil {
		cloned.Shein = cloneHistorySheinRevisionInput(src.Shein)
	}
	return cloned
}

func cloneHistorySheinRevisionInput(src *SheinRevisionInput) *SheinRevisionInput {
	if src == nil {
		return nil
	}
	cloned := &SheinRevisionInput{
		SpuName:                 cloneHistoryStringPointer(src.SpuName),
		ProductNameEn:           cloneHistoryStringPointer(src.ProductNameEn),
		BrandName:               cloneHistoryStringPointer(src.BrandName),
		Description:             cloneHistoryStringPointer(src.Description),
		SellingPoints:           append([]string(nil), src.SellingPoints...),
		CategoryName:            cloneHistoryStringPointer(src.CategoryName),
		CategoryPath:            append([]string(nil), src.CategoryPath...),
		CategoryID:              cloneHistoryIntPointer(src.CategoryID),
		CategoryIDList:          append([]int(nil), src.CategoryIDList...),
		ProductTypeID:           cloneHistoryIntPointer(src.ProductTypeID),
		TopCategoryID:           cloneHistoryIntPointer(src.TopCategoryID),
		Images:                  clonePlatformImageSetForEditor(src.Images),
		ProductAttributes:       append([]PlatformAttribute(nil), src.ProductAttributes...),
		ResolvedAttributes:      append([]SheinResolvedAttribute(nil), src.ResolvedAttributes...),
		CategoryResolution:      cloneHistorySheinCategoryResolutionPatch(src.CategoryResolution),
		AttributeResolution:     cloneHistorySheinAttributeResolutionPatch(src.AttributeResolution),
		SaleAttributeResolution: cloneHistorySheinSaleAttributeResolutionPatch(src.SaleAttributeResolution),
		SKCPatches:              cloneHistorySheinSKCRevisionPatches(src.SKCPatches),
		RequestDraft:            cloneHistorySheinRequestDraft(src.RequestDraft),
		ReviewNotes:             append([]string(nil), src.ReviewNotes...),
	}
	return cloned
}

func cloneHistoryStringPointer(src *string) *string {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}

func cloneHistoryIntPointer(src *int) *int {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}

func cloneHistorySheinCategoryResolutionPatch(src *SheinCategoryResolutionPatch) *SheinCategoryResolutionPatch {
	if src == nil {
		return nil
	}
	return &SheinCategoryResolutionPatch{
		Status:         cloneHistoryStringPointer(src.Status),
		Source:         cloneHistoryStringPointer(src.Source),
		QueryText:      cloneHistoryStringPointer(src.QueryText),
		MatchedPath:    append([]string(nil), src.MatchedPath...),
		CategoryID:     cloneHistoryIntPointer(src.CategoryID),
		CategoryIDList: append([]int(nil), src.CategoryIDList...),
		ProductTypeID:  cloneHistoryIntPointer(src.ProductTypeID),
		TopCategoryID:  cloneHistoryIntPointer(src.TopCategoryID),
		ReviewNotes:    append([]string(nil), src.ReviewNotes...),
	}
}

func cloneHistorySheinAttributeResolutionPatch(src *SheinAttributeResolutionPatch) *SheinAttributeResolutionPatch {
	if src == nil {
		return nil
	}
	return &SheinAttributeResolutionPatch{
		Status:             cloneHistoryStringPointer(src.Status),
		Source:             cloneHistoryStringPointer(src.Source),
		CategoryID:         cloneHistoryIntPointer(src.CategoryID),
		TemplateCount:      cloneHistoryIntPointer(src.TemplateCount),
		ResolvedCount:      cloneHistoryIntPointer(src.ResolvedCount),
		UnresolvedCount:    cloneHistoryIntPointer(src.UnresolvedCount),
		ResolvedAttributes: append([]SheinResolvedAttribute(nil), src.ResolvedAttributes...),
		ReviewNotes:        append([]string(nil), src.ReviewNotes...),
	}
}

func cloneHistorySheinSaleAttributeResolutionPatch(src *SheinSaleAttributeResolutionPatch) *SheinSaleAttributeResolutionPatch {
	if src == nil {
		return nil
	}
	return &SheinSaleAttributeResolutionPatch{
		Status:               cloneHistoryStringPointer(src.Status),
		Source:               cloneHistoryStringPointer(src.Source),
		PrimaryAttributeID:   cloneHistoryIntPointer(src.PrimaryAttributeID),
		SecondaryAttributeID: cloneHistoryIntPointer(src.SecondaryAttributeID),
		SKCAttributes:        append([]SheinResolvedSaleAttribute(nil), src.SKCAttributes...),
		SKUAttributes:        append([]SheinResolvedSaleAttribute(nil), src.SKUAttributes...),
		SelectionSummary:     append([]string(nil), src.SelectionSummary...),
		ReviewNotes:          append([]string(nil), src.ReviewNotes...),
	}
}

func cloneHistorySheinSKCRevisionPatches(src []SheinSKCRevisionPatch) []SheinSKCRevisionPatch {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]SheinSKCRevisionPatch, 0, len(src))
	for _, patch := range src {
		item := SheinSKCRevisionPatch{
			SupplierCode: patch.SupplierCode,
			SkcName:      cloneHistoryStringPointer(patch.SkcName),
			SaleName:     cloneHistoryStringPointer(patch.SaleName),
			MainImageURL: cloneHistoryStringPointer(patch.MainImageURL),
			SKUPatches:   cloneHistorySheinSKURevisionPatches(patch.SKUPatches),
		}
		if patch.SaleAttribute != nil {
			attr := *patch.SaleAttribute
			item.SaleAttribute = &attr
		}
		cloned = append(cloned, item)
	}
	return cloned
}

func cloneHistorySheinSKURevisionPatches(src []SheinSKURevisionPatch) []SheinSKURevisionPatch {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]SheinSKURevisionPatch, 0, len(src))
	for _, patch := range src {
		item := SheinSKURevisionPatch{
			SupplierSKU:    patch.SupplierSKU,
			Attributes:     cloneMap(patch.Attributes),
			BasePrice:      cloneHistoryStringPointer(patch.BasePrice),
			CostPrice:      cloneHistoryStringPointer(patch.CostPrice),
			Currency:       cloneHistoryStringPointer(patch.Currency),
			StockCount:     cloneHistoryIntPointer(patch.StockCount),
			MainImage:      cloneHistoryStringPointer(patch.MainImage),
			Barcode:        cloneHistoryStringPointer(patch.Barcode),
			SaleAttributes: append([]SheinResolvedSaleAttribute(nil), patch.SaleAttributes...),
			SitePriceList:  append([]SheinSitePrice(nil), patch.SitePriceList...),
			StockInfoList:  append([]SheinStockInfo(nil), patch.StockInfoList...),
		}
		cloned = append(cloned, item)
	}
	return cloned
}

func cloneHistorySheinRequestDraft(src *SheinRequestDraft) *SheinRequestDraft {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.MultiLanguageNameList = append([]LocalizedText(nil), src.MultiLanguageNameList...)
	cloned.MultiLanguageDescList = append([]LocalizedText(nil), src.MultiLanguageDescList...)
	cloned.ProductAttributeList = append([]PlatformAttribute(nil), src.ProductAttributeList...)
	cloned.ResolvedAttributes = append([]SheinResolvedAttribute(nil), src.ResolvedAttributes...)
	cloned.SiteList = append([]PlatformSite(nil), src.SiteList...)
	cloned.SKCList = cloneHistorySheinSKCRequestDrafts(src.SKCList)
	if src.ImageInfo != nil {
		imageInfo := *src.ImageInfo
		imageInfo.Gallery = append([]string(nil), src.ImageInfo.Gallery...)
		imageInfo.Source = append([]string(nil), src.ImageInfo.Source...)
		cloned.ImageInfo = &imageInfo
	}
	return &cloned
}

func cloneHistorySheinSKCRequestDrafts(src []SheinSKCRequestDraft) []SheinSKCRequestDraft {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]SheinSKCRequestDraft, 0, len(src))
	for _, skc := range src {
		item := skc
		item.MultiLanguageNameList = append([]LocalizedText(nil), skc.MultiLanguageNameList...)
		item.SKUList = cloneHistorySheinSKUDrafts(skc.SKUList)
		if skc.ImageInfo != nil {
			imageInfo := *skc.ImageInfo
			imageInfo.Gallery = append([]string(nil), skc.ImageInfo.Gallery...)
			imageInfo.Source = append([]string(nil), skc.ImageInfo.Source...)
			item.ImageInfo = &imageInfo
		}
		if skc.SaleAttribute != nil {
			attr := *skc.SaleAttribute
			item.SaleAttribute = &attr
		}
		cloned = append(cloned, item)
	}
	return cloned
}

func cloneHistorySheinSKUDrafts(src []SheinSKUDraft) []SheinSKUDraft {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]SheinSKUDraft, 0, len(src))
	for _, sku := range src {
		item := sku
		item.Attributes = cloneMap(sku.Attributes)
		item.SaleAttributes = append([]SheinResolvedSaleAttribute(nil), sku.SaleAttributes...)
		item.StockInfoList = append([]SheinStockInfo(nil), sku.StockInfoList...)
		item.SitePriceList = append([]SheinSitePrice(nil), sku.SitePriceList...)
		cloned = append(cloned, item)
	}
	return cloned
}

func buildSheinRevisionInputFromEditorContext(ctx *SheinEditorContext) *SheinRevisionInput {
	if ctx == nil {
		return nil
	}
	input := &SheinRevisionInput{}
	if ctx.Basics != nil {
		input.SpuName = stringPointerOrNil(ctx.Basics.SpuName)
		input.ProductNameEn = stringPointerOrNil(ctx.Basics.ProductNameEn)
		input.BrandName = stringPointerOrNil(ctx.Basics.BrandName)
		input.Description = stringPointerOrNil(ctx.Basics.Description)
		input.Images = clonePlatformImageSetForEditor(ctx.Basics.Images)
		input.ReviewNotes = append([]string(nil), ctx.Basics.ReviewNotes...)
	}
	if ctx.Category != nil {
		input.CategoryResolution = cloneHistorySheinCategoryResolutionPatch(ctx.Category.SuggestedPatch)
	}
	if ctx.Attributes != nil {
		input.AttributeResolution = cloneHistorySheinAttributeResolutionPatch(ctx.Attributes.SuggestedPatch)
	}
	if ctx.SaleAttributes != nil {
		input.SaleAttributeResolution = cloneHistorySheinSaleAttributeResolutionPatch(ctx.SaleAttributes.SuggestedResolutionPatch)
		input.SKCPatches = cloneHistorySheinSKCRevisionPatches(ctx.SaleAttributes.SuggestedSKCPatches)
	}
	if isEmptyHistorySheinRevisionInput(input) {
		return nil
	}
	return input
}

func isEmptyHistorySheinRevisionInput(input *SheinRevisionInput) bool {
	return input == nil ||
		(input.SpuName == nil &&
			input.ProductNameEn == nil &&
			input.BrandName == nil &&
			input.Description == nil &&
			len(input.SellingPoints) == 0 &&
			input.CategoryName == nil &&
			len(input.CategoryPath) == 0 &&
			input.CategoryID == nil &&
			len(input.CategoryIDList) == 0 &&
			input.ProductTypeID == nil &&
			input.TopCategoryID == nil &&
			input.Images == nil &&
			len(input.ProductAttributes) == 0 &&
			len(input.ResolvedAttributes) == 0 &&
			input.CategoryResolution == nil &&
			input.AttributeResolution == nil &&
			input.SaleAttributeResolution == nil &&
			len(input.SKCPatches) == 0 &&
			input.RequestDraft == nil &&
			len(input.ReviewNotes) == 0)
}
