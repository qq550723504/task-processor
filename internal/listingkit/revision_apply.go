package listingkit

import (
	"strings"
	"time"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func applyListingKitRevision(result *ListingKitResult, req *ApplyRevisionRequest) error {
	if result == nil {
		return ErrTaskResultUnavailable
	}
	if req == nil {
		return ErrInvalidRevisionRequest
	}
	if err := validateApplyRevisionRequest(req); err != nil {
		return err
	}

	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if len(normalizePlatforms([]string{platform})) == 0 {
		return ErrUnsupportedPreviewPlatform
	}

	switch platform {
	case "amazon":
		if req.Amazon == nil {
			return ErrInvalidRevisionRequest
		}
		if result.Amazon == nil || result.Amazon.Draft == nil {
			return ErrPreviewPlatformUnavailable
		}
		applyAmazonRevision(result.Amazon, req.Amazon)
	case "shein":
		if req.Shein == nil {
			return ErrInvalidRevisionRequest
		}
		if result.Shein == nil {
			return ErrPreviewPlatformUnavailable
		}
		applySheinRevision(result.Shein, req.Shein)
	case "temu":
		if req.Temu == nil {
			return ErrInvalidRevisionRequest
		}
		if result.Temu == nil {
			return ErrPreviewPlatformUnavailable
		}
		applyTemuRevision(result.Temu, req.Temu)
	case "walmart":
		if req.Walmart == nil {
			return ErrInvalidRevisionRequest
		}
		if result.Walmart == nil {
			return ErrPreviewPlatformUnavailable
		}
		applyWalmartRevision(result.Walmart, req.Walmart)
	}

	result.UpdatedAt = time.Now()
	result.Revision = &ListingKitRevisionSummary{
		UpdatedAt: result.UpdatedAt,
		UpdatedBy: strings.TrimSpace(req.Actor),
		Reason:    strings.TrimSpace(req.Reason),
		Platform:  platform,
	}
	return nil
}

func applyAmazonRevision(pkg *AmazonPackage, req *AmazonRevisionInput) {
	if pkg == nil || pkg.Draft == nil || req == nil {
		return
	}
	if req.Title != nil {
		pkg.Draft.Title = strings.TrimSpace(*req.Title)
	}
	if req.Brand != nil {
		pkg.Draft.Brand = strings.TrimSpace(*req.Brand)
	}
	if req.BulletPoints != nil {
		pkg.Draft.BulletPoints = append([]string(nil), req.BulletPoints...)
	}
	if req.Description != nil {
		pkg.Draft.Description = strings.TrimSpace(*req.Description)
	}
}

func applySheinRevision(pkg *sheinpub.Package, req *SheinRevisionInput) {
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
		pkg.RequestDraft = &draftCopy
	}
	if req.SKCPatches != nil {
		ensureSheinRequestDraft(pkg)
		applySheinSKCRevisionPatches(pkg, req.SKCPatches)
	}
	if req.ReviewNotes != nil {
		pkg.ReviewNotes = uniqueStrings(append([]string(nil), req.ReviewNotes...))
	}

	ensureSheinRequestDraft(pkg)
	syncSheinDraftFromPackage(pkg)
	pkg.PreviewProduct = sheinpub.BuildPreviewProduct(pkg)
	refreshSheinReviewState(pkg)
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
	}
	if patch.ReviewNotes != nil {
		pkg.AttributeResolution.ReviewNotes = uniqueStrings(append([]string(nil), patch.ReviewNotes...))
	}
	if patch.ResolvedCount == nil && patch.ResolvedAttributes != nil {
		pkg.AttributeResolution.ResolvedCount = len(patch.ResolvedAttributes)
	}
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
	if patch.SelectionSummary != nil {
		pkg.SaleAttributeResolution.SelectionSummary = append([]string(nil), patch.SelectionSummary...)
	}
	if patch.ReviewNotes != nil {
		pkg.SaleAttributeResolution.ReviewNotes = uniqueStrings(append([]string(nil), patch.ReviewNotes...))
	}
}

