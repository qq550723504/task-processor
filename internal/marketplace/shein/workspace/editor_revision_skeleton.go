package workspace

import (
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func BuildEditorRevisionSkeleton(
	pkg *sheinpub.Package,
	categoryPatch *CategoryResolutionPatch,
	attributePatch *AttributeResolutionPatch,
	salePatch *SaleAttributeResolutionPatch,
	skcPatches []SKCRevisionPatch,
) *EditorRevisionSkeleton {
	if pkg == nil {
		return nil
	}

	return &EditorRevisionSkeleton{
		Platform: "shein",
		Actor:    "desktop-client",
		Reason:   "manual adjustment",
		Shein: &RevisionInput{
			SpuName:                 stringPointerOrNil(pkg.SpuName),
			ProductNameEn:           stringPointerOrNil(pkg.ProductNameEn),
			BrandName:               stringPointerOrNil(pkg.BrandName),
			Description:             stringPointerOrNil(pkg.Description),
			Images:                  cloneImageSet(pkg.Images),
			ProductAttributes:       append([]common.Attribute(nil), pkg.ProductAttributes...),
			ResolvedAttributes:      append([]sheinpub.ResolvedAttribute(nil), pkg.ResolvedAttributes...),
			CategoryResolution:      cloneCategoryPatch(categoryPatch),
			AttributeResolution:     cloneAttributePatch(attributePatch),
			SaleAttributeResolution: cloneSalePatch(salePatch),
			SKCPatches:              cloneSKCPatches(skcPatches),
			ReviewNotes:             append([]string(nil), pkg.ReviewNotes...),
		},
	}
}

func cloneImageSet(set *common.ImageSet) *common.ImageSet {
	return common.CloneImageSet(set)
}

func cloneCategoryPatch(src *CategoryResolutionPatch) *CategoryResolutionPatch {
	if src == nil {
		return nil
	}
	out := *src
	out.MatchedPath = append([]string(nil), src.MatchedPath...)
	out.CategoryIDList = append([]int(nil), src.CategoryIDList...)
	out.ReviewNotes = append([]string(nil), src.ReviewNotes...)
	return &out
}

func CloneCategoryResolutionPatch(src *CategoryResolutionPatch) *CategoryResolutionPatch {
	return cloneCategoryPatch(src)
}

func cloneAttributePatch(src *AttributeResolutionPatch) *AttributeResolutionPatch {
	if src == nil {
		return nil
	}
	out := *src
	out.ResolvedAttributes = append([]sheinpub.ResolvedAttribute(nil), src.ResolvedAttributes...)
	out.PendingAttributes = append([]common.Attribute(nil), src.PendingAttributes...)
	out.PendingAttributeCandidates = clonePendingAttributeCandidates(src.PendingAttributeCandidates)
	out.RecommendedAttributeCandidates = clonePendingAttributeCandidates(src.RecommendedAttributeCandidates)
	out.ReviewNotes = append([]string(nil), src.ReviewNotes...)
	return &out
}

func CloneAttributeResolutionPatch(src *AttributeResolutionPatch) *AttributeResolutionPatch {
	return cloneAttributePatch(src)
}

func cloneSalePatch(src *SaleAttributeResolutionPatch) *SaleAttributeResolutionPatch {
	if src == nil {
		return nil
	}
	out := *src
	out.PrimarySourceDimension = cloneStringPointer(src.PrimarySourceDimension)
	out.SecondarySourceDimension = cloneStringPointer(src.SecondarySourceDimension)
	out.SKCAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), src.SKCAttributes...)
	out.SKUAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), src.SKUAttributes...)
	out.SKCValueAssignments = cloneResolvedSaleAttributeMap(src.SKCValueAssignments)
	out.SKUValueAssignments = cloneResolvedSaleAttributeMap(src.SKUValueAssignments)
	out.CustomAttributeRelation = append(out.CustomAttributeRelation[:0:0], src.CustomAttributeRelation...)
	out.SelectionSummary = append([]string(nil), src.SelectionSummary...)
	out.ReviewNotes = append([]string(nil), src.ReviewNotes...)
	return &out
}

func CloneSaleAttributeResolutionPatch(src *SaleAttributeResolutionPatch) *SaleAttributeResolutionPatch {
	return cloneSalePatch(src)
}

func cloneSKCPatches(items []SKCRevisionPatch) []SKCRevisionPatch {
	if len(items) == 0 {
		return nil
	}
	out := make([]SKCRevisionPatch, 0, len(items))
	for _, item := range items {
		patch := item
		patch.SkcName = cloneStringPointer(item.SkcName)
		patch.SaleName = cloneStringPointer(item.SaleName)
		patch.MainImageURL = cloneStringPointer(item.MainImageURL)
		if item.SaleAttribute != nil {
			attr := *item.SaleAttribute
			patch.SaleAttribute = &attr
		}
		patch.SKUPatches = cloneSKUPatches(item.SKUPatches)
		out = append(out, patch)
	}
	return out
}

func CloneSKCRevisionPatches(items []SKCRevisionPatch) []SKCRevisionPatch {
	return cloneSKCPatches(items)
}

func cloneSKUPatches(items []SKURevisionPatch) []SKURevisionPatch {
	if len(items) == 0 {
		return nil
	}
	out := make([]SKURevisionPatch, 0, len(items))
	for _, item := range items {
		patch := item
		patch.Attributes = cloneMap(item.Attributes)
		patch.BasePrice = cloneStringPointer(item.BasePrice)
		patch.CostPrice = cloneStringPointer(item.CostPrice)
		patch.Currency = cloneStringPointer(item.Currency)
		patch.StockCount = cloneNonNegativeIntPointer(item.StockCount)
		patch.MainImage = cloneStringPointer(item.MainImage)
		patch.Barcode = cloneStringPointer(item.Barcode)
		patch.SaleAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), item.SaleAttributes...)
		patch.SitePriceList = append([]sheinpub.SitePrice(nil), item.SitePriceList...)
		patch.StockInfoList = append([]sheinpub.StockInfo(nil), item.StockInfoList...)
		out = append(out, patch)
	}
	return out
}

func stringPointerOrNil(value string) *string {
	if value == "" {
		return nil
	}
	out := value
	return &out
}

func cloneStringPointer(in *string) *string {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneNonNegativeIntPointer(in *int) *int {
	if in == nil || *in < 0 {
		return nil
	}
	out := *in
	return &out
}
