package workspace

import sheinpub "task-processor/internal/publishing/shein"

// BuildSubmitPayloadReadinessChecks builds SHEIN payload readiness checks from package state.
func BuildSubmitPayloadReadinessChecks(pkg *sheinpub.Package, action string) []ReadinessCheckSpec {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	checks := make([]ReadinessCheckSpec, 0, 11)

	requestDraftReady := pkg != nil && pkg.DraftPayload != nil
	checks = append(checks, BuildSubmitReadinessCheck(
		"request_draft",
		"请求草稿",
		requestDraftReady,
		"request_draft 尚未生成，当前无法作为提交草稿继续流转",
		[]string{"shein.request_draft"},
		"重新生成预览草稿",
		false,
	))

	previewProductReady := pkg != nil && pkg.PreviewPayload != nil
	checks = append(checks, BuildSubmitReadinessCheck(
		"preview_product",
		"预览载荷",
		previewProductReady,
		"preview_product 尚未生成，当前还不能进入提交前预览校验",
		[]string{"shein.preview_product"},
		"重建预览载荷",
		false,
	))

	checks = append(checks, BuildSubmitReadinessCheck(
		"images",
		"主图资产",
		sheinpub.HasSubmitImage(pkg),
		"主图还没有准备好，提交前至少需要一张可用主图",
		[]string{"shein.images.main_image", "shein.request_draft.image_info.main_image"},
		"补充主图",
		false,
	))

	finalImagesReady, finalImagesMessage := sheinpub.FinalSubmitImagesReady(pkg, action)
	checks = append(checks, BuildSubmitReadinessCheck(
		"final_images",
		"最终图片",
		finalImagesReady,
		finalImagesMessage,
		[]string{"shein.final_draft.main_image_url", "shein.final_draft.image_role_overrides", "shein.preview_product.image_info"},
		"确认图片",
		false,
	))

	coverageMessage, coverageBlocked := sheinpub.VariantImageCoverageStatus(pkg)
	checks = append(checks, BuildSubmitReadinessCheck(
		"variant_image_coverage",
		"变体图片覆盖",
		!coverageBlocked,
		firstNonEmpty(coverageMessage, "变体图片覆盖不完整，请为每个颜色规格补齐独立商品图后再提交"),
		[]string{"shein.metadata.variant_image_coverage_status", "shein.metadata.variant_image_coverage_message", "shein.request_draft.skc_list"},
		"确认图片",
		false,
	))

	checks = append(checks, BuildSubmitReadinessCheck(
		"variants",
		"规格结构",
		submitVariantStructureReady(pkg),
		"当前还没有完整的 SKC/SKU 结构，提交前需要至少一个 SKC 和一个 SKU",
		[]string{"shein.skc_list", "shein.request_draft.skc_list"},
		"补充规格",
		false,
	))

	pricingReady := pkg == nil || pkg.Pricing == nil || sheinpub.SubmitPricingReady(pkg)
	checks = append(checks, BuildSubmitReadinessCheck(
		"pricing",
		"价格确认",
		pricingReady,
		"SKU 价格尚未全部生成或确认，提交前需要完成价格规则预览和人工覆盖确认",
		[]string{"shein.pricing", "shein.request_draft.skc_list.sku_list.base_price"},
		"确认价格",
		false,
	))

	checks = append(checks, BuildSubmitReadinessCheck(
		"final_review",
		"最终确认",
		sheinpub.FinalReviewReady(pkg, action),
		sheinpub.FinalReviewMessage(action),
		[]string{"shein.final_draft", "shein.final_review"},
		"最终确认",
		false,
	))

	checks = append(checks, BuildManualNotesReadinessCheck(pkg))
	checks = append(checks, BuildSourceFactsReadinessCheck(pkg))
	return checks
}

func submitVariantStructureReady(pkg *sheinpub.Package) bool {
	if pkg == nil {
		return false
	}
	skcCount := len(pkg.SkcList)
	if skcCount == 0 && pkg.DraftPayload != nil {
		skcCount = len(pkg.DraftPayload.SKCList)
	}
	return skcCount > 0 && sheinpub.HasAnySubmitSKU(pkg)
}
