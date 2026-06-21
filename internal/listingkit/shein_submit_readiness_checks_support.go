package listingkit

import (
	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
)

func buildSheinSubmitReadinessChecks(pkg *SheinPackage, pod *PodExecutionSummary, action string, validation sheinBuildValidation) []sheinworkspace.ReadinessCheckSpec {
	checks := make([]sheinworkspace.ReadinessCheckSpec, 0, 8)
	checks = append(checks, sheinSubmitReadinessCheck(
		sheinCookieUnavailableIssueCode,
		"SHEIN 店铺登录",
		!sheinCookieUnavailable(pkg),
		"SHEIN 店铺 cookie 不可用。标准商品和 SDS 图片仍可继续使用，但 SHEIN 平台在线类目、属性解析和提交已在平台阶段受阻，请先重新登录店铺后再继续。",
		[]string{"shein.review_notes", "shein.category_resolution.review_notes", "shein.attribute_resolution.review_notes", "shein.sale_attribute_resolution.review_notes"},
		"重新登录 SHEIN 店铺",
		false,
	))
	checks = appendSheinPodReadinessChecks(checks, pod, action)
	checks = appendSheinTemplateReadinessChecks(checks, validation)
	checks = appendSheinPayloadReadinessChecks(checks, pkg, action)
	checks = appendSheinBuildValidationChecks(checks, validation)
	return checks
}

func sheinSubmitReadinessCheck(key, label string, ok bool, message string, fieldPaths []string, suggestedAction string, warningOnly bool) sheinworkspace.ReadinessCheckSpec {
	return sheinworkspace.ReadinessCheckSpec{
		Key:             key,
		Label:           label,
		OK:              ok,
		Message:         message,
		FieldPaths:      append([]string(nil), fieldPaths...),
		SuggestedAction: suggestedAction,
		WarningOnly:     warningOnly,
		Taxonomy:        sheinReadinessTaxonomyForKey(key, warningOnly),
	}
}

func sheinReadinessTaxonomyForKey(key string, warningOnly bool) sheinworkspace.ReadinessTaxonomy {
	return sheinworkspace.BuildReadinessTaxonomy(key, warningOnly)
}

func appendSheinPodReadinessChecks(checks []sheinworkspace.ReadinessCheckSpec, pod *PodExecutionSummary, action string) []sheinworkspace.ReadinessCheckSpec {
	if pod == nil || pod.DependencyMode == podDependencyModeDisabled {
		return checks
	}
	podBlocked := action != "save_draft" && podSubmissionBlocked(pod)
	podMessage := podReadinessMessage(pod)
	if action == "save_draft" && pod.Status != podStatusSucceeded {
		podMessage = firstNonEmptyString(podMessage, "POD 平台处理尚未完成；当前允许先保存草稿，正式发布前仍需确认平台结果")
	}
	return append(checks, sheinSubmitReadinessCheck(
		"pod_platform",
		"POD 平台处理",
		!podBlocked && (action == "save_draft" || (pod.Status != podStatusFailedDegraded && pod.Status != podStatusBypassed)),
		firstNonEmptyString(podMessage, "POD 平台处理状态尚未满足发布要求"),
		[]string{"pod_execution"},
		"处理 POD 平台结果",
		action == "save_draft" || !podBlocked,
	))
}

func appendSheinTemplateReadinessChecks(checks []sheinworkspace.ReadinessCheckSpec, validation sheinBuildValidation) []sheinworkspace.ReadinessCheckSpec {
	checks = append(checks, sheinSubmitReadinessCheck(
		"category",
		"类目骨架",
		validation.categoryReady,
		validation.categoryMessage,
		[]string{"shein.category_id", "shein.category_id_list", "shein.product_type_id", "shein.sale_attribute_resolution.category_review_reason"},
		"确认类目",
		false,
	))
	checks = append(checks, sheinSubmitReadinessCheck(
		"category_review",
		"类目复核",
		validation.categoryReviewReady,
		"当前类目仍被建议复核，提交前必须先确认 SHEIN 类目是否匹配",
		[]string{"shein.category_resolution.suggested_category", "shein.sale_attribute_resolution.category_review_reason"},
		"复核类目",
		false,
	))
	checks = append(checks, sheinSubmitReadinessCheck(
		"attributes",
		"普通属性",
		validation.attributeReady,
		validation.attributeMessage,
		[]string{"shein.resolved_attributes", "shein.request_draft.resolved_attributes"},
		"确认属性",
		false,
	))
	checks = append(checks, sheinSubmitReadinessCheck(
		"attribute_review",
		"属性复核",
		validation.attributeReady,
		"普通属性仍有模板必填项未确认，提交前必须补齐或人工确认",
		[]string{"shein.attribute_resolution.pending_attributes", "shein.attribute_resolution.review_notes"},
		"复核属性",
		false,
	))
	checks = append(checks, sheinSubmitReadinessCheck(
		"sale_attributes",
		"销售属性",
		validation.saleAttributeReady,
		validation.saleAttributeMessage,
		[]string{"shein.sale_attribute_resolution", "shein.request_draft.skc_list"},
		"确认规格",
		false,
	))
	return checks
}

