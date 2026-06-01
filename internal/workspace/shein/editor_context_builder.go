package shein

import (
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func BuildEditorContext(pkg *sheinpub.Package) *EditorContext {
	if pkg == nil {
		return nil
	}

	context := &EditorContext{
		Basics: &EditorBasicsContext{
			SpuName:       pkg.SpuName,
			ProductNameEn: pkg.ProductNameEn,
			BrandName:     pkg.BrandName,
			Description:   pkg.Description,
			Images:        cloneImageSet(pkg.Images),
			ReviewNotes:   append([]string(nil), pkg.ReviewNotes...),
		},
		Category: &EditorCategoryContext{
			Current:        BuildCategoryPayload(pkg),
			SuggestedPatch: BuildCategoryResolutionPatch(pkg),
			Recommendation: BuildCategoryRecommendationMeta(pkg),
			PreviewEffects: BuildCategoryEffects(),
		},
		Attributes: &EditorAttributeContext{
			Current:        BuildAttributePayload(pkg),
			SuggestedPatch: BuildAttributeResolutionPatch(pkg),
			Recommendation: BuildAttributeRecommendationMeta(pkg),
			Suggestions:    BuildAttributeSuggestions(pkg),
			PreviewEffects: BuildAttributeEffects(),
		},
		SaleAttributes: &EditorSaleAttributeContext{
			Current:                  BuildSaleAttributePayload(pkg),
			SuggestedResolutionPatch: BuildSaleAttributeResolutionPatch(pkg),
			SuggestedSKCPatches:      BuildEditorSKCPatches(pkg),
			Recommendation:           BuildSaleRecommendationMeta(pkg),
			CandidateSuggestions:     BuildSaleCandidateSuggestions(pkg),
			PreviewEffects:           BuildSaleAttributeEffects(),
		},
	}
	context.RevisionSkeleton = BuildEditorRevisionSkeleton(
		pkg,
		context.Category.SuggestedPatch,
		context.Attributes.SuggestedPatch,
		context.SaleAttributes.SuggestedResolutionPatch,
		context.SaleAttributes.SuggestedSKCPatches,
	)
	context.DirtyHints = BuildEditorDirtyHints(pkg)
	context.Progress = BuildEditorProgress(pkg, 0)
	return context
}

func BuildCategoryResolutionPatch(pkg *sheinpub.Package) *CategoryResolutionPatch {
	if pkg == nil {
		return nil
	}
	patch := &CategoryResolutionPatch{
		MatchedPath:    append([]string(nil), pkg.CategoryPath...),
		CategoryIDList: append([]int(nil), pkg.CategoryIDList...),
	}
	if pkg.CategoryResolution != nil {
		if pkg.CategoryResolution.Status != "" {
			status := pkg.CategoryResolution.Status
			patch.Status = &status
		}
		if pkg.CategoryResolution.Source != "" {
			source := pkg.CategoryResolution.Source
			patch.Source = &source
		}
		if pkg.CategoryResolution.QueryText != "" {
			queryText := pkg.CategoryResolution.QueryText
			patch.QueryText = &queryText
		}
		if len(pkg.CategoryResolution.MatchedPath) > 0 {
			patch.MatchedPath = append([]string(nil), pkg.CategoryResolution.MatchedPath...)
		}
		patch.ReviewNotes = append([]string(nil), pkg.CategoryResolution.ReviewNotes...)
	}
	if pkg.CategoryID > 0 {
		categoryID := pkg.CategoryID
		patch.CategoryID = &categoryID
	}
	if pkg.ProductTypeID != nil {
		productTypeID := *pkg.ProductTypeID
		patch.ProductTypeID = &productTypeID
	}
	if pkg.TopCategoryID > 0 {
		topCategoryID := pkg.TopCategoryID
		patch.TopCategoryID = &topCategoryID
	}
	return patch
}

func BuildAttributeResolutionPatch(pkg *sheinpub.Package) *AttributeResolutionPatch {
	if pkg == nil {
		return nil
	}
	patch := &AttributeResolutionPatch{
		ResolvedAttributes: append([]sheinpub.ResolvedAttribute(nil), pkg.ResolvedAttributes...),
	}
	if pkg.AttributeResolution != nil {
		if pkg.AttributeResolution.Status != "" {
			status := pkg.AttributeResolution.Status
			patch.Status = &status
		}
		if pkg.AttributeResolution.Source != "" {
			source := pkg.AttributeResolution.Source
			patch.Source = &source
		}
		if pkg.AttributeResolution.CategoryID > 0 {
			categoryID := pkg.AttributeResolution.CategoryID
			patch.CategoryID = &categoryID
		}
		templateCount := pkg.AttributeResolution.TemplateCount
		patch.TemplateCount = &templateCount
		resolvedCount := pkg.AttributeResolution.ResolvedCount
		patch.ResolvedCount = &resolvedCount
		unresolvedCount := pkg.AttributeResolution.UnresolvedCount
		patch.UnresolvedCount = &unresolvedCount
		if len(pkg.AttributeResolution.ResolvedAttributes) > 0 {
			patch.ResolvedAttributes = append([]sheinpub.ResolvedAttribute(nil), pkg.AttributeResolution.ResolvedAttributes...)
		}
		patch.PendingAttributes = append([]common.Attribute(nil), pkg.AttributeResolution.PendingAttributes...)
		patch.PendingAttributeCandidates = clonePendingAttributeCandidates(pkg.AttributeResolution.PendingAttributeCandidates)
		patch.RecommendedAttributeCandidates = clonePendingAttributeCandidates(pkg.AttributeResolution.RecommendedAttributeCandidates)
		patch.ReviewNotes = append([]string(nil), pkg.AttributeResolution.ReviewNotes...)
	}
	return patch
}

func BuildSaleAttributeResolutionPatch(pkg *sheinpub.Package) *SaleAttributeResolutionPatch {
	if pkg == nil {
		return nil
	}
	patch := &SaleAttributeResolutionPatch{}
	if pkg.SaleAttributeResolution == nil {
		return patch
	}
	if pkg.SaleAttributeResolution.Status != "" {
		status := pkg.SaleAttributeResolution.Status
		patch.Status = &status
	}
	if pkg.SaleAttributeResolution.Source != "" {
		source := pkg.SaleAttributeResolution.Source
		patch.Source = &source
	}
	if pkg.SaleAttributeResolution.RecommendCategoryReview {
		recommend := pkg.SaleAttributeResolution.RecommendCategoryReview
		patch.RecommendCategoryReview = &recommend
	}
	if pkg.SaleAttributeResolution.CategoryReviewReason != "" {
		reason := pkg.SaleAttributeResolution.CategoryReviewReason
		patch.CategoryReviewReason = &reason
	}
	if pkg.SaleAttributeResolution.PrimaryAttributeID > 0 {
		primaryAttributeID := pkg.SaleAttributeResolution.PrimaryAttributeID
		patch.PrimaryAttributeID = &primaryAttributeID
	}
	if pkg.SaleAttributeResolution.SecondaryAttributeID > 0 {
		secondaryAttributeID := pkg.SaleAttributeResolution.SecondaryAttributeID
		patch.SecondaryAttributeID = &secondaryAttributeID
	}
	if pkg.SaleAttributeResolution.PrimarySourceDimension != "" {
		primarySourceDimension := pkg.SaleAttributeResolution.PrimarySourceDimension
		patch.PrimarySourceDimension = &primarySourceDimension
	}
	if pkg.SaleAttributeResolution.SecondarySourceDimension != "" {
		secondarySourceDimension := pkg.SaleAttributeResolution.SecondarySourceDimension
		patch.SecondarySourceDimension = &secondarySourceDimension
	}
	patch.SKCAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), pkg.SaleAttributeResolution.SKCAttributes...)
	patch.SKUAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), pkg.SaleAttributeResolution.SKUAttributes...)
	patch.SKCValueAssignments = cloneResolvedSaleAttributeMap(pkg.SaleAttributeResolution.SKCValueAssignments)
	patch.SKUValueAssignments = cloneResolvedSaleAttributeMap(pkg.SaleAttributeResolution.SKUValueAssignments)
	patch.CustomAttributeRelation = append(patch.CustomAttributeRelation, pkg.SaleAttributeResolution.CustomAttributeRelation...)
	patch.SelectionSummary = append([]string(nil), pkg.SaleAttributeResolution.SelectionSummary...)
	patch.ReviewNotes = append([]string(nil), pkg.SaleAttributeResolution.ReviewNotes...)
	if patch.Status != nil && *patch.Status == "resolved" && !IsSaleAttributeResolved(pkg) {
		status := "partial"
		patch.Status = &status
	}
	return patch
}

