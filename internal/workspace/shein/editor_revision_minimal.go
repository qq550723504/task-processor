package shein

import (
	"strings"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func stringPointerOrNil(value string) *string {
	if value == "" {
		return nil
	}
	copied := value
	return &copied
}

func BuildMinimalRevisionSkeleton(full *EditorRevisionSkeleton) *EditorRevisionSkeleton {
	if full == nil || full.Shein == nil {
		return full
	}
	return &EditorRevisionSkeleton{
		Platform: full.Platform,
		Actor:    full.Actor,
		Reason:   full.Reason,
		Shein:    PruneRevisionInput(full.Shein),
	}
}

func PruneRevisionInput(input *RevisionInput) *RevisionInput {
	if input == nil {
		return nil
	}
	out := &RevisionInput{}
	out.SpuName = cloneStringPointer(input.SpuName)
	out.ProductNameEn = cloneStringPointer(input.ProductNameEn)
	out.BrandName = cloneStringPointer(input.BrandName)
	out.Description = cloneStringPointer(input.Description)
	if len(input.SellingPoints) > 0 {
		out.SellingPoints = append([]string(nil), input.SellingPoints...)
	}
	out.CategoryName = cloneStringPointer(input.CategoryName)
	if len(input.CategoryPath) > 0 {
		out.CategoryPath = append([]string(nil), input.CategoryPath...)
	}
	out.CategoryID = clonePositiveIntPointer(input.CategoryID)
	if len(input.CategoryIDList) > 0 {
		out.CategoryIDList = append([]int(nil), input.CategoryIDList...)
	}
	out.ProductTypeID = clonePositiveIntPointer(input.ProductTypeID)
	out.TopCategoryID = clonePositiveIntPointer(input.TopCategoryID)
	if input.Images != nil && hasImageContent(input.Images) {
		out.Images = cloneImageSet(input.Images)
	}
	if len(input.ProductAttributes) > 0 {
		out.ProductAttributes = append([]common.Attribute(nil), input.ProductAttributes...)
	}
	if len(input.ResolvedAttributes) > 0 {
		out.ResolvedAttributes = append([]sheinpub.ResolvedAttribute(nil), input.ResolvedAttributes...)
	}
	if patch := pruneCategoryResolutionPatch(input.CategoryResolution); patch != nil {
		out.CategoryResolution = patch
	}
	if patch := pruneAttributeResolutionPatch(input.AttributeResolution); patch != nil {
		out.AttributeResolution = patch
	}
	if patch := pruneSaleAttributeResolutionPatch(input.SaleAttributeResolution); patch != nil {
		out.SaleAttributeResolution = patch
	}
	if patches := pruneSKCRevisionPatches(input.SKCPatches); len(patches) > 0 {
		out.SKCPatches = patches
	}
	if input.RequestDraft != nil {
		out.RequestDraft = input.RequestDraft
	}
	if len(input.ReviewNotes) > 0 {
		out.ReviewNotes = append([]string(nil), input.ReviewNotes...)
	}
	if isEmptyRevisionInput(out) {
		return &RevisionInput{}
	}
	return out
}

func IsEmptyRevisionInput(input *RevisionInput) bool {
	return isEmptyRevisionInput(input)
}

func pruneCategoryResolutionPatch(patch *CategoryResolutionPatch) *CategoryResolutionPatch {
	if patch == nil {
		return nil
	}
	out := &CategoryResolutionPatch{
		Status:        cloneStringPointer(patch.Status),
		Source:        cloneStringPointer(patch.Source),
		QueryText:     cloneStringPointer(patch.QueryText),
		CategoryID:    clonePositiveIntPointer(patch.CategoryID),
		ProductTypeID: clonePositiveIntPointer(patch.ProductTypeID),
		TopCategoryID: clonePositiveIntPointer(patch.TopCategoryID),
	}
	if len(patch.MatchedPath) > 0 {
		out.MatchedPath = append([]string(nil), patch.MatchedPath...)
	}
	if len(patch.CategoryIDList) > 0 {
		out.CategoryIDList = append([]int(nil), patch.CategoryIDList...)
	}
	if len(patch.ReviewNotes) > 0 {
		out.ReviewNotes = append([]string(nil), patch.ReviewNotes...)
	}
	if isEmptyCategoryPatch(out) {
		return nil
	}
	return out
}

func pruneAttributeResolutionPatch(patch *AttributeResolutionPatch) *AttributeResolutionPatch {
	if patch == nil {
		return nil
	}
	out := &AttributeResolutionPatch{
		Status:          cloneStringPointer(patch.Status),
		Source:          cloneStringPointer(patch.Source),
		CategoryID:      clonePositiveIntPointer(patch.CategoryID),
		TemplateCount:   cloneNonNegativeIntPointer(patch.TemplateCount),
		ResolvedCount:   cloneNonNegativeIntPointer(patch.ResolvedCount),
		UnresolvedCount: cloneNonNegativeIntPointer(patch.UnresolvedCount),
	}
	if len(patch.ResolvedAttributes) > 0 {
		out.ResolvedAttributes = append([]sheinpub.ResolvedAttribute(nil), patch.ResolvedAttributes...)
	}
	if len(patch.ReviewNotes) > 0 {
		out.ReviewNotes = append([]string(nil), patch.ReviewNotes...)
	}
	if isEmptyAttributePatch(out) {
		return nil
	}
	return out
}

func pruneSaleAttributeResolutionPatch(patch *SaleAttributeResolutionPatch) *SaleAttributeResolutionPatch {
	if patch == nil {
		return nil
	}
	out := &SaleAttributeResolutionPatch{
		Status:                  cloneStringPointer(patch.Status),
		Source:                  cloneStringPointer(patch.Source),
		RecommendCategoryReview: cloneBoolPointer(patch.RecommendCategoryReview),
		CategoryReviewReason:    cloneStringPointer(patch.CategoryReviewReason),
		PrimaryAttributeID:      clonePositiveIntPointer(patch.PrimaryAttributeID),
		SecondaryAttributeID:    clonePositiveIntPointer(patch.SecondaryAttributeID),
	}
	if len(patch.SKCAttributes) > 0 {
		out.SKCAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), patch.SKCAttributes...)
	}
	if len(patch.SKUAttributes) > 0 {
		out.SKUAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), patch.SKUAttributes...)
	}
	if len(patch.CustomAttributeRelation) > 0 {
		out.CustomAttributeRelation = append(out.CustomAttributeRelation, patch.CustomAttributeRelation...)
	}
	if len(patch.SelectionSummary) > 0 {
		out.SelectionSummary = append([]string(nil), patch.SelectionSummary...)
	}
	if len(patch.ReviewNotes) > 0 {
		out.ReviewNotes = append([]string(nil), patch.ReviewNotes...)
	}
	if isEmptySalePatch(out) {
		return nil
	}
	return out
}

