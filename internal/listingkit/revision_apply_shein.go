package listingkit

import (
	"strings"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func applySheinRevision(pkg *sheinpub.Package, req *SheinRevisionInput) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || req == nil {
		return
	}
	if req.SpuName != nil {
		pkg.SpuName = strings.TrimSpace(*req.SpuName)
	}
	if req.ProductNameEn != nil {
		pkg.ProductNameEn = strings.TrimSpace(*req.ProductNameEn)
	}
	if req.BrandName != nil {
		pkg.BrandName = strings.TrimSpace(*req.BrandName)
	}
	if req.Description != nil {
		pkg.Description = strings.TrimSpace(*req.Description)
	}
	if req.SellingPoints != nil {
		pkg.SellingPoints = append([]string(nil), req.SellingPoints...)
	}
	if req.CategoryName != nil {
		pkg.CategoryName = strings.TrimSpace(*req.CategoryName)
	}
	if req.CategoryPath != nil {
		pkg.CategoryPath = append([]string(nil), req.CategoryPath...)
	}
	if req.CategoryID != nil {
		pkg.CategoryID = *req.CategoryID
	}
	if req.CategoryIDList != nil {
		pkg.CategoryIDList = append([]int(nil), req.CategoryIDList...)
	}
	if req.ProductTypeID != nil {
		productTypeID := *req.ProductTypeID
		pkg.ProductTypeID = &productTypeID
	}
	if req.TopCategoryID != nil {
		pkg.TopCategoryID = *req.TopCategoryID
	}
	if req.Images != nil {
		pkg.Images = clonePlatformImageSet(req.Images)
	}
	if req.ProductAttributes != nil {
		pkg.ProductAttributes = append([]common.Attribute(nil), req.ProductAttributes...)
	}
	if req.ResolvedAttributes != nil {
		pkg.ResolvedAttributes = append([]sheinpub.ResolvedAttribute(nil), req.ResolvedAttributes...)
	}
	if req.CategoryResolution != nil {
		applySheinCategoryResolutionPatch(pkg, req.CategoryResolution)
	}
	if req.AttributeResolution != nil {
		applySheinAttributeResolutionPatch(pkg, req.AttributeResolution)
	}
	if req.SaleAttributeResolution != nil {
		applySheinSaleAttributeResolutionPatch(pkg, req.SaleAttributeResolution)
	}
	if req.RequestDraft != nil {
		draftCopy := *req.RequestDraft
		pkg.DraftPayload = &draftCopy
	}
	ensureSheinRequestDraft(pkg)
	if req.SaleAttributeResolution != nil {
		sheinpub.ApplySaleAttributeResolution(pkg, pkg.SaleAttributeResolution)
	}
	if req.SKCPatches != nil {
		applySheinSKCRevisionPatches(pkg, req.SKCPatches)
	}
	if req.ReviewNotes != nil {
		pkg.ReviewNotes = uniqueStrings(append([]string(nil), req.ReviewNotes...))
	}
	normalizeSheinSaleAttributeState(pkg)

	syncSheinDraftFromPackage(pkg)
	preview := sheinpub.BuildPreviewProduct(pkg)
	sheinpub.SetPreviewPayload(pkg, preview)
	refreshSheinReviewState(pkg)
}

func normalizeSheinSaleAttributeState(pkg *sheinpub.Package) {
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return
	}
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg.DraftPayload == nil || len(pkg.DraftPayload.SKCList) == 0 {
		return
	}
	if sheinSaleAttributesReadyForSubmit((*SheinPackage)(pkg)) {
		return
	}
	if pkg.SaleAttributeResolution.Status == "" || pkg.SaleAttributeResolution.Status == "resolved" {
		pkg.SaleAttributeResolution.Status = "partial"
	}
	pkg.SaleAttributeResolution.ReviewNotes = uniqueStrings(append(
		[]string(nil),
		append(
			pkg.SaleAttributeResolution.ReviewNotes,
			"当前销售属性仍缺少真实 sale attribute value 映射，请重新确认规格。",
		)...,
	))
}

