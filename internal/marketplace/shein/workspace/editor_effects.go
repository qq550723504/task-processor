package workspace

func BuildCategoryEffects() []EditorEffect {
	return []EditorEffect{{
		Key:            "category_resolution",
		Label:          "类目修改影响",
		AffectedFields: []string{"shein.category_id", "shein.category_id_list", "shein.product_type_id", "shein.category_resolution"},
		PreviewBlocks:  []string{"inspection.category", "submit_readiness", "submit_checklist", "status_overview", "editor_context.category", "preview_product"},
		Reason:         "修改类目会直接影响类目解析状态、提交前校验以及 preview_product 的类目骨架",
	}}
}

func BuildAttributeEffects() []EditorEffect {
	return []EditorEffect{{
		Key:            "attribute_resolution",
		Label:          "属性修改影响",
		AffectedFields: []string{"shein.resolved_attributes", "shein.attribute_resolution", "shein.request_draft.resolved_attributes"},
		PreviewBlocks:  []string{"inspection.attributes", "submit_readiness", "submit_checklist", "editor_context.attributes", "preview_product"},
		Reason:         "修改普通属性会同步影响属性解析状态、提交前校验和 preview_product 的属性列表",
	}}
}

func BuildSaleAttributeEffects() []EditorEffect {
	return []EditorEffect{{
		Key:            "sale_attribute_resolution",
		Label:          "规格修改影响",
		AffectedFields: []string{"shein.sale_attribute_resolution", "shein.request_draft.skc_list", "shein.skc_list"},
		PreviewBlocks:  []string{"inspection.sale_attributes", "submit_readiness", "submit_checklist", "status_overview", "editor_context.sale_attributes", "preview_product"},
		Reason:         "修改销售属性或 SKC/SKU patch 会同步刷新规格解析、预览规格和提交前状态",
	}}
}
