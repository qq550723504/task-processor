package listingkit

import (
	"strings"

	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
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
		sheinworkspace.ApplyCategoryResolutionPatch(pkg, req.CategoryResolution)
	}
	if req.AttributeResolution != nil {
		sheinworkspace.ApplyAttributeResolutionPatch(pkg, req.AttributeResolution)
	}
	if req.SaleAttributeResolution != nil {
		sheinworkspace.ApplySaleAttributeResolutionPatch(pkg, req.SaleAttributeResolution)
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
		sheinworkspace.ApplySKCRevisionPatches(pkg, req.SKCPatches)
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
