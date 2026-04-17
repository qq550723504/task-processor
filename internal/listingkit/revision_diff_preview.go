package listingkit

type RevisionDiffPreview struct {
	ChangeCount int                   `json:"change_count"`
	Changes     []RevisionFieldChange `json:"changes,omitempty"`
}

type RevisionFieldChange struct {
	FieldPath string `json:"field_path,omitempty"`
	Label     string `json:"label,omitempty"`
	Before    any    `json:"before,omitempty"`
	After     any    `json:"after,omitempty"`
}

func buildSheinRevisionDiffPreview(pkg *SheinPackage, revision *SheinEditorRevisionSkeleton) *RevisionDiffPreview {
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
	return &RevisionDiffPreview{
		ChangeCount: len(changes),
		Changes:     changes,
	}
}

func buildSheinRevisionDiffPreviewFromInput(revision *SheinEditorRevisionSkeleton) *RevisionDiffPreview {
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
	return &RevisionDiffPreview{
		ChangeCount: len(changes),
		Changes:     changes,
	}
}

func appendStringChange(changes *[]RevisionFieldChange, fieldPath, label, before string, after *string) {
	if changes == nil || after == nil {
		return
	}
	if before == *after {
		return
	}
	*changes = append(*changes, RevisionFieldChange{
		FieldPath: fieldPath,
		Label:     label,
		Before:    before,
		After:     *after,
	})
}

func appendIntChange(changes *[]RevisionFieldChange, fieldPath, label string, before int, after *int) {
	if changes == nil || after == nil {
		return
	}
	if before == *after {
		return
	}
	*changes = append(*changes, RevisionFieldChange{
		FieldPath: fieldPath,
		Label:     label,
		Before:    before,
		After:     *after,
	})
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
	*changes = append(*changes, RevisionFieldChange{
		FieldPath: fieldPath,
		Label:     label,
		Before:    beforeValue,
		After:     *after,
	})
}

func appendIntSliceChange(changes *[]RevisionFieldChange, fieldPath, label string, before, after []int) {
	if changes == nil || len(after) == 0 {
		return
	}
	if equalIntSlices(before, after) {
		return
	}
	*changes = append(*changes, RevisionFieldChange{
		FieldPath: fieldPath,
		Label:     label,
		Before:    append([]int(nil), before...),
		After:     append([]int(nil), after...),
	})
}

func appendImageChange(changes *[]RevisionFieldChange, before, after *PlatformImageSet) {
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
	*changes = append(*changes, RevisionFieldChange{
		FieldPath: "shein.images.main_image",
		Label:     "主图",
		Before:    beforeMain,
		After:     after.MainImage,
	})
}

func appendAttributesChange(changes *[]RevisionFieldChange, fieldPath, label string, before, after []PlatformAttribute) {
	if changes == nil || len(after) == 0 {
		return
	}
	if equalPlatformAttributes(before, after) {
		return
	}
	*changes = append(*changes, RevisionFieldChange{
		FieldPath: fieldPath,
		Label:     label,
		Before:    before,
		After:     after,
	})
}

func appendResolvedAttributesChange(changes *[]RevisionFieldChange, before, after []SheinResolvedAttribute) {
	if changes == nil || len(after) == 0 {
		return
	}
	if equalResolvedAttributes(before, after) {
		return
	}
	*changes = append(*changes, RevisionFieldChange{
		FieldPath: "shein.resolved_attributes",
		Label:     "已解析属性",
		Before:    before,
		After:     after,
	})
}

func appendSalePatchChange(changes *[]RevisionFieldChange, pkg *SheinPackage, after []SheinSKCRevisionPatch) {
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
	*changes = append(*changes, RevisionFieldChange{
		FieldPath: "shein.skc_patches",
		Label:     "规格补丁",
		Before:    beforeCount,
		After:     len(after),
	})
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

func equalPlatformAttributes(a, b []PlatformAttribute) bool {
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

func equalResolvedAttributes(a, b []SheinResolvedAttribute) bool {
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