func appendSheinPayloadReadinessChecks(checks []sheinworkspace.ReadinessCheckSpec, pkg *SheinPackage, action string) []sheinworkspace.ReadinessCheckSpec {
	requestDraftReady := pkg.DraftPayload != nil
	checks = append(checks, sheinSubmitReadinessCheck(
		"request_draft",
		"请求草稿",
		requestDraftReady,
		"request_draft 尚未生成，当前无法作为提交草稿继续流转",
		[]string{"shein.request_draft"},
		"重新生成预览草稿",
		false,
	))

	previewProductReady := pkg.PreviewPayload != nil
	checks = append(checks, sheinSubmitReadinessCheck(
		"preview_product",
		"预览载荷",
		previewProductReady,
		"preview_product 尚未生成，当前还不能进入提交前预览校验",
		[]string{"shein.preview_product"},
		"重建预览载荷",
		false,
	))

	imageReady := sheinHasSubmitImage(pkg)
	checks = append(checks, sheinSubmitReadinessCheck(
		"images",
		"主图资产",
		imageReady,
		"主图还没有准备好，提交前至少需要一张可用主图",
		[]string{"shein.images.main_image", "shein.request_draft.image_info.main_image"},
		"补充主图",
		false,
	))

	finalImagesReady, finalImagesMessage := sheinFinalImagesReadyForAction(pkg, action)
	checks = append(checks, sheinSubmitReadinessCheck(
		"final_images",
		"最终图片",
		finalImagesReady,
		finalImagesMessage,
		[]string{"shein.final_draft.main_image_url", "shein.final_draft.image_role_overrides", "shein.preview_product.image_info"},
		"确认图片",
		false,
	))

	coverageMessage, coverageBlocked := sheinVariantImageCoverageStatus(pkg)
	checks = append(checks, sheinSubmitReadinessCheck(
		"variant_image_coverage",
		"变体图片覆盖",
		!coverageBlocked,
		firstNonEmptyString(coverageMessage, "变体图片覆盖不完整，请为每个颜色规格补齐独立商品图后再提交"),
		[]string{"shein.metadata.variant_image_coverage_status", "shein.metadata.variant_image_coverage_message", "shein.request_draft.skc_list"},
		"确认图片",
		false,
	))

	skcCount := len(pkg.SkcList)
	if skcCount == 0 && pkg.DraftPayload != nil {
		skcCount = len(pkg.DraftPayload.SKCList)
	}
	checks = append(checks, sheinSubmitReadinessCheck(
		"variants",
		"规格结构",
		skcCount > 0 && sheinHasAnySKU(pkg),
		"当前还没有完整的 SKC/SKU 结构，提交前需要至少一个 SKC 和一个 SKU",
		[]string{"shein.skc_list", "shein.request_draft.skc_list"},
		"补充规格",
		false,
	))

	pricingReady := pkg.Pricing == nil || sheinPricingReady(pkg)
	checks = append(checks, sheinSubmitReadinessCheck(
		"pricing",
		"价格确认",
		pricingReady,
		"SKU 价格尚未全部生成或确认，提交前需要完成价格规则预览和人工覆盖确认",
		[]string{"shein.pricing", "shein.request_draft.skc_list.sku_list.base_price"},
		"确认价格",
		false,
	))

	checks = append(checks, sheinSubmitReadinessCheck(
		"final_review",
		"最终确认",
		sheinSubmitReadinessFinalDraftReady(pkg, action),
		sheinSubmitReadinessFinalReviewMessage(action),
		[]string{"shein.final_draft", "shein.final_review"},
		"最终确认",
		false,
	))

	manualNotes := filterManualSheinReviewNotes(pkg.ReviewNotes)
	checks = append(checks, sheinSubmitReadinessCheck(
		"manual_notes",
		"人工备注",
		len(manualNotes) == 0,
		"仍有人工备注未处理，建议在提交前再次确认",
		[]string{"shein.review_notes"},
		"处理备注",
		true,
	))

	var sourceMetadata map[string]string
	if pkg != nil {
		sourceMetadata = pkg.Metadata
	}
	sourceFactsReady, sourceFactsMessage := sheinworkspace.SourceFactsReady(sourceMetadata)
	checks = append(checks, sheinSubmitReadinessCheck(
		"source_facts",
		"来源事实",
		sourceFactsReady,
		sourceFactsMessage,
		[]string{"shein.metadata.source_fact_review_required", "shein.metadata.source_fact_review_fields"},
		"复核来源事实",
		false,
	))

	return checks
}

func sheinSubmitReadinessFinalDraftReady(pkg *SheinPackage, action string) bool {
	if action == "save_draft" {
		return true
	}
	return pkg.FinalSubmissionDraft == nil || pkg.FinalSubmissionDraft.Confirmed
}

func sheinSubmitReadinessFinalReviewMessage(action string) string {
	if action == "save_draft" {
		return "保存草稿允许跳过最终确认；正式发布前仍需在最终确认页核对图片、价格、属性和 SKU"
	}
	return "提交前必须在最终确认页核对图片、价格、属性和 SKU 后确认"
}
