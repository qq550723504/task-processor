package listingkit

func buildSheinRevisionDiffBetweenRevisions(base, target *SheinEditorRevisionSkeleton) *RevisionDiffPreview {
	if target == nil || target.Shein == nil {
		return nil
	}

	var baseShein *SheinRevisionInput
	if base != nil {
		baseShein = base.Shein
	}

	changes := make([]RevisionFieldChange, 0, 16)
	appendCompareStringChange(&changes, "shein.spu_name", "SPU 名称", baseShein, target.Shein, func(in *SheinRevisionInput) *string { return in.SpuName })
	appendCompareStringChange(&changes, "shein.product_name_en", "英文标题", baseShein, target.Shein, func(in *SheinRevisionInput) *string { return in.ProductNameEn })
	appendCompareStringChange(&changes, "shein.brand_name", "品牌", baseShein, target.Shein, func(in *SheinRevisionInput) *string { return in.BrandName })
	appendCompareStringChange(&changes, "shein.description", "描述", baseShein, target.Shein, func(in *SheinRevisionInput) *string { return in.Description })
	appendCompareImageChange(&changes, baseShein, target.Shein)
	appendCompareIntChange(&changes, "shein.category_id", "类目 ID", baseShein, target.Shein, func(in *SheinRevisionInput) *int { return in.CategoryID })
	appendCompareIntSliceChange(&changes, "shein.category_id_list", "类目层级", baseShein, target.Shein, func(in *SheinRevisionInput) []int { return in.CategoryIDList })
	appendCompareIntPtrChange(&changes, "shein.product_type_id", "Product Type ID", baseShein, target.Shein, func(in *SheinRevisionInput) *int { return in.ProductTypeID })
	appendCompareAttributesChange(&changes, "shein.product_attributes", "普通属性", baseShein, target.Shein)
	appendCompareResolvedAttributesChange(&changes, baseShein, target.Shein)
	appendCompareSalePatchChange(&changes, baseShein, target.Shein)

	if len(changes) == 0 {
		return &RevisionDiffPreview{}
	}
	return &RevisionDiffPreview{
		ChangeCount: len(changes),
		Changes:     changes,
	}
}

func appendCompareStringChange(changes *[]RevisionFieldChange, fieldPath, label string, base, target *SheinRevisionInput, getter func(*SheinRevisionInput) *string) {
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

func appendCompareIntChange(changes *[]RevisionFieldChange, fieldPath, label string, base, target *SheinRevisionInput, getter func(*SheinRevisionInput) *int) {
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

func appendCompareIntPtrChange(changes *[]RevisionFieldChange, fieldPath, label string, base, target *SheinRevisionInput, getter func(*SheinRevisionInput) *int) {
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

func appendCompareIntSliceChange(changes *[]RevisionFieldChange, fieldPath, label string, base, target *SheinRevisionInput, getter func(*SheinRevisionInput) []int) {
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

func appendCompareImageChange(changes *[]RevisionFieldChange, base, target *SheinRevisionInput) {
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

func appendCompareAttributesChange(changes *[]RevisionFieldChange, fieldPath, label string, base, target *SheinRevisionInput) {
	if changes == nil || target == nil || len(target.ProductAttributes) == 0 {
		return
	}
	var before []PlatformAttribute
	if base != nil {
		before = base.ProductAttributes
	}
	if equalPlatformAttributes(before, target.ProductAttributes) {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: fieldPath, Label: label, Before: before, After: target.ProductAttributes})
}

func appendCompareResolvedAttributesChange(changes *[]RevisionFieldChange, base, target *SheinRevisionInput) {
	if changes == nil || target == nil || len(target.ResolvedAttributes) == 0 {
		return
	}
	var before []SheinResolvedAttribute
	if base != nil {
		before = base.ResolvedAttributes
	}
	if equalResolvedAttributes(before, target.ResolvedAttributes) {
		return
	}
	*changes = append(*changes, RevisionFieldChange{FieldPath: "shein.resolved_attributes", Label: "已解析属性", Before: before, After: target.ResolvedAttributes})
}

func appendCompareSalePatchChange(changes *[]RevisionFieldChange, base, target *SheinRevisionInput) {
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
