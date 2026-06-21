package workspace

import sheinpub "task-processor/internal/publishing/shein"

const (
	categoryTemplateValidationMessage      = "类目、类目层级和 product_type_id 需要确认；如当前类目被建议复核，也不能直接进入提交态"
	attributeTemplateValidationMessage     = "普通属性还没有全部映射到真实 attribute_id / attribute_value_id，或仍存在模板必填/重要属性未确认"
	saleAttributeTemplateValidationMessage = "销售属性主副规格还没有稳定映射到真实 sale attribute/value，或当前类目/规格组合仍需复核"
	submitPayloadTemplateValidationMessage = "发布载荷结构需要满足 SHEIN 提交要求，包括 SKC 图片、方形图、SKU 数量/包装/仓库/尺寸字段"
)

// PackageTemplateValidation summarizes SHEIN template and prepared payload readiness inputs.
type PackageTemplateValidation struct {
	CategoryReady        bool
	CategoryReviewReady  bool
	CategoryMessage      string
	AttributeReady       bool
	AttributeMessage     string
	SaleAttributeReady   bool
	SaleAttributeMessage string
	SubmitPayloadReady   bool
	SubmitPayloadMessage string
}

// BuildPackageTemplateValidation builds SHEIN template validation state from package data and prepared payload validation.
func BuildPackageTemplateValidation(pkg *sheinpub.Package, submitPayloadErr error) PackageTemplateValidation {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	result := PackageTemplateValidation{
		CategoryReady:        pkg != nil && IsCategoryResolved(pkg) && pkg.CategoryID > 0 && pkg.ProductTypeID != nil && *pkg.ProductTypeID > 0,
		CategoryReviewReady:  !sheinpub.SaleAttributeReviewPending(pkg),
		CategoryMessage:      categoryTemplateValidationMessage,
		AttributeReady:       IsAttributeResolved(pkg) && !sheinpub.HasBlockingPendingAttributes(pkg),
		AttributeMessage:     attributeTemplateValidationMessage,
		SaleAttributeReady:   sheinpub.SaleAttributesReadyForSubmit(pkg),
		SaleAttributeMessage: saleAttributeTemplateValidationMessage,
		SubmitPayloadReady:   true,
		SubmitPayloadMessage: submitPayloadTemplateValidationMessage,
	}
	if submitPayloadErr != nil {
		result.SubmitPayloadReady = false
		result.SubmitPayloadMessage = submitPayloadErr.Error()
	}
	return result
}
