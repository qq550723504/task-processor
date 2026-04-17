package listingkit

import "strings"

func buildSheinMinimalRevisionSkeleton(pkg *SheinPackage) *SheinEditorRevisionSkeleton {
	full := buildSheinEditorRevisionSkeleton(pkg)
	if full == nil || full.Shein == nil {
		return full
	}

	minimal := &SheinEditorRevisionSkeleton{
		Platform: full.Platform,
		Actor:    full.Actor,
		Reason:   full.Reason,
		Shein:    pruneSheinRevisionInput(full.Shein),
	}
	return minimal
}

func pruneSheinRevisionInput(input *SheinRevisionInput) *SheinRevisionInput {
	if input == nil {
		return nil
	}
	out := &SheinRevisionInput{}

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
	if input.Images != nil && hasPlatformImageContent(input.Images) {
		out.Images = clonePlatformImageSetForEditor(input.Images)
	}
	if len(input.ProductAttributes) > 0 {
		out.ProductAttributes = append([]PlatformAttribute(nil), input.ProductAttributes...)
	}
	if len(input.ResolvedAttributes) > 0 {
		out.ResolvedAttributes = append([]SheinResolvedAttribute(nil), input.ResolvedAttributes...)
	}
	if patch := pruneSheinCategoryResolutionPatch(input.CategoryResolution); patch != nil {
		out.CategoryResolution = patch
	}
	if patch := pruneSheinAttributeResolutionPatch(input.AttributeResolution); patch != nil {
		out.AttributeResolution = patch
	}
	if patch := pruneSheinSaleAttributeResolutionPatch(input.SaleAttributeResolution); patch != nil {
		out.SaleAttributeResolution = patch
	}
	if patches := pruneSheinSKCRevisionPatches(input.SKCPatches); len(patches) > 0 {
		out.SKCPatches = patches
	}
	if input.RequestDraft != nil {
		out.RequestDraft = input.RequestDraft
	}
	if len(input.ReviewNotes) > 0 {
		out.ReviewNotes = append([]string(nil), input.ReviewNotes...)
	}

	if isEmptySheinRevisionInput(out) {
		return &SheinRevisionInput{}
	}
	return out
}

func pruneSheinCategoryResolutionPatch(patch *SheinCategoryResolutionPatch) *SheinCategoryResolutionPatch {
	if patch == nil {
		return nil
	}
	out := &SheinCategoryResolutionPatch{
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

func pruneSheinAttributeResolutionPatch(patch *SheinAttributeResolutionPatch) *SheinAttributeResolutionPatch {
	if patch == nil {
		return nil
	}
	out := &SheinAttributeResolutionPatch{
		Status:          cloneStringPointer(patch.Status),
		Source:          cloneStringPointer(patch.Source),
		CategoryID:      clonePositiveIntPointer(patch.CategoryID),
		TemplateCount:   cloneNonNegativeIntPointer(patch.TemplateCount),
		ResolvedCount:   cloneNonNegativeIntPointer(patch.ResolvedCount),
		UnresolvedCount: cloneNonNegativeIntPointer(patch.UnresolvedCount),
	}
	if len(patch.ResolvedAttributes) > 0 {
		out.ResolvedAttributes = append([]SheinResolvedAttribute(nil), patch.ResolvedAttributes...)
	}
	if len(patch.ReviewNotes) > 0 {
		out.ReviewNotes = append([]string(nil), patch.ReviewNotes...)
	}
	if isEmptyAttributePatch(out) {
		return nil
	}
	return out
}

func pruneSheinSaleAttributeResolutionPatch(patch *SheinSaleAttributeResolutionPatch) *SheinSaleAttributeResolutionPatch {
	if patch == nil {
		return nil
	}
	out := &SheinSaleAttributeResolutionPatch{
		Status:               cloneStringPointer(patch.Status),
		Source:               cloneStringPointer(patch.Source),
		PrimaryAttributeID:   clonePositiveIntPointer(patch.PrimaryAttributeID),
		SecondaryAttributeID: clonePositiveIntPointer(patch.SecondaryAttributeID),
	}
	if len(patch.SKCAttributes) > 0 {
		out.SKCAttributes = append([]SheinResolvedSaleAttribute(nil), patch.SKCAttributes...)
	}
	if len(patch.SKUAttributes) > 0 {
		out.SKUAttributes = append([]SheinResolvedSaleAttribute(nil), patch.SKUAttributes...)
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

func pruneSheinSKCRevisionPatches(items []SheinSKCRevisionPatch) []SheinSKCRevisionPatch {
	if len(items) == 0 {
		return nil
	}
	out := make([]SheinSKCRevisionPatch, 0, len(items))
	for _, item := range items {
		patch := SheinSKCRevisionPatch{
			SupplierCode: stringsTrim(item.SupplierCode),
			SkcName:      cloneStringPointer(item.SkcName),
			SaleName:     cloneStringPointer(item.SaleName),
			MainImageURL: cloneStringPointer(item.MainImageURL),
		}
		if item.SaleAttribute != nil {
			attr := *item.SaleAttribute
			patch.SaleAttribute = &attr
		}
		patch.SKUPatches = pruneSheinSKURevisionPatches(item.SKUPatches)
		if patch.SupplierCode == "" && patch.SaleAttribute == nil && patch.SkcName == nil && patch.SaleName == nil && patch.MainImageURL == nil && len(patch.SKUPatches) == 0 {
			continue
		}
		out = append(out, patch)
	}
	return out
}

func pruneSheinSKURevisionPatches(items []SheinSKURevisionPatch) []SheinSKURevisionPatch {
	if len(items) == 0 {
		return nil
	}
	out := make([]SheinSKURevisionPatch, 0, len(items))
	for _, item := range items {
		patch := SheinSKURevisionPatch{
			SupplierSKU: stringsTrim(item.SupplierSKU),
		}
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
			patch.SaleAttributes = append([]SheinResolvedSaleAttribute(nil), item.SaleAttributes...)
		}
		if len(item.SitePriceList) > 0 {
			patch.SitePriceList = append([]SheinSitePrice(nil), item.SitePriceList...)
		}
		if len(item.StockInfoList) > 0 {
			patch.StockInfoList = append([]SheinStockInfo(nil), item.StockInfoList...)
		}
		if patch.SupplierSKU == "" && len(patch.Attributes) == 0 && patch.BasePrice == nil && patch.CostPrice == nil && patch.Currency == nil && patch.StockCount == nil && patch.MainImage == nil && patch.Barcode == nil && len(patch.SaleAttributes) == 0 && len(patch.SitePriceList) == 0 && len(patch.StockInfoList) == 0 {
			continue
		}
		out = append(out, patch)
	}
	return out
}

func hasPlatformImageContent(images *PlatformImageSet) bool {
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

func stringsTrim(v string) string {
	return strings.TrimSpace(v)
}

func isEmptySheinRevisionInput(in *SheinRevisionInput) bool {
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

func isEmptyCategoryPatch(in *SheinCategoryResolutionPatch) bool {
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

func isEmptyAttributePatch(in *SheinAttributeResolutionPatch) bool {
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

func isEmptySalePatch(in *SheinSaleAttributeResolutionPatch) bool {
	return in == nil ||
		(in.Status == nil &&
			in.Source == nil &&
			in.PrimaryAttributeID == nil &&
			in.SecondaryAttributeID == nil &&
			len(in.SKCAttributes) == 0 &&
			len(in.SKUAttributes) == 0 &&
			len(in.SelectionSummary) == 0 &&
			len(in.ReviewNotes) == 0)
}
