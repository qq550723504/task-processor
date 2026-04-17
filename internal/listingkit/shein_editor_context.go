package listingkit

type SheinEditorContext struct {
	Basics           *SheinEditorBasicsContext        `json:"basics,omitempty"`
	Category         *SheinEditorCategoryContext      `json:"category,omitempty"`
	Attributes       *SheinEditorAttributeContext     `json:"attributes,omitempty"`
	SaleAttributes   *SheinEditorSaleAttributeContext `json:"sale_attributes,omitempty"`
	RevisionSkeleton *SheinEditorRevisionSkeleton     `json:"revision_skeleton,omitempty"`
	DirtyHints       *SheinEditorDirtyHints           `json:"dirty_hints,omitempty"`
	Progress         *SheinEditorProgress             `json:"progress,omitempty"`
}

type SheinEditorBasicsContext struct {
	SpuName       string            `json:"spu_name,omitempty"`
	ProductNameEn string            `json:"product_name_en,omitempty"`
	BrandName     string            `json:"brand_name,omitempty"`
	Description   string            `json:"description,omitempty"`
	Images        *PlatformImageSet `json:"images,omitempty"`
	ReviewNotes   []string          `json:"review_notes,omitempty"`
}

type SheinEditorCategoryContext struct {
	Current        *SheinInspectionCategoryPayload `json:"current,omitempty"`
	SuggestedPatch *SheinCategoryResolutionPatch   `json:"suggested_patch,omitempty"`
	Recommendation *SheinEditorRecommendationMeta  `json:"recommendation,omitempty"`
	PreviewEffects []SheinEditorEffect             `json:"preview_effects,omitempty"`
}

type SheinEditorAttributeContext struct {
	Current        *SheinInspectionAttributePayload `json:"current,omitempty"`
	SuggestedPatch *SheinAttributeResolutionPatch   `json:"suggested_patch,omitempty"`
	Recommendation *SheinEditorRecommendationMeta   `json:"recommendation,omitempty"`
	Suggestions    []SheinEditorAttributeSuggestion `json:"suggestions,omitempty"`
	PreviewEffects []SheinEditorEffect              `json:"preview_effects,omitempty"`
}

type SheinEditorSaleAttributeContext struct {
	Current                  *SheinInspectionSaleAttributePayload `json:"current,omitempty"`
	SuggestedResolutionPatch *SheinSaleAttributeResolutionPatch   `json:"suggested_resolution_patch,omitempty"`
	SuggestedSKCPatches      []SheinSKCRevisionPatch              `json:"suggested_skc_patches,omitempty"`
	Recommendation           *SheinEditorRecommendationMeta       `json:"recommendation,omitempty"`
	CandidateSuggestions     []SheinEditorSaleCandidateSuggestion `json:"candidate_suggestions,omitempty"`
	PreviewEffects           []SheinEditorEffect                  `json:"preview_effects,omitempty"`
}

func buildSheinEditorContext(pkg *SheinPackage) *SheinEditorContext {
	if pkg == nil {
		return nil
	}

	context := &SheinEditorContext{
		Basics: &SheinEditorBasicsContext{
			SpuName:       pkg.SpuName,
			ProductNameEn: pkg.ProductNameEn,
			BrandName:     pkg.BrandName,
			Description:   pkg.Description,
			Images:        clonePlatformImageSetForEditor(pkg.Images),
			ReviewNotes:   append([]string(nil), pkg.ReviewNotes...),
		},
		Category: &SheinEditorCategoryContext{
			Current:        buildSheinCategoryPayload(pkg),
			SuggestedPatch: buildSheinCategoryResolutionPatch(pkg),
			Recommendation: buildSheinCategoryRecommendationMeta(pkg),
			PreviewEffects: buildSheinCategoryEffects(),
		},
		Attributes: &SheinEditorAttributeContext{
			Current:        buildSheinAttributePayload(pkg),
			SuggestedPatch: buildSheinAttributeResolutionPatch(pkg),
			Recommendation: buildSheinAttributeRecommendationMeta(pkg),
			Suggestions:    buildSheinAttributeSuggestions(pkg),
			PreviewEffects: buildSheinAttributeEffects(),
		},
		SaleAttributes: &SheinEditorSaleAttributeContext{
			Current:                  buildSheinSaleAttributePayload(pkg),
			SuggestedResolutionPatch: buildSheinSaleAttributeResolutionPatch(pkg),
			SuggestedSKCPatches:      buildSheinEditorSKCPatches(pkg),
			Recommendation:           buildSheinSaleRecommendationMeta(pkg),
			CandidateSuggestions:     buildSheinSaleCandidateSuggestions(pkg),
			PreviewEffects:           buildSheinSaleAttributeEffects(),
		},
		RevisionSkeleton: buildSheinEditorRevisionSkeleton(pkg),
		DirtyHints:       buildSheinEditorDirtyHints(pkg),
	}
	context.Progress = buildSheinEditorProgress(pkg, nil)

	return context
}