func applySheinSKCRevisionPatches(pkg *sheinpub.Package, patches []SheinSKCRevisionPatch) {
	if pkg == nil || pkg.RequestDraft == nil || len(patches) == 0 {
		return
	}
	for _, patch := range patches {
		if strings.TrimSpace(patch.SupplierCode) == "" {
			continue
		}
		draft := findSheinRequestSKC(pkg.RequestDraft.SKCList, patch.SupplierCode)
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

func ensureSheinImageDraft(info **sheinpub.ImageDraft) {
	if info == nil || *info != nil {
		return
	}
	*info = &sheinpub.ImageDraft{}
}

func findSheinRequestSKC(items []sheinpub.SKCRequestDraft, supplierCode string) *sheinpub.SKCRequestDraft {
	for i := range items {
		if strings.EqualFold(strings.TrimSpace(items[i].SupplierCode), strings.TrimSpace(supplierCode)) {
			return &items[i]
		}
	}
	return nil
}

func findSheinPackageSKC(items []sheinpub.SKCPackage, supplierCode string) *sheinpub.SKCPackage {
	for i := range items {
		if strings.EqualFold(strings.TrimSpace(items[i].SupplierCode), strings.TrimSpace(supplierCode)) {
			return &items[i]
		}
	}
	return nil
}

func findSheinRequestSKU(items []sheinpub.SKUDraft, supplierSKU string) *sheinpub.SKUDraft {
	for i := range items {
		if strings.EqualFold(strings.TrimSpace(items[i].SupplierSKU), strings.TrimSpace(supplierSKU)) {
			return &items[i]
		}
	}
	return nil
}

func findSheinPackageSKU(skc *sheinpub.SKCPackage, supplierSKU string) *common.Variant {
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

func ensureSheinRequestDraft(pkg *sheinpub.Package) {
	if pkg == nil || pkg.RequestDraft != nil {
		return
	}
	pkg.RequestDraft = &sheinpub.RequestDraft{}
}

func syncSheinDraftFromPackage(pkg *sheinpub.Package) {
	if pkg == nil || pkg.RequestDraft == nil {
		return
	}
	if strings.TrimSpace(pkg.SpuName) != "" {
		pkg.RequestDraft.SpuName = pkg.SpuName
	}
	if pkg.Images != nil {
		pkg.RequestDraft.ImageInfo = sheinpub.BuildImageDraft(pkg.Images)
	}
	if pkg.ProductAttributes != nil {
		pkg.RequestDraft.ProductAttributeList = append([]common.Attribute(nil), pkg.ProductAttributes...)
	}
	if pkg.ResolvedAttributes != nil {
		pkg.RequestDraft.ResolvedAttributes = append([]sheinpub.ResolvedAttribute(nil), pkg.ResolvedAttributes...)
	}
	if strings.TrimSpace(pkg.Description) != "" {
		updateLocalizedTexts(&pkg.RequestDraft.MultiLanguageDescList, pkg.Description)
	}
	name := firstNonEmpty(pkg.ProductNameEn, pkg.SpuName)
	if strings.TrimSpace(name) != "" {
		updateLocalizedTexts(&pkg.RequestDraft.MultiLanguageNameList, name)
	}
}

func updateLocalizedTexts(items *[]sheinpub.LocalizedText, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if items == nil {
		return
	}
	if len(*items) == 0 {
		*items = []sheinpub.LocalizedText{
			{Language: "en", Name: value},
		}
		return
	}
	for i := range *items {
		(*items)[i].Name = value
	}
}

func applyTemuRevision(pkg *TemuPackage, req *TemuRevisionInput) {
	if pkg == nil || req == nil {
		return
	}
	if req.GoodsName != nil {
		pkg.GoodsName = strings.TrimSpace(*req.GoodsName)
	}
	if req.ShortDescription != nil {
		pkg.ShortDescription = strings.TrimSpace(*req.ShortDescription)
	}
	if req.BulletPoints != nil {
		pkg.BulletPoints = append([]string(nil), req.BulletPoints...)
	}
	if req.Images != nil {
		pkg.Images = clonePlatformImageSet(req.Images)
	}
	if req.ReviewNotes != nil {
		pkg.ReviewNotes = uniqueStrings(append([]string(nil), req.ReviewNotes...))
	}
}

func applyWalmartRevision(pkg *WalmartPackage, req *WalmartRevisionInput) {
	if pkg == nil || req == nil {
		return
	}
	if req.ProductName != nil {
		pkg.ProductName = strings.TrimSpace(*req.ProductName)
	}
	if req.Brand != nil {
		pkg.Brand = strings.TrimSpace(*req.Brand)
	}
	if req.ShortDescription != nil {
		pkg.ShortDescription = strings.TrimSpace(*req.ShortDescription)
	}
	if req.LongDescription != nil {
		pkg.LongDescription = strings.TrimSpace(*req.LongDescription)
	}
	if req.KeyFeatures != nil {
		pkg.KeyFeatures = append([]string(nil), req.KeyFeatures...)
	}
	if req.Images != nil {
		pkg.Images = clonePlatformImageSet(req.Images)
	}
	if req.ReviewNotes != nil {
		pkg.ReviewNotes = uniqueStrings(append([]string(nil), req.ReviewNotes...))
	}
}

func clonePlatformImageSet(images *PlatformImageSet) *PlatformImageSet {
	if images == nil {
		return nil
	}
	return &PlatformImageSet{
		MainImage:    images.MainImage,
		WhiteBgImage: images.WhiteBgImage,
		Gallery:      append([]string(nil), images.Gallery...),
		SourceImages: append([]string(nil), images.SourceImages...),
	}
}
