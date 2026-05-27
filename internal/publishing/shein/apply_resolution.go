package shein

func ApplyCategoryResolution(pkg *Package, resolution *CategoryResolution) {
	pkg = NormalizePackageSemanticFields(pkg)
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
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || resolution == nil {
		return
	}
	pkg.ResolvedAttributes = append([]ResolvedAttribute(nil), resolution.ResolvedAttributes...)
	if pkg.DraftPayload != nil {
		pkg.DraftPayload.ResolvedAttributes = append([]ResolvedAttribute(nil), resolution.ResolvedAttributes...)
	}
}

func ApplySaleAttributeResolution(pkg *Package, resolution *SaleAttributeResolution) {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || resolution == nil || pkg.DraftPayload == nil {
		return
	}
	pkg.CustomAttributeRelation = dedupeCustomAttributeRelations(append(pkg.CustomAttributeRelation, resolution.CustomAttributeRelation...))
	pkg.DraftPayload.CustomAttributeRelation = dedupeCustomAttributeRelations(append(pkg.DraftPayload.CustomAttributeRelation, resolution.CustomAttributeRelation...))
	if len(pkg.DraftPayload.SKCList) == 0 {
		return
	}

	for skcIndex := range pkg.DraftPayload.SKCList {
		skc := &pkg.DraftPayload.SKCList[skcIndex]
		var skcPackage *SKCPackage
		if skcIndex < len(pkg.SkcList) {
			skcPackage = &pkg.SkcList[skcIndex]
		}
		skcValueAssignments := effectiveSKCValueAssignments(resolution)
		skuValueAssignments := effectiveSKUValueAssignments(resolution)
		if assigned, ok := resolution.skcAssignments[skc.SupplierCode]; ok {
			assignedCopy := assigned
			skc.SaleAttribute = &assignedCopy
		} else if assigned, ok := resolveSaleAttributeValueAssignment(skcValueAssignments, lookupSKCSourceValue(skcPackage, resolution.PrimarySourceDimension)); ok {
			assignedCopy := assigned
			skc.SaleAttribute = &assignedCopy
		} else if skcIndex == 0 && len(resolution.SKCAttributes) > 0 && saleAttributeHasResolvedValue(resolution.SKCAttributes[0]) {
			assignedCopy := resolution.SKCAttributes[0]
			skc.SaleAttribute = &assignedCopy
		}

		for skuIndex := range skc.SKUList {
			sku := &skc.SKUList[skuIndex]
			if assigned, ok := resolution.skuAssignments[sku.SupplierSKU]; ok {
				sku.SaleAttributes = append([]ResolvedSaleAttribute(nil), assigned...)
				continue
			}
			if assigned, ok := resolveSaleAttributeValueAssignment(skuValueAssignments, lookupAttributeValue(sku.Attributes, resolution.SecondarySourceDimension)); ok {
				sku.SaleAttributes = append([]ResolvedSaleAttribute(nil), assigned)
				continue
			}
			if skcIndex == 0 && skuIndex == 0 && len(resolution.SKUAttributes) > 0 && saleAttributeHasResolvedValue(resolution.SKUAttributes[0]) {
				sku.SaleAttributes = append([]ResolvedSaleAttribute(nil), resolution.SKUAttributes...)
			}
		}
	}
}

func effectiveSKCValueAssignments(resolution *SaleAttributeResolution) map[string]ResolvedSaleAttribute {
	if resolution == nil {
		return nil
	}
	if len(resolution.skcValueAssignments) > 0 {
		return resolution.skcValueAssignments
	}
	return resolution.SKCValueAssignments
}

func effectiveSKUValueAssignments(resolution *SaleAttributeResolution) map[string]ResolvedSaleAttribute {
	if resolution == nil {
		return nil
	}
	if len(resolution.skuValueAssignments) > 0 {
		return resolution.skuValueAssignments
	}
	return resolution.SKUValueAssignments
}

func resolveSaleAttributeValueAssignment(assignments map[string]ResolvedSaleAttribute, value string) (ResolvedSaleAttribute, bool) {
	if len(assignments) == 0 {
		return ResolvedSaleAttribute{}, false
	}
	assigned, ok := assignments[normalizeText(value)]
	return assigned, ok
}

func lookupSKCSourceValue(skc *SKCPackage, dimensionName string) string {
	if skc == nil {
		return ""
	}
	return lookupAttributeValue(skc.Attributes, dimensionName)
}

func saleAttributeHasResolvedValue(attr ResolvedSaleAttribute) bool {
	return attr.AttributeID > 0 && attr.AttributeValueID != nil && *attr.AttributeValueID > 0
}