func buildSheinCategoryResolutionPatch(pkg *SheinPackage) *SheinCategoryResolutionPatch {
	if pkg == nil {
		return nil
	}
	patch := &SheinCategoryResolutionPatch{
		MatchedPath:    append([]string(nil), pkg.CategoryPath...),
		CategoryIDList: append([]int(nil), pkg.CategoryIDList...),
	}
	if pkg.CategoryResolution != nil {
		if pkg.CategoryResolution.Status != "" {
			status := pkg.CategoryResolution.Status
			patch.Status = &status
		}
		if pkg.CategoryResolution.Source != "" {
			source := pkg.CategoryResolution.Source
			patch.Source = &source
		}
		if pkg.CategoryResolution.QueryText != "" {
			queryText := pkg.CategoryResolution.QueryText
			patch.QueryText = &queryText
		}
		if len(pkg.CategoryResolution.MatchedPath) > 0 {
			patch.MatchedPath = append([]string(nil), pkg.CategoryResolution.MatchedPath...)
		}
		patch.ReviewNotes = append([]string(nil), pkg.CategoryResolution.ReviewNotes...)
	}
	if pkg.CategoryID > 0 {
		categoryID := pkg.CategoryID
		patch.CategoryID = &categoryID
	}
	if pkg.ProductTypeID != nil {
		productTypeID := *pkg.ProductTypeID
		patch.ProductTypeID = &productTypeID
	}
	if pkg.TopCategoryID > 0 {
		topCategoryID := pkg.TopCategoryID
		patch.TopCategoryID = &topCategoryID
	}
	return patch
}

func buildSheinAttributeResolutionPatch(pkg *SheinPackage) *SheinAttributeResolutionPatch {
	if pkg == nil {
		return nil
	}
	patch := &SheinAttributeResolutionPatch{
		ResolvedAttributes: append([]SheinResolvedAttribute(nil), pkg.ResolvedAttributes...),
	}
	if pkg.AttributeResolution != nil {
		if pkg.AttributeResolution.Status != "" {
			status := pkg.AttributeResolution.Status
			patch.Status = &status
		}
		if pkg.AttributeResolution.Source != "" {
			source := pkg.AttributeResolution.Source
			patch.Source = &source
		}
		if pkg.AttributeResolution.CategoryID > 0 {
			categoryID := pkg.AttributeResolution.CategoryID
			patch.CategoryID = &categoryID
		}
		templateCount := pkg.AttributeResolution.TemplateCount
		patch.TemplateCount = &templateCount
		resolvedCount := pkg.AttributeResolution.ResolvedCount
		patch.ResolvedCount = &resolvedCount
		unresolvedCount := pkg.AttributeResolution.UnresolvedCount
		patch.UnresolvedCount = &unresolvedCount
		if len(pkg.AttributeResolution.ResolvedAttributes) > 0 {
			patch.ResolvedAttributes = append([]SheinResolvedAttribute(nil), pkg.AttributeResolution.ResolvedAttributes...)
		}
		patch.ReviewNotes = append([]string(nil), pkg.AttributeResolution.ReviewNotes...)
	}
	return patch
}

