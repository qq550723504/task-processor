package shein

import (
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func BuildRestoreDraftFromSkeleton(reason string, skeleton *EditorRevisionSkeleton) *EditorRevisionSkeleton {
	if skeleton == nil {
		return nil
	}
	restore := CloneEditorRevisionSkeleton(skeleton)
	restore.Actor = "desktop-client"
	restore.Reason = reason
	return restore
}

func CloneEditorRevisionSkeleton(src *EditorRevisionSkeleton) *EditorRevisionSkeleton {
	if src == nil {
		return nil
	}
	cloned := &EditorRevisionSkeleton{
		Platform: src.Platform,
		Actor:    src.Actor,
		Reason:   src.Reason,
	}
	if src.Shein != nil {
		cloned.Shein = CloneRevisionInput(src.Shein)
	}
	return cloned
}

func CloneRevisionInput(src *RevisionInput) *RevisionInput {
	if src == nil {
		return nil
	}
	cloned := &RevisionInput{
		SpuName:                 cloneStringPointer(src.SpuName),
		ProductNameEn:           cloneStringPointer(src.ProductNameEn),
		BrandName:               cloneStringPointer(src.BrandName),
		Description:             cloneStringPointer(src.Description),
		SellingPoints:           append([]string(nil), src.SellingPoints...),
		CategoryName:            cloneStringPointer(src.CategoryName),
		CategoryPath:            append([]string(nil), src.CategoryPath...),
		CategoryID:              clonePositiveIntPointer(src.CategoryID),
		CategoryIDList:          append([]int(nil), src.CategoryIDList...),
		ProductTypeID:           clonePositiveIntPointer(src.ProductTypeID),
		TopCategoryID:           clonePositiveIntPointer(src.TopCategoryID),
		Images:                  cloneImageSet(src.Images),
		ProductAttributes:       append([]common.Attribute(nil), src.ProductAttributes...),
		ResolvedAttributes:      append([]sheinpub.ResolvedAttribute(nil), src.ResolvedAttributes...),
		CategoryResolution:      cloneCategoryPatch(src.CategoryResolution),
		AttributeResolution:     cloneAttributePatch(src.AttributeResolution),
		SaleAttributeResolution: cloneSalePatch(src.SaleAttributeResolution),
		SKCPatches:              cloneSKCPatches(src.SKCPatches),
		ReviewNotes:             append([]string(nil), src.ReviewNotes...),
	}
	if src.RequestDraft != nil {
		requestDraft := *src.RequestDraft
		requestDraft.MultiLanguageNameList = append([]sheinpub.LocalizedText(nil), src.RequestDraft.MultiLanguageNameList...)
		requestDraft.MultiLanguageDescList = append([]sheinpub.LocalizedText(nil), src.RequestDraft.MultiLanguageDescList...)
		requestDraft.ProductAttributeList = append([]common.Attribute(nil), src.RequestDraft.ProductAttributeList...)
		requestDraft.ResolvedAttributes = append([]sheinpub.ResolvedAttribute(nil), src.RequestDraft.ResolvedAttributes...)
		requestDraft.SiteList = append([]common.Site(nil), src.RequestDraft.SiteList...)
		requestDraft.SKCList = cloneRequestSKCs(src.RequestDraft.SKCList)
		if src.RequestDraft.ImageInfo != nil {
			imageInfo := *src.RequestDraft.ImageInfo
			imageInfo.Gallery = append([]string(nil), src.RequestDraft.ImageInfo.Gallery...)
			imageInfo.Source = append([]string(nil), src.RequestDraft.ImageInfo.Source...)
			requestDraft.ImageInfo = &imageInfo
		}
		cloned.RequestDraft = &requestDraft
	}
	return cloned
}

func cloneRequestSKCs(src []sheinpub.SKCRequestDraft) []sheinpub.SKCRequestDraft {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]sheinpub.SKCRequestDraft, 0, len(src))
	for _, skc := range src {
		item := skc
		item.MultiLanguageNameList = append([]sheinpub.LocalizedText(nil), skc.MultiLanguageNameList...)
		item.SKUList = cloneRequestSKUs(skc.SKUList)
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

func cloneRequestSKUs(src []sheinpub.SKUDraft) []sheinpub.SKUDraft {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]sheinpub.SKUDraft, 0, len(src))
	for _, sku := range src {
		item := sku
		item.Attributes = cloneMap(sku.Attributes)
		item.SaleAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), sku.SaleAttributes...)
		item.StockInfoList = append([]sheinpub.StockInfo(nil), sku.StockInfoList...)
		item.SitePriceList = append([]sheinpub.SitePrice(nil), sku.SitePriceList...)
		cloned = append(cloned, item)
	}
	return cloned
}