func applySheinCategoryResolutionPatch(pkg *sheinpub.Package, patch *SheinCategoryResolutionPatch) {
	if pkg == nil || patch == nil {
		return
	}
	if pkg.CategoryResolution == nil {
		pkg.CategoryResolution = &sheinpub.CategoryResolution{}
	}
	if patch.Status != nil {
		pkg.CategoryResolution.Status = strings.TrimSpace(*patch.Status)
	}
	if patch.Source != nil {
		pkg.CategoryResolution.Source = strings.TrimSpace(*patch.Source)
	}
	if patch.QueryText != nil {
		pkg.CategoryResolution.QueryText = strings.TrimSpace(*patch.QueryText)
	}
	if patch.MatchedPath != nil {
		pkg.CategoryResolution.MatchedPath = append([]string(nil), patch.MatchedPath...)
		pkg.CategoryPath = append([]string(nil), patch.MatchedPath...)
		if len(patch.MatchedPath) > 0 {
			pkg.CategoryName = patch.MatchedPath[len(patch.MatchedPath)-1]
		}
	}
	if patch.CategoryID != nil {
		pkg.CategoryResolution.CategoryID = *patch.CategoryID
		pkg.CategoryID = *patch.CategoryID
	}
	if patch.CategoryIDList != nil {
		pkg.CategoryResolution.CategoryIDList = append([]int(nil), patch.CategoryIDList...)
		pkg.CategoryIDList = append([]int(nil), patch.CategoryIDList...)
	}
	if patch.ProductTypeID != nil {
		pkg.CategoryResolution.ProductTypeID = *patch.ProductTypeID
		productTypeID := *patch.ProductTypeID
		pkg.ProductTypeID = &productTypeID
	}
	if patch.TopCategoryID != nil {
		pkg.CategoryResolution.TopCategoryID = *patch.TopCategoryID
		pkg.TopCategoryID = *patch.TopCategoryID
	}
	if patch.ReviewNotes != nil {
		pkg.CategoryResolution.ReviewNotes = uniqueStrings(append([]string(nil), patch.ReviewNotes...))
	}
}

