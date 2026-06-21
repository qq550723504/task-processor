package workspace

import (
	"strings"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

// ApplyCategoryResolutionPatch applies editor category resolution changes to a SHEIN package.
func ApplyCategoryResolutionPatch(pkg *sheinpub.Package, patch *CategoryResolutionPatch) {
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

// ApplyAttributeResolutionPatch applies editor attribute resolution changes to a SHEIN package.
func ApplyAttributeResolutionPatch(pkg *sheinpub.Package, patch *AttributeResolutionPatch) {
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

// ApplySaleAttributeResolutionPatch applies editor sale attribute resolution changes to a SHEIN package.
func ApplySaleAttributeResolutionPatch(pkg *sheinpub.Package, patch *SaleAttributeResolutionPatch) {
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
	if patch.PrimarySourceDimension != nil {
		pkg.SaleAttributeResolution.PrimarySourceDimension = strings.TrimSpace(*patch.PrimarySourceDimension)
	}
	if patch.SecondarySourceDimension != nil {
		pkg.SaleAttributeResolution.SecondarySourceDimension = strings.TrimSpace(*patch.SecondarySourceDimension)
	}
	if patch.SKCAttributes != nil {
		pkg.SaleAttributeResolution.SKCAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), patch.SKCAttributes...)
	}
	if patch.SKUAttributes != nil {
		pkg.SaleAttributeResolution.SKUAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), patch.SKUAttributes...)
	}
	if patch.SKCValueAssignments != nil {
		pkg.SaleAttributeResolution.SKCValueAssignments = cloneResolvedSaleAttributeMap(patch.SKCValueAssignments)
	}
	if patch.SKUValueAssignments != nil {
		pkg.SaleAttributeResolution.SKUValueAssignments = cloneResolvedSaleAttributeMap(patch.SKUValueAssignments)
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

// ApplySKCRevisionPatches applies editor SKC and nested SKU changes to a SHEIN package.
func ApplySKCRevisionPatches(pkg *sheinpub.Package, patches []SKCRevisionPatch) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || len(patches) == 0 {
		return
	}
	for _, patch := range patches {
		if strings.TrimSpace(patch.SupplierCode) == "" {
			continue
		}
		draft := findRequestSKC(pkg.DraftPayload.SKCList, patch.SupplierCode)
		pkgSKC := findPackageSKC(pkg.SkcList, patch.SupplierCode)
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
			ensureImageDraft(&draft.ImageInfo)
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
		ApplySKURevisionPatches(pkg, draft, pkgSKC, patch.SKUPatches)
	}
}

// ApplySKURevisionPatches applies editor SKU changes to draft and package SKU views.
func ApplySKURevisionPatches(pkg *sheinpub.Package, draft *sheinpub.SKCRequestDraft, pkgSKC *sheinpub.SKCPackage, patches []SKURevisionPatch) {
	if pkg == nil || draft == nil || len(patches) == 0 {
		return
	}
	for _, patch := range patches {
		if strings.TrimSpace(patch.SupplierSKU) == "" {
			continue
		}
		skuDraft := findRequestSKU(draft.SKUList, patch.SupplierSKU)
		pkgSKU := findPackageSKU(pkgSKC, patch.SupplierSKU)
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
			skuDraft.SaleAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), patch.SaleAttributes...)
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

func ensureImageDraft(info **sheinpub.ImageDraft) {
	if info == nil || *info != nil {
		return
	}
	*info = &sheinpub.ImageDraft{}
}

func findRequestSKC(items []sheinpub.SKCRequestDraft, supplierCode string) *sheinpub.SKCRequestDraft {
	for i := range items {
		if strings.EqualFold(strings.TrimSpace(items[i].SupplierCode), strings.TrimSpace(supplierCode)) {
			return &items[i]
		}
	}
	return nil
}

func findPackageSKC(items []sheinpub.SKCPackage, supplierCode string) *sheinpub.SKCPackage {
	for i := range items {
		if strings.EqualFold(strings.TrimSpace(items[i].SupplierCode), strings.TrimSpace(supplierCode)) {
			return &items[i]
		}
	}
	return nil
}

func findRequestSKU(items []sheinpub.SKUDraft, supplierSKU string) *sheinpub.SKUDraft {
	for i := range items {
		if strings.EqualFold(strings.TrimSpace(items[i].SupplierSKU), strings.TrimSpace(supplierSKU)) {
			return &items[i]
		}
	}
	return nil
}

func findPackageSKU(skc *sheinpub.SKCPackage, supplierSKU string) *common.Variant {
	if skc == nil {
		return nil
	}
	for i := range skc.SKUs {
		if strings.EqualFold(strings.TrimSpace(skc.SKUs[i].SKU), strings.TrimSpace(supplierSKU)) {
			return &skc.SKUs[i]
		}
	}
	return nil
}