func buildSheinSaleAttributeResolutionPatch(pkg *SheinPackage) *SheinSaleAttributeResolutionPatch {
	if pkg == nil {
		return nil
	}
	patch := &SheinSaleAttributeResolutionPatch{}
	if pkg.SaleAttributeResolution == nil {
		return patch
	}
	if pkg.SaleAttributeResolution.Status != "" {
		status := pkg.SaleAttributeResolution.Status
		patch.Status = &status
	}
	if pkg.SaleAttributeResolution.Source != "" {
		source := pkg.SaleAttributeResolution.Source
		patch.Source = &source
	}
	if pkg.SaleAttributeResolution.PrimaryAttributeID > 0 {
		primaryAttributeID := pkg.SaleAttributeResolution.PrimaryAttributeID
		patch.PrimaryAttributeID = &primaryAttributeID
	}
	if pkg.SaleAttributeResolution.SecondaryAttributeID > 0 {
		secondaryAttributeID := pkg.SaleAttributeResolution.SecondaryAttributeID
		patch.SecondaryAttributeID = &secondaryAttributeID
	}
	patch.SKCAttributes = append([]SheinResolvedSaleAttribute(nil), pkg.SaleAttributeResolution.SKCAttributes...)
	patch.SKUAttributes = append([]SheinResolvedSaleAttribute(nil), pkg.SaleAttributeResolution.SKUAttributes...)
	patch.SelectionSummary = append([]string(nil), pkg.SaleAttributeResolution.SelectionSummary...)
	patch.ReviewNotes = append([]string(nil), pkg.SaleAttributeResolution.ReviewNotes...)
	return patch
}

func buildSheinEditorSKCPatches(pkg *SheinPackage) []SheinSKCRevisionPatch {
	if pkg == nil || pkg.RequestDraft == nil || len(pkg.RequestDraft.SKCList) == 0 {
		return nil
	}
	patches := make([]SheinSKCRevisionPatch, 0, len(pkg.RequestDraft.SKCList))
	for _, skc := range pkg.RequestDraft.SKCList {
		patch := SheinSKCRevisionPatch{
			SupplierCode: skc.SupplierCode,
			SKUPatches:   buildSheinEditorSKUPatches(skc.SKUList),
		}
		if skc.SkcName != "" {
			skcName := skc.SkcName
			patch.SkcName = &skcName
		}
		if skc.SaleName != "" {
			saleName := skc.SaleName
			patch.SaleName = &saleName
		}
		if skc.ImageInfo != nil && skc.ImageInfo.MainImage != "" {
			mainImageURL := skc.ImageInfo.MainImage
			patch.MainImageURL = &mainImageURL
		}
		if skc.SaleAttribute != nil {
			attr := *skc.SaleAttribute
			patch.SaleAttribute = &attr
		}
		patches = append(patches, patch)
	}
	return patches
}

func buildSheinEditorSKUPatches(items []SheinSKUDraft) []SheinSKURevisionPatch {
	if len(items) == 0 {
		return nil
	}
	patches := make([]SheinSKURevisionPatch, 0, len(items))
	for _, sku := range items {
		patch := SheinSKURevisionPatch{
			SupplierSKU:    sku.SupplierSKU,
			Attributes:     cloneMap(sku.Attributes),
			SaleAttributes: append([]SheinResolvedSaleAttribute(nil), sku.SaleAttributes...),
			SitePriceList:  append([]SheinSitePrice(nil), sku.SitePriceList...),
			StockInfoList:  append([]SheinStockInfo(nil), sku.StockInfoList...),
		}
		if sku.BasePrice != "" {
			basePrice := sku.BasePrice
			patch.BasePrice = &basePrice
		}
		if sku.CostPrice != "" {
			costPrice := sku.CostPrice
			patch.CostPrice = &costPrice
		}
		if sku.Currency != "" {
			currency := sku.Currency
			patch.Currency = &currency
		}
		stockCount := sku.StockCount
		patch.StockCount = &stockCount
		if sku.MainImage != "" {
			mainImage := sku.MainImage
			patch.MainImage = &mainImage
		}
		if sku.Barcode != "" {
			barcode := sku.Barcode
			patch.Barcode = &barcode
		}
		patches = append(patches, patch)
	}
	return patches
}

func clonePlatformImageSetForEditor(set *PlatformImageSet) *PlatformImageSet {
	if set == nil {
		return nil
	}
	return &PlatformImageSet{
		MainImage:    set.MainImage,
		WhiteBgImage: set.WhiteBgImage,
		Gallery:      append([]string(nil), set.Gallery...),
		SourceImages: append([]string(nil), set.SourceImages...),
	}
}
