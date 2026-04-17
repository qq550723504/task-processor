package listingkit

import (
	"strings"

	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

func buildSheinPackage(req *GenerateRequest, canonical *productenrich.CanonicalProduct, image *productimage.ImageProcessResult, categoryResolver SheinCategoryResolver, attributeResolver SheinAttributeResolver, saleAttributeResolver SheinSaleAttributeResolver) *SheinPackage {
	if canonical == nil {
		return &SheinPackage{ReviewNotes: []string{"canonical product is empty"}}
	}

	images := buildPlatformImages(canonical, image)
	spuName := withBrandHint(canonical.Title, req)
	brand := resolveBrand(canonical, req)
	skcList := buildSheinSKCs(canonical, images)
	productAttributes := buildPlatformAttributes(canonical.Attributes)
	siteList := defaultPlatformSites(req)
	categoryName := lastCategory(canonical.CategoryPath)
	supplierCode := firstSupplierCode(skcList)

	pkg := &SheinPackage{
		SpuName:           spuName,
		BrandName:         brand,
		ProductNameEn:     spuName,
		ProductNameMulti:  firstNonEmpty(strings.TrimSpace(canonical.Description), spuName),
		CategoryName:      categoryName,
		CategoryPath:      append([]string(nil), canonical.CategoryPath...),
		Description:       canonical.Description,
		SellingPoints:     append([]string(nil), canonical.SellingPoints...),
		Attributes:        flattenAttributes(canonical.Attributes),
		ProductAttributes: productAttributes,
		SiteList:          siteList,
		SkcList:           skcList,
		Images:            images,
		RequestDraft: &SheinRequestDraft{
			SpuName:      spuName,
			SupplierCode: supplierCode,
			MultiLanguageNameList: []LocalizedText{
				{Language: req.Language, Name: spuName},
				{Language: "en", Name: spuName},
			},
			MultiLanguageDescList: []LocalizedText{
				{Language: req.Language, Name: canonical.Description},
				{Language: "en", Name: canonical.Description},
			},
			ProductAttributeList: productAttributes,
			ImageInfo:            buildSheinImageDraft(images),
			SiteList:             siteList,
			SKCList:              buildSheinRequestSKCs(canonical, images, siteList),
		},
		Metadata: map[string]string{
			"target_platform": "shein",
			"country":         req.Country,
			"language":        req.Language,
			"category_name":   categoryName,
		},
	}
	if strings.TrimSpace(req.TargetCategoryHint) != "" {
		pkg.Metadata["target_category_hint"] = req.TargetCategoryHint
	}
	if categoryResolver != nil {
		pkg.CategoryResolution = categoryResolver.Resolve(req, canonical, pkg)
		applySheinCategoryResolution(pkg, pkg.CategoryResolution)
	}
	if attributeResolver != nil {
		pkg.AttributeResolution = attributeResolver.Resolve(req, canonical, pkg)
		applySheinAttributeResolution(pkg, pkg.AttributeResolution)
	}
	if saleAttributeResolver != nil {
		pkg.SaleAttributeResolution = saleAttributeResolver.Resolve(req, canonical, pkg)
		applySheinSaleAttributeResolution(pkg, pkg.SaleAttributeResolution)
	}
	pkg.PreviewProduct = buildSheinPreviewProduct(pkg)
	refreshSheinReviewState(pkg, collectReviewNotes(canonical, image)...)
	return pkg
}

func applySheinCategoryResolution(pkg *SheinPackage, resolution *SheinCategoryResolution) {
	if pkg == nil || resolution == nil {
		return
	}
	pkg.CategoryID = resolution.CategoryID
	pkg.CategoryIDList = append([]int(nil), resolution.CategoryIDList...)
	if resolution.ProductTypeID > 0 {
		productTypeID := resolution.ProductTypeID
		pkg.ProductTypeID = &productTypeID
	}
	pkg.TopCategoryID = resolution.TopCategoryID
	if len(resolution.MatchedPath) > 0 {
		pkg.CategoryPath = append([]string(nil), resolution.MatchedPath...)
		pkg.CategoryName = lastCategory(resolution.MatchedPath)
	}
}

func applySheinAttributeResolution(pkg *SheinPackage, resolution *SheinAttributeResolution) {
	if pkg == nil || resolution == nil {
		return
	}
	pkg.ResolvedAttributes = append([]SheinResolvedAttribute(nil), resolution.ResolvedAttributes...)
	if pkg.RequestDraft != nil {
		pkg.RequestDraft.ResolvedAttributes = append([]SheinResolvedAttribute(nil), resolution.ResolvedAttributes...)
	}
}

func applySheinSaleAttributeResolution(pkg *SheinPackage, resolution *SheinSaleAttributeResolution) {
	if pkg == nil || resolution == nil || pkg.RequestDraft == nil {
		return
	}
	if len(pkg.RequestDraft.SKCList) > 0 && len(resolution.SKCAttributes) > 0 {
		pkg.RequestDraft.SKCList[0].SaleAttribute = &resolution.SKCAttributes[0]
	}
	if len(pkg.RequestDraft.SKCList) > 0 && len(pkg.RequestDraft.SKCList[0].SKUList) > 0 && len(resolution.SKUAttributes) > 0 {
		pkg.RequestDraft.SKCList[0].SKUList[0].SaleAttributes = append([]SheinResolvedSaleAttribute(nil), resolution.SKUAttributes...)
	}
}

func buildSheinSKCs(canonical *productenrich.CanonicalProduct, images *PlatformImageSet) []SheinSKCPackage {
	variants := buildPlatformVariants(canonical)
	if len(variants) == 0 {
		return nil
	}

	result := make([]SheinSKCPackage, 0, len(variants))
	for _, variant := range variants {
		saleName := firstNonEmpty(variant.Attributes["color"], variant.Attributes["style"], variant.Attributes["size"], variant.SKU)
		mainImage := firstNonEmpty(variant.Image, images.MainImage)
		result = append(result, SheinSKCPackage{
			SkcName:      saleName,
			SaleName:     saleName,
			SupplierCode: variant.SKU,
			MainImageURL: mainImage,
			Attributes:   variant.Attributes,
			SKUs:         []PlatformVariant{variant},
		})
	}
	return result
}

func buildSheinRequestSKCs(canonical *productenrich.CanonicalProduct, images *PlatformImageSet, siteList []PlatformSite) []SheinSKCRequestDraft {
	variants := buildPlatformVariants(canonical)
	if len(variants) == 0 {
		return nil
	}

	result := make([]SheinSKCRequestDraft, 0, len(variants))
	for idx, variant := range variants {
		saleName := firstNonEmpty(variant.Attributes["color"], variant.Attributes["style"], variant.Attributes["size"], variant.SKU)
		mainImage := firstNonEmpty(variant.Image, images.MainImage)
		result = append(result, SheinSKCRequestDraft{
			SkcName:      saleName,
			SaleName:     saleName,
			SupplierCode: variant.SKU,
			Sort:         idx + 1,
			MultiLanguageNameList: []LocalizedText{
				{Language: "en", Name: saleName},
			},
			ImageInfo: &SheinImageDraft{
				MainImage: mainImage,
				Gallery:   append([]string(nil), images.Gallery...),
				WhiteBg:   images.WhiteBgImage,
			},
			SKUList: []SheinSKUDraft{
				buildSheinSKUDraft(variant, canonical, mainImage, siteList),
			},
		})
	}
	return result
}

func buildSheinSKUDraft(variant PlatformVariant, canonical *productenrich.CanonicalProduct, mainImage string, siteList []PlatformSite) SheinSKUDraft {
	draft := SheinSKUDraft{
		SupplierSKU: variant.SKU,
		Attributes:  cloneMap(variant.Attributes),
		StockCount:  variant.Stock,
		MainImage:   mainImage,
		Barcode:     variant.Barcode,
		IsDefault:   variant.IsDefault,
	}
	if variant.Price != nil {
		draft.Currency = variant.Price.Currency
		draft.CostPrice = formatFloat(variant.Price.CostPrice)
		draft.BasePrice = formatFloat(variant.Price.Amount)
		draft.SitePriceList = buildSheinSitePrices(variant.Price, siteList)
	}
	if canonical != nil && canonical.Specifications != nil {
		if canonical.Specifications.Weight != nil {
			draft.Weight = canonical.Specifications.Weight.Value
			draft.WeightUnit = canonical.Specifications.Weight.Unit
		}
		if canonical.Specifications.Dimensions != nil {
			draft.Length = formatFloat(canonical.Specifications.Dimensions.Length)
			draft.Width = formatFloat(canonical.Specifications.Dimensions.Width)
			draft.Height = formatFloat(canonical.Specifications.Dimensions.Height)
			draft.LengthUnit = canonical.Specifications.Dimensions.Unit
		}
	}
	draft.StockInfoList = []SheinStockInfo{{
		WarehouseCode: "DEFAULT",
		InventoryNum:  variant.Stock,
	}}
	return draft
}

func buildSheinSitePrices(price *PlatformPrice, siteList []PlatformSite) []SheinSitePrice {
	if price == nil {
		return nil
	}
	subSite := "US"
	if len(siteList) > 0 && len(siteList[0].SubSites) > 0 {
		subSite = siteList[0].SubSites[0]
	}
	return []SheinSitePrice{{
		SubSite:   subSite,
		BasePrice: formatFloat(price.Amount),
		Currency:  price.Currency,
	}}
}

func buildSheinImageDraft(images *PlatformImageSet) *SheinImageDraft {
	if images == nil {
		return nil
	}
	return &SheinImageDraft{
		MainImage: images.MainImage,
		Gallery:   append([]string(nil), images.Gallery...),
		WhiteBg:   images.WhiteBgImage,
		Source:    append([]string(nil), images.SourceImages...),
	}
}

func firstSupplierCode(skcs []SheinSKCPackage) string {
	if len(skcs) == 0 {
		return ""
	}
	return skcs[0].SupplierCode
}
