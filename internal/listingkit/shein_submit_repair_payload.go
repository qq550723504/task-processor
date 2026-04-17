package listingkit

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