func pruneSKCRevisionPatches(items []SKCRevisionPatch) []SKCRevisionPatch {
	if len(items) == 0 {
		return nil
	}
	out := make([]SKCRevisionPatch, 0, len(items))
	for _, item := range items {
		patch := SKCRevisionPatch{
			SupplierCode: stringsTrim(item.SupplierCode),
			SkcName:      cloneStringPointer(item.SkcName),
			SaleName:     cloneStringPointer(item.SaleName),
			MainImageURL: cloneStringPointer(item.MainImageURL),
		}
		if item.SaleAttribute != nil {
			attr := *item.SaleAttribute
			patch.SaleAttribute = &attr
		}
		patch.SKUPatches = pruneSKURevisionPatches(item.SKUPatches)
		if patch.SupplierCode == "" && patch.SaleAttribute == nil && patch.SkcName == nil && patch.SaleName == nil && patch.MainImageURL == nil && len(patch.SKUPatches) == 0 {
			continue
		}
		out = append(out, patch)
	}
	return out
}

func pruneSKURevisionPatches(items []SKURevisionPatch) []SKURevisionPatch {
	if len(items) == 0 {
		return nil
	}
	out := make([]SKURevisionPatch, 0, len(items))
	for _, item := range items {
		patch := SKURevisionPatch{SupplierSKU: stringsTrim(item.SupplierSKU)}
		if len(item.Attributes) > 0 {
			patch.Attributes = cloneMap(item.Attributes)
		}
		patch.BasePrice = cloneStringPointer(item.BasePrice)
		patch.CostPrice = cloneStringPointer(item.CostPrice)
		patch.Currency = cloneStringPointer(item.Currency)
		patch.StockCount = cloneNonNegativeIntPointer(item.StockCount)
		patch.MainImage = cloneStringPointer(item.MainImage)
		patch.Barcode = cloneStringPointer(item.Barcode)
		if len(item.SaleAttributes) > 0 {
			patch.SaleAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), item.SaleAttributes...)
		}
		if len(item.SitePriceList) > 0 {
			patch.SitePriceList = append([]sheinpub.SitePrice(nil), item.SitePriceList...)
		}
		if len(item.StockInfoList) > 0 {
			patch.StockInfoList = append([]sheinpub.StockInfo(nil), item.StockInfoList...)
		}
		if patch.SupplierSKU == "" && len(patch.Attributes) == 0 && patch.BasePrice == nil && patch.CostPrice == nil && patch.Currency == nil && patch.StockCount == nil && patch.MainImage == nil && patch.Barcode == nil && len(patch.SaleAttributes) == 0 && len(patch.SitePriceList) == 0 && len(patch.StockInfoList) == 0 {
			continue
		}
		out = append(out, patch)
	}
	return out
}

