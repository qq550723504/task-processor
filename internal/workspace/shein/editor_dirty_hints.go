package shein

import sheinpub "task-processor/internal/publishing/shein"

type EditorDirtyHints struct {
	EditableFields       []string                 `json:"editable_fields,omitempty"`
	DefaultChangedFields []string                 `json:"default_changed_fields,omitempty"`
	Sections             []EditorDirtyHintSection `json:"sections,omitempty"`
}

type EditorDirtyHintSection struct {
	Key                  string   `json:"key,omitempty"`
	Label                string   `json:"label,omitempty"`
	EditableFields       []string `json:"editable_fields,omitempty"`
	DefaultChangedFields []string `json:"default_changed_fields,omitempty"`
}

func BuildEditorDirtyHints(pkg *sheinpub.Package) *EditorDirtyHints {
	if pkg == nil {
		return nil
	}

	sections := []EditorDirtyHintSection{
		{
			Key:                  "basics",
			Label:                "基础信息",
			EditableFields:       []string{"shein.spu_name", "shein.product_name_en", "shein.brand_name", "shein.description", "shein.images", "shein.review_notes"},
			DefaultChangedFields: collectBasicDirtyFields(pkg),
		},
		{
			Key:                  "category",
			Label:                "类目",
			EditableFields:       []string{"shein.category_name", "shein.category_path", "shein.category_id", "shein.category_id_list", "shein.product_type_id", "shein.top_category_id", "shein.category_resolution"},
			DefaultChangedFields: collectCategoryDirtyFields(pkg),
		},
		{
			Key:                  "attributes",
			Label:                "普通属性",
			EditableFields:       []string{"shein.product_attributes", "shein.resolved_attributes", "shein.attribute_resolution"},
			DefaultChangedFields: collectAttributeDirtyFields(pkg),
		},
		{
			Key:                  "sale_attributes",
			Label:                "规格",
			EditableFields:       []string{"shein.sale_attribute_resolution", "shein.skc_patches"},
			DefaultChangedFields: collectSaleDirtyFields(pkg),
		},
	}

	hints := &EditorDirtyHints{Sections: sections}
	for _, section := range sections {
		hints.EditableFields = append(hints.EditableFields, section.EditableFields...)
		hints.DefaultChangedFields = append(hints.DefaultChangedFields, section.DefaultChangedFields...)
	}
	hints.EditableFields = uniqueStrings(hints.EditableFields)
	hints.DefaultChangedFields = uniqueStrings(hints.DefaultChangedFields)
	return hints
}

func collectBasicDirtyFields(pkg *sheinpub.Package) []string {
	fields := make([]string, 0, 6)
	if pkg == nil {
		return fields
	}
	if pkg.SpuName != "" {
		fields = append(fields, "shein.spu_name")
	}
	if pkg.ProductNameEn != "" {
		fields = append(fields, "shein.product_name_en")
	}
	if pkg.BrandName != "" {
		fields = append(fields, "shein.brand_name")
	}
	if pkg.Description != "" {
		fields = append(fields, "shein.description")
	}
	if pkg.Images != nil {
		fields = append(fields, "shein.images")
	}
	if len(pkg.ReviewNotes) > 0 {
		fields = append(fields, "shein.review_notes")
	}
	return fields
}

func collectCategoryDirtyFields(pkg *sheinpub.Package) []string {
	fields := make([]string, 0, 7)
	if pkg == nil {
		return fields
	}
	if pkg.CategoryName != "" {
		fields = append(fields, "shein.category_name")
	}
	if len(pkg.CategoryPath) > 0 {
		fields = append(fields, "shein.category_path")
	}
	if pkg.CategoryID > 0 {
		fields = append(fields, "shein.category_id")
	}
	if len(pkg.CategoryIDList) > 0 {
		fields = append(fields, "shein.category_id_list")
	}
	if pkg.ProductTypeID != nil {
		fields = append(fields, "shein.product_type_id")
	}
	if pkg.TopCategoryID > 0 {
		fields = append(fields, "shein.top_category_id")
	}
	if pkg.CategoryResolution != nil {
		fields = append(fields, "shein.category_resolution")
	}
	return fields
}

func collectAttributeDirtyFields(pkg *sheinpub.Package) []string {
	fields := make([]string, 0, 3)
	if pkg == nil {
		return fields
	}
	if len(pkg.ProductAttributes) > 0 {
		fields = append(fields, "shein.product_attributes")
	}
	if len(pkg.ResolvedAttributes) > 0 {
		fields = append(fields, "shein.resolved_attributes")
	}
	if pkg.AttributeResolution != nil {
		fields = append(fields, "shein.attribute_resolution")
	}
	return fields
}

func collectSaleDirtyFields(pkg *sheinpub.Package) []string {
	fields := make([]string, 0, 2)
	if pkg == nil {
		return fields
	}
	if pkg.SaleAttributeResolution != nil {
		fields = append(fields, "shein.sale_attribute_resolution")
	}
	if pkg.RequestDraft != nil && len(pkg.RequestDraft.SKCList) > 0 {
		fields = append(fields, "shein.skc_patches")
	}
	return fields
}