func clonePendingAttributeCandidates(items []sheinpub.PendingAttributeCandidate) []sheinpub.PendingAttributeCandidate {
	if len(items) == 0 {
		return nil
	}
	result := make([]sheinpub.PendingAttributeCandidate, 0, len(items))
	for _, item := range items {
		clone := item
		clone.AttributeValueList = append([]sheinpub.AttributeValueCandidate(nil), item.AttributeValueList...)
		result = append(result, clone)
	}
	return result
}

func BuildEditorSKCPatches(pkg *sheinpub.Package) []SKCRevisionPatch {
	if pkg == nil || pkg.RequestDraft == nil || len(pkg.RequestDraft.SKCList) == 0 {
		return nil
	}
	patches := make([]SKCRevisionPatch, 0, len(pkg.RequestDraft.SKCList))
	for _, skc := range pkg.RequestDraft.SKCList {
		patch := SKCRevisionPatch{
			SupplierCode: skc.SupplierCode,
			SKUPatches:   buildEditorSKUPatches(skc.SKUList),
		}
		if skc.SkcName != "" {
			skcName := skc.SkcName
			patch.SkcName = &skcName
		}
		if skc.SaleName != "" {
			saleName := skc.SaleName
			patch.SaleName = &saleName
		}
		if skc.ImageInfo != nil && skc.ImageInfo.MainImage != "" {
			mainImageURL := skc.ImageInfo.MainImage
			patch.MainImageURL = &mainImageURL
		}
		if skc.SaleAttribute != nil {
			attr := *skc.SaleAttribute
			patch.SaleAttribute = &attr
		}
		patches = append(patches, patch)
	}
	return patches
}

