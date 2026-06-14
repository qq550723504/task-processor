package workspace

import (
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func BuildRevisionDiffPreview(pkg *sheinpub.Package, revision *EditorRevisionSkeleton) *RevisionDiffPreview {
	if pkg == nil || revision == nil || revision.Shein == nil {
		return nil
	}

	changes := make([]RevisionFieldChange, 0, 16)
	appendStringChange(&changes, "shein.spu_name", "SPU 名称", pkg.SpuName, revision.Shein.SpuName)
	appendStringChange(&changes, "shein.product_name_en", "英文标题", pkg.ProductNameEn, revision.Shein.ProductNameEn)
	appendStringChange(&changes, "shein.brand_name", "品牌", pkg.BrandName, revision.Shein.BrandName)
	appendStringChange(&changes, "shein.description", "描述", pkg.Description, revision.Shein.Description)
	appendImageChange(&changes, pkg.Images, revision.Shein.Images)
	appendIntChange(&changes, "shein.category_id", "类目 ID", pkg.CategoryID, revision.Shein.CategoryID)
	appendIntSliceChange(&changes, "shein.category_id_list", "类目层级", pkg.CategoryIDList, revision.Shein.CategoryIDList)
	appendIntPtrChange(&changes, "shein.product_type_id", "Product Type ID", pkg.ProductTypeID, revision.Shein.ProductTypeID)
	appendAttributesChange(&changes, "shein.product_attributes", "普通属性", pkg.ProductAttributes, revision.Shein.ProductAttributes)
	appendResolvedAttributesChange(&changes, pkg.ResolvedAttributes, revision.Shein.ResolvedAttributes)
	appendSalePatchChange(&changes, pkg, revision.Shein.SKCPatches)

	if patch := revision.Shein.CategoryResolution; patch != nil {
		appendIntPtrChange(&changes, "shein.category_resolution.category_id", "类目解析 ID", intPointer(pkg.CategoryID), patch.CategoryID)
	}
	if patch := revision.Shein.SaleAttributeResolution; patch != nil {
		beforePrimary := 0
		if pkg.SaleAttributeResolution != nil {
			beforePrimary = pkg.SaleAttributeResolution.PrimaryAttributeID
		}
		appendIntChange(&changes, "shein.sale_attribute_resolution.primary_attribute_id", "主销售属性 ID", beforePrimary, patch.PrimaryAttributeID)
	}

	if len(changes) == 0 {
		return &RevisionDiffPreview{}
	}
	return &RevisionDiffPreview{ChangeCount: len(changes), Changes: changes}
}

func BuildRevisionDiffPreviewFromInput(revision *EditorRevisionSkeleton) *RevisionDiffPreview {
	if revision == nil || revision.Shein == nil {
		return nil
	}
	changes := make([]RevisionFieldChange, 0, 16)
	appendStringChange(&changes, "shein.spu_name", "SPU 名称", "", revision.Shein.SpuName)
	appendStringChange(&changes, "shein.product_name_en", "英文标题", "", revision.Shein.ProductNameEn)
	appendStringChange(&changes, "shein.brand_name", "品牌", "", revision.Shein.BrandName)
	appendStringChange(&changes, "shein.description", "描述", "", revision.Shein.Description)
	appendImageChange(&changes, nil, revision.Shein.Images)
	appendIntChange(&changes, "shein.category_id", "类目 ID", 0, revision.Shein.CategoryID)
	appendIntSliceChange(&changes, "shein.category_id_list", "类目层级", nil, revision.Shein.CategoryIDList)
	appendIntPtrChange(&changes, "shein.product_type_id", "Product Type ID", nil, revision.Shein.ProductTypeID)
	appendAttributesChange(&changes, "shein.product_attributes", "普通属性", nil, revision.Shein.ProductAttributes)
	appendResolvedAttributesChange(&changes, nil, revision.Shein.ResolvedAttributes)
	appendSalePatchChange(&changes, nil, revision.Shein.SKCPatches)

	if patch := revision.Shein.CategoryResolution; patch != nil {
		appendIntPtrChange(&changes, "shein.category_resolution.category_id", "类目解析 ID", nil, patch.CategoryID)
	}
	if patch := revision.Shein.SaleAttributeResolution; patch != nil {
		appendIntChange(&changes, "shein.sale_attribute_resolution.primary_attribute_id", "主销售属性 ID", 0, patch.PrimaryAttributeID)
	}

	if len(changes) == 0 {
		return &RevisionDiffPreview{}
	}
	return &RevisionDiffPreview{ChangeCount: len(changes), Changes: changes}
}

func BuildRevisionDiffBetweenRevisions(base, target *EditorRevisionSkeleton) *RevisionDiffPreview {
	if target == nil || target.Shein == nil {
		return nil
	}

	var baseShein *RevisionInput
	if base != nil {
		baseShein = base.Shein
	}

	changes := make([]RevisionFieldChange, 0, 16)
	appendCompareStringChange(&changes, "shein.spu_name", "SPU 名称", baseShein, target.Shein, func(in *RevisionInput) *string { return in.SpuName })
	appendCompareStringChange(&changes, "shein.product_name_en", "英文标题", baseShein, target.Shein, func(in *RevisionInput) *string { return in.ProductNameEn })
	appendCompareStringChange(&changes, "shein.brand_name", "品牌", baseShein, target.Shein, func(in *RevisionInput) *string { return in.BrandName })
	appendCompareStringChange(&changes, "shein.description", "描述", baseShein, target.Shein, func(in *RevisionInput) *string { return in.Description })
	appendCompareImageChange(&changes, baseShein, target.Shein)
	appendCompareIntChange(&changes, "shein.category_id", "类目 ID", baseShein, target.Shein, func(in *RevisionInput) *int { return in.CategoryID })
	appendCompareIntSliceChange(&changes, "shein.category_id_list", "类目层级", baseShein, target.Shein, func(in *RevisionInput) []int { return in.CategoryIDList })
	appendCompareIntPtrChange(&changes, "shein.product_type_id", "Product Type ID", baseShein, target.Shein, func(in *RevisionInput) *int { return in.ProductTypeID })
	appendCompareAttributesChange(&changes, "shein.product_attributes", "普通属性", baseShein, target.Shein)
	appendCompareResolvedAttributesChange(&changes, baseShein, target.Shein)
	appendCompareSalePatchChange(&changes, baseShein, target.Shein)

	if len(changes) == 0 {
		return &RevisionDiffPreview{}
	}
	return &RevisionDiffPreview{ChangeCount: len(changes), Changes: changes}
}

func appendStringChange(changes *[]RevisionFieldChange, fieldPath, label, before string, after *string) {
	if changes == nil || after == nil || before == *after {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: fieldPath, Label: label, Before: before, After: *after})
}

