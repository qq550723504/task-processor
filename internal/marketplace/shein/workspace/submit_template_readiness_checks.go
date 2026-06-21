package workspace

// SubmitTemplateReadinessInput carries SHEIN template validation results for submit readiness checks.
type SubmitTemplateReadinessInput struct {
	CategoryReady        bool
	CategoryMessage      string
	CategoryReviewReady  bool
	AttributeReady       bool
	AttributeMessage     string
	SaleAttributeReady   bool
	SaleAttributeMessage string
}

// BuildSubmitTemplateReadinessChecks builds SHEIN category, attribute, and sale-attribute readiness checks.
func BuildSubmitTemplateReadinessChecks(input SubmitTemplateReadinessInput) []ReadinessCheckSpec {
	return []ReadinessCheckSpec{
		BuildSubmitReadinessCheck(
			"category",
			"类目骨架",
			input.CategoryReady,
			input.CategoryMessage,
			[]string{"shein.category_id", "shein.category_id_list", "shein.product_type_id", "shein.sale_attribute_resolution.category_review_reason"},
			"确认类目",
			false,
		),
		BuildSubmitReadinessCheck(
			"category_review",
			"类目复核",
			input.CategoryReviewReady,
			"当前类目仍被建议复核，提交前必须先确认 SHEIN 类目是否匹配",
			[]string{"shein.category_resolution.suggested_category", "shein.sale_attribute_resolution.category_review_reason"},
			"复核类目",
			false,
		),
		BuildSubmitReadinessCheck(
			"attributes",
			"普通属性",
			input.AttributeReady,
			input.AttributeMessage,
			[]string{"shein.resolved_attributes", "shein.request_draft.resolved_attributes"},
			"确认属性",
			false,
		),
		BuildSubmitReadinessCheck(
			"attribute_review",
			"属性复核",
			input.AttributeReady,
			"普通属性仍有模板必填项未确认，提交前必须补齐或人工确认",
			[]string{"shein.attribute_resolution.pending_attributes", "shein.attribute_resolution.review_notes"},
			"复核属性",
			false,
		),
		BuildSubmitReadinessCheck(
			"sale_attributes",
			"销售属性",
			input.SaleAttributeReady,
			input.SaleAttributeMessage,
			[]string{"shein.sale_attribute_resolution", "shein.request_draft.skc_list"},
			"确认规格",
			false,
		),
	}
}