func buildEditorSKUPatches(items []sheinpub.SKUDraft) []SKURevisionPatch {
	if len(items) == 0 {
		return nil
	}
	patches := make([]SKURevisionPatch, 0, len(items))
	for _, sku := range items {
		patch := SKURevisionPatch{
			SupplierSKU:    sku.SupplierSKU,
			Attributes:     cloneMap(sku.Attributes),
			SaleAttributes: append([]sheinpub.ResolvedSaleAttribute(nil), sku.SaleAttributes...),
			SitePriceList:  append([]sheinpub.SitePrice(nil), sku.SitePriceList...),
			StockInfoList:  append([]sheinpub.StockInfo(nil), sku.StockInfoList...),
		}
		if sku.BasePrice != "" {
			basePrice := sku.BasePrice
			patch.BasePrice = &basePrice
		}
		if sku.CostPrice != "" {
			costPrice := sku.CostPrice
			patch.CostPrice = &costPrice
		}
		if sku.Currency != "" {
			currency := sku.Currency
			patch.Currency = &currency
		}
		stockCount := sku.StockCount
		patch.StockCount = &stockCount
		if sku.MainImage != "" {
			mainImage := sku.MainImage
			patch.MainImage = &mainImage
		}
		if sku.Barcode != "" {
			barcode := sku.Barcode
			patch.Barcode = &barcode
		}
		patches = append(patches, patch)
	}
	return patches
}
