package listingkit

import (
	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
	sheinpub "task-processor/internal/publishing/shein"
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
	return sheinworkspace.BuildSubmitReadinessCheck(key, label, ok, message, fieldPaths, suggestedAction, warningOnly)
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
	return append(checks, sheinworkspace.BuildSubmitPayloadReadinessChecks(pkg, action)...)
}

func sheinSubmitReadinessFinalDraftReady(pkg *SheinPackage, action string) bool {
	return sheinpub.FinalReviewReady(pkg, action)
}

func sheinSubmitReadinessFinalReviewMessage(action string) string {
	return sheinpub.FinalReviewMessage(action)
}