func appendIntChange(changes *[]RevisionFieldChange, fieldPath, label string, before int, after *int) {
	if changes == nil || after == nil || before == *after {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: fieldPath, Label: label, Before: before, After: *after})
}

func appendIntPtrChange(changes *[]RevisionFieldChange, fieldPath, label string, before, after *int) {
	if changes == nil || after == nil {
		return
	}
	var beforeValue any
	if before != nil {
		beforeValue = *before
	}
	if before != nil && *before == *after {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: fieldPath, Label: label, Before: beforeValue, After: *after})
}

func appendIntSliceChange(changes *[]RevisionFieldChange, fieldPath, label string, before, after []int) {
	if changes == nil || len(after) == 0 || equalIntSlices(before, after) {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: fieldPath, Label: label, Before: append([]int(nil), before...), After: append([]int(nil), after...)})
}

func appendImageChange(changes *[]RevisionFieldChange, before, after *common.ImageSet) {
	if changes == nil || after == nil {
		return
	}
	beforeMain := ""
	if before != nil {
		beforeMain = before.MainImage
	}
	if beforeMain == after.MainImage {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: "shein.images.main_image", Label: "主图", Before: beforeMain, After: after.MainImage})
}

func appendAttributesChange(changes *[]RevisionFieldChange, fieldPath, label string, before, after []common.Attribute) {
	if changes == nil || len(after) == 0 || equalPlatformAttributes(before, after) {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: fieldPath, Label: label, Before: before, After: after})
}

func appendResolvedAttributesChange(changes *[]RevisionFieldChange, before, after []sheinpub.ResolvedAttribute) {
	if changes == nil || len(after) == 0 || equalResolvedAttributes(before, after) {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: "shein.resolved_attributes", Label: "已解析属性", Before: before, After: after})
}