func hasImageContent(images *common.ImageSet) bool {
	return images != nil && (images.MainImage != "" || images.WhiteBgImage != "" || len(images.Gallery) > 0 || len(images.SourceImages) > 0)
}

func cloneStringPointer(in *string) *string {
	if in == nil || stringsTrim(*in) == "" {
		return nil
	}
	value := stringsTrim(*in)
	return &value
}

func clonePositiveIntPointer(in *int) *int {
	if in == nil || *in <= 0 {
		return nil
	}
	value := *in
	return &value
}

func cloneNonNegativeIntPointer(in *int) *int {
	if in == nil || *in < 0 {
		return nil
	}
	value := *in
	return &value
}

func cloneBoolPointer(in *bool) *bool {
	if in == nil {
		return nil
	}
	value := *in
	return &value
}

func stringsTrim(v string) string {
	return strings.TrimSpace(v)
}

func isEmptyRevisionInput(in *RevisionInput) bool {
	return in == nil ||
		(in.SpuName == nil &&
			in.ProductNameEn == nil &&
			in.BrandName == nil &&
			in.Description == nil &&
			len(in.SellingPoints) == 0 &&
			in.CategoryName == nil &&
			len(in.CategoryPath) == 0 &&
			in.CategoryID == nil &&
			len(in.CategoryIDList) == 0 &&
			in.ProductTypeID == nil &&
			in.TopCategoryID == nil &&
			in.Images == nil &&
			len(in.ProductAttributes) == 0 &&
			len(in.ResolvedAttributes) == 0 &&
			in.CategoryResolution == nil &&
			in.AttributeResolution == nil &&
			in.SaleAttributeResolution == nil &&
			len(in.SKCPatches) == 0 &&
			in.RequestDraft == nil &&
			len(in.ReviewNotes) == 0)
}

func isEmptyCategoryPatch(in *CategoryResolutionPatch) bool {
	return in == nil ||
		(in.Status == nil &&
			in.Source == nil &&
			in.QueryText == nil &&
			len(in.MatchedPath) == 0 &&
			in.CategoryID == nil &&
			len(in.CategoryIDList) == 0 &&
			in.ProductTypeID == nil &&
			in.TopCategoryID == nil &&
			len(in.ReviewNotes) == 0)
}

func isEmptyAttributePatch(in *AttributeResolutionPatch) bool {
	return in == nil ||
		(in.Status == nil &&
			in.Source == nil &&
			in.CategoryID == nil &&
			in.TemplateCount == nil &&
			in.ResolvedCount == nil &&
			in.UnresolvedCount == nil &&
			len(in.ResolvedAttributes) == 0 &&
			len(in.ReviewNotes) == 0)
}

func isEmptySalePatch(in *SaleAttributeResolutionPatch) bool {
	return in == nil ||
		(in.Status == nil &&
			in.Source == nil &&
			in.RecommendCategoryReview == nil &&
			in.CategoryReviewReason == nil &&
			in.PrimaryAttributeID == nil &&
			in.SecondaryAttributeID == nil &&
			len(in.SKCAttributes) == 0 &&
			len(in.SKUAttributes) == 0 &&
			len(in.CustomAttributeRelation) == 0 &&
			len(in.SelectionSummary) == 0 &&
			len(in.ReviewNotes) == 0)
}