func applySheinAttributeResolutionPatch(pkg *sheinpub.Package, patch *SheinAttributeResolutionPatch) {
	if pkg == nil || patch == nil {
		return
	}
	if pkg.AttributeResolution == nil {
		pkg.AttributeResolution = &sheinpub.AttributeResolution{}
	}
	if patch.Status != nil {
		pkg.AttributeResolution.Status = strings.TrimSpace(*patch.Status)
	}
	if patch.Source != nil {
		pkg.AttributeResolution.Source = strings.TrimSpace(*patch.Source)
	}
	if patch.CategoryID != nil {
		pkg.AttributeResolution.CategoryID = *patch.CategoryID
	}
	if patch.TemplateCount != nil {
		pkg.AttributeResolution.TemplateCount = *patch.TemplateCount
	}
	if patch.ResolvedCount != nil {
		pkg.AttributeResolution.ResolvedCount = *patch.ResolvedCount
	}
	if patch.UnresolvedCount != nil {
		pkg.AttributeResolution.UnresolvedCount = *patch.UnresolvedCount
	}
	if patch.ResolvedAttributes != nil {
		resolved := append([]sheinpub.ResolvedAttribute(nil), patch.ResolvedAttributes...)
		pkg.AttributeResolution.ResolvedAttributes = resolved
		pkg.ResolvedAttributes = append([]sheinpub.ResolvedAttribute(nil), patch.ResolvedAttributes...)
		if pkg.DraftPayload != nil {
			pkg.DraftPayload.ResolvedAttributes = append([]sheinpub.ResolvedAttribute(nil), patch.ResolvedAttributes...)
		}
	}
	if patch.PendingAttributes != nil {
		pkg.AttributeResolution.PendingAttributes = append([]common.Attribute(nil), patch.PendingAttributes...)
	}
	if patch.PendingAttributeCandidates != nil {
		pkg.AttributeResolution.PendingAttributeCandidates = clonePendingAttributeCandidates(patch.PendingAttributeCandidates)
	}
	if patch.RecommendedAttributeCandidates != nil {
		pkg.AttributeResolution.RecommendedAttributeCandidates = clonePendingAttributeCandidates(patch.RecommendedAttributeCandidates)
	}
	if patch.ReviewNotes != nil {
		pkg.AttributeResolution.ReviewNotes = uniqueStrings(append([]string(nil), patch.ReviewNotes...))
	}
	if patch.ResolvedCount == nil && patch.ResolvedAttributes != nil {
		pkg.AttributeResolution.ResolvedCount = len(patch.ResolvedAttributes)
	}
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

func applySheinSaleAttributeResolutionPatch(pkg *sheinpub.Package, patch *SheinSaleAttributeResolutionPatch) {
	if pkg == nil || patch == nil {
		return
	}
	if pkg.SaleAttributeResolution == nil {
		pkg.SaleAttributeResolution = &sheinpub.SaleAttributeResolution{}
	}
	if patch.Status != nil {
		pkg.SaleAttributeResolution.Status = strings.TrimSpace(*patch.Status)
	}
	if patch.Source != nil {
		pkg.SaleAttributeResolution.Source = strings.TrimSpace(*patch.Source)
	}
	if patch.RecommendCategoryReview != nil {
		pkg.SaleAttributeResolution.RecommendCategoryReview = *patch.RecommendCategoryReview
	}
	if patch.CategoryReviewReason != nil {
		pkg.SaleAttributeResolution.CategoryReviewReason = strings.TrimSpace(*patch.CategoryReviewReason)
	}
	if patch.PrimaryAttributeID != nil {
		pkg.SaleAttributeResolution.PrimaryAttributeID = *patch.PrimaryAttributeID
	}
	if patch.SecondaryAttributeID != nil {
		pkg.SaleAttributeResolution.SecondaryAttributeID = *patch.SecondaryAttributeID
	}
	if patch.SKCAttributes != nil {
		pkg.SaleAttributeResolution.SKCAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), patch.SKCAttributes...)
	}
	if patch.SKUAttributes != nil {
		pkg.SaleAttributeResolution.SKUAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), patch.SKUAttributes...)
	}
	if patch.CustomAttributeRelation != nil {
		pkg.SaleAttributeResolution.CustomAttributeRelation = append([]sheinattribute.CustomAttributeRelation(nil), patch.CustomAttributeRelation...)
	}
	if patch.SelectionSummary != nil {
		pkg.SaleAttributeResolution.SelectionSummary = append([]string(nil), patch.SelectionSummary...)
	}
	if patch.ReviewNotes != nil {
		pkg.SaleAttributeResolution.ReviewNotes = uniqueStrings(append([]string(nil), patch.ReviewNotes...))
	}
}