func appendSalePatchChange(changes *[]RevisionFieldChange, pkg *sheinpub.Package, after []SKCRevisionPatch) {
	if changes == nil || len(after) == 0 {
		return
	}
	beforeCount := 0
	if pkg != nil && pkg.RequestDraft != nil {
		beforeCount = len(pkg.RequestDraft.SKCList)
	}
	if beforeCount == len(after) {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: "shein.skc_patches", Label: "规格补丁", Before: beforeCount, After: len(after)})
}

func appendCompareStringChange(changes *[]RevisionFieldChange, fieldPath, label string, base, target *RevisionInput, getter func(*RevisionInput) *string) {
	if changes == nil || target == nil {
		return
	}
	var before string
	if base != nil && getter(base) != nil {
		before = *getter(base)
	}
	after := getter(target)
	if after == nil || before == *after {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: fieldPath, Label: label, Before: before, After: *after})
}

func appendCompareIntChange(changes *[]RevisionFieldChange, fieldPath, label string, base, target *RevisionInput, getter func(*RevisionInput) *int) {
	if changes == nil || target == nil {
		return
	}
	var before int
	if base != nil && getter(base) != nil {
		before = *getter(base)
	}
	after := getter(target)
	if after == nil || before == *after {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: fieldPath, Label: label, Before: before, After: *after})
}

func appendCompareIntPtrChange(changes *[]RevisionFieldChange, fieldPath, label string, base, target *RevisionInput, getter func(*RevisionInput) *int) {
	if changes == nil || target == nil {
		return
	}
	var before any
	if base != nil && getter(base) != nil {
		before = *getter(base)
	}
	after := getter(target)
	if after == nil {
		return
	}
	if base != nil && getter(base) != nil && *getter(base) == *after {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: fieldPath, Label: label, Before: before, After: *after})
}

func appendCompareIntSliceChange(changes *[]RevisionFieldChange, fieldPath, label string, base, target *RevisionInput, getter func(*RevisionInput) []int) {
	if changes == nil || target == nil {
		return
	}
	var before []int
	if base != nil {
		before = getter(base)
	}
	after := getter(target)
	if len(after) == 0 || equalIntSlices(before, after) {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: fieldPath, Label: label, Before: append([]int(nil), before...), After: append([]int(nil), after...)})
}

func appendCompareImageChange(changes *[]RevisionFieldChange, base, target *RevisionInput) {
	if changes == nil || target == nil || target.Images == nil {
		return
	}
	var before string
	if base != nil && base.Images != nil {
		before = base.Images.MainImage
	}
	if before == target.Images.MainImage {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: "shein.images.main_image", Label: "主图", Before: before, After: target.Images.MainImage})
}

func appendCompareAttributesChange(changes *[]RevisionFieldChange, fieldPath, label string, base, target *RevisionInput) {
	if changes == nil || target == nil || len(target.ProductAttributes) == 0 {
		return
	}
	var before []common.Attribute
	if base != nil {
		before = base.ProductAttributes
	}
	if equalPlatformAttributes(before, target.ProductAttributes) {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: fieldPath, Label: label, Before: before, After: target.ProductAttributes})
}

func appendCompareResolvedAttributesChange(changes *[]RevisionFieldChange, base, target *RevisionInput) {
	if changes == nil || target == nil || len(target.ResolvedAttributes) == 0 {
		return
	}
	var before []sheinpub.ResolvedAttribute
	if base != nil {
		before = base.ResolvedAttributes
	}
	if equalResolvedAttributes(before, target.ResolvedAttributes) {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: "shein.resolved_attributes", Label: "已解析属性", Before: before, After: target.ResolvedAttributes})
}

func appendCompareSalePatchChange(changes *[]RevisionFieldChange, base, target *RevisionInput) {
	if changes == nil || target == nil || len(target.SKCPatches) == 0 {
		return
	}
	beforeCount := 0
	if base != nil {
		beforeCount = len(base.SKCPatches)
	}
	if beforeCount == len(target.SKCPatches) {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: "shein.skc_patches", Label: "规格补丁", Before: beforeCount, After: len(target.SKCPatches)})
}

func intPointer(v int) *int {
	if v == 0 {
		return nil
	}
	value := v
	return &value
}

func equalIntSlices(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalPlatformAttributes(a, b []common.Attribute) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalResolvedAttributes(a, b []sheinpub.ResolvedAttribute) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Value != b[i].Value || a[i].AttributeID != b[i].AttributeID {
			return false
		}
	}
	return true
}
