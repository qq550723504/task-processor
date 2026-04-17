package shein

func ApplyCategoryResolution(pkg *Package, resolution *CategoryResolution) {
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
		pkg.CategoryName = resolution.MatchedPath[len(resolution.MatchedPath)-1]
	}
}

func ApplyAttributeResolution(pkg *Package, resolution *AttributeResolution) {
	if pkg == nil || resolution == nil {
		return
	}
	pkg.ResolvedAttributes = append([]ResolvedAttribute(nil), resolution.ResolvedAttributes...)
	if pkg.RequestDraft != nil {
		pkg.RequestDraft.ResolvedAttributes = append([]ResolvedAttribute(nil), resolution.ResolvedAttributes...)
	}
}

func ApplySaleAttributeResolution(pkg *Package, resolution *SaleAttributeResolution) {
	if pkg == nil || resolution == nil || pkg.RequestDraft == nil {
		return
	}
	if len(pkg.RequestDraft.SKCList) > 0 && len(resolution.SKCAttributes) > 0 {
		pkg.RequestDraft.SKCList[0].SaleAttribute = &resolution.SKCAttributes[0]
	}
	if len(pkg.RequestDraft.SKCList) > 0 && len(pkg.RequestDraft.SKCList[0].SKUList) > 0 && len(resolution.SKUAttributes) > 0 {
		pkg.RequestDraft.SKCList[0].SKUList[0].SaleAttributes = append([]ResolvedSaleAttribute(nil), resolution.SKUAttributes...)
	}
}