func applySheinSKCRevisionPatches(pkg *sheinpub.Package, patches []SheinSKCRevisionPatch) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || len(patches) == 0 {
		return
	}
	for _, patch := range patches {
		if strings.TrimSpace(patch.SupplierCode) == "" {
			continue
		}
		draft := findSheinRequestSKC(pkg.DraftPayload.SKCList, patch.SupplierCode)
		pkgSKC := findSheinPackageSKC(pkg.SkcList, patch.SupplierCode)
		if draft == nil {
			continue
		}
		if patch.SkcName != nil {
			draft.SkcName = strings.TrimSpace(*patch.SkcName)
			if pkgSKC != nil {
				pkgSKC.SkcName = draft.SkcName
			}
		}
		if patch.SaleName != nil {
			draft.SaleName = strings.TrimSpace(*patch.SaleName)
			if pkgSKC != nil {
				pkgSKC.SaleName = draft.SaleName
			}
		}
		if patch.MainImageURL != nil {
			image := strings.TrimSpace(*patch.MainImageURL)
			ensureSheinImageDraft(&draft.ImageInfo)
			draft.ImageInfo.MainImage = image
			if len(draft.SKUList) > 0 && strings.TrimSpace(draft.SKUList[0].MainImage) == "" {
				draft.SKUList[0].MainImage = image
			}
			if pkgSKC != nil {
				pkgSKC.MainImageURL = image
			}
		}
		if patch.SaleAttribute != nil {
			saleAttribute := *patch.SaleAttribute
			draft.SaleAttribute = &saleAttribute
			if pkg.SaleAttributeResolution == nil {
				pkg.SaleAttributeResolution = &sheinpub.SaleAttributeResolution{}
			}
			pkg.SaleAttributeResolution.SKCAttributes = []sheinpub.ResolvedSaleAttribute{saleAttribute}
			if saleAttribute.AttributeID > 0 {
				pkg.SaleAttributeResolution.PrimaryAttributeID = saleAttribute.AttributeID
			}
		}
		applySheinSKURevisionPatches(pkg, draft, pkgSKC, patch.SKUPatches)
	}
}

func applySheinSKURevisionPatches(pkg *sheinpub.Package, draft *sheinpub.SKCRequestDraft, pkgSKC *sheinpub.SKCPackage, patches []SheinSKURevisionPatch) {
	if pkg == nil || draft == nil || len(patches) == 0 {
		return
	}
	for _, patch := range patches {
		if strings.TrimSpace(patch.SupplierSKU) == "" {
			continue
		}
		skuDraft := findSheinRequestSKU(draft.SKUList, patch.SupplierSKU)
		pkgSKU := findSheinPackageSKU(pkgSKC, patch.SupplierSKU)
		if skuDraft == nil {
			continue
		}
		if patch.Attributes != nil {
			skuDraft.Attributes = cloneMap(patch.Attributes)
			if pkgSKU != nil {
				pkgSKU.Attributes = cloneMap(patch.Attributes)
			}
		}
		if patch.BasePrice != nil {
			skuDraft.BasePrice = strings.TrimSpace(*patch.BasePrice)
		}
		if patch.CostPrice != nil {
			skuDraft.CostPrice = strings.TrimSpace(*patch.CostPrice)
		}
		if patch.Currency != nil {
			skuDraft.Currency = strings.TrimSpace(*patch.Currency)
		}
		if patch.StockCount != nil {
			skuDraft.StockCount = *patch.StockCount
			if pkgSKU != nil {
				pkgSKU.Stock = *patch.StockCount
			}
		}
		if patch.MainImage != nil {
			skuDraft.MainImage = strings.TrimSpace(*patch.MainImage)
			if pkgSKU != nil {
				pkgSKU.Image = skuDraft.MainImage
			}
		}
		if patch.Barcode != nil {
			skuDraft.Barcode = strings.TrimSpace(*patch.Barcode)
			if pkgSKU != nil {
				pkgSKU.Barcode = skuDraft.Barcode
			}
		}
		if patch.SaleAttributes != nil {
			skuDraft.SaleAttributes = append([]SheinResolvedSaleAttribute(nil), patch.SaleAttributes...)
			if pkg.SaleAttributeResolution == nil {
				pkg.SaleAttributeResolution = &sheinpub.SaleAttributeResolution{}
			}
			pkg.SaleAttributeResolution.SKUAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), patch.SaleAttributes...)
			if len(patch.SaleAttributes) > 0 && patch.SaleAttributes[0].AttributeID > 0 {
				pkg.SaleAttributeResolution.SecondaryAttributeID = patch.SaleAttributes[0].AttributeID
			}
		}
		if patch.SitePriceList != nil {
			skuDraft.SitePriceList = append([]sheinpub.SitePrice(nil), patch.SitePriceList...)
		}
		if patch.StockInfoList != nil {
			skuDraft.StockInfoList = append([]sheinpub.StockInfo(nil), patch.StockInfoList...)
		}
	}
}
