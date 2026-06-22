package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

func buildSheinSubmitReadinessChecks(pkg *SheinPackage, pod *PodExecutionSummary, action string, validation sheinBuildValidation) []sheinworkspace.ReadinessCheckSpec {
	checks := make([]sheinworkspace.ReadinessCheckSpec, 0, 8)
	checks = append(checks, sheinworkspace.BuildSubmitReadinessCheck(
		sheinCookieUnavailableIssueCode,
		"SHEIN 店铺登录",
		!sheinworkspace.HasCookieUnavailableReviewNotes(pkg),
		"SHEIN 店铺 cookie 不可用。标准商品和 SDS 图片仍可继续使用，但 SHEIN 平台在线类目、属性解析和提交已在平台阶段受阻，请先重新登录店铺后再继续。",
		[]string{"shein.review_notes", "shein.category_resolution.review_notes", "shein.attribute_resolution.review_notes", "shein.sale_attribute_resolution.review_notes"},
		"重新登录 SHEIN 店铺",
		false,
	))
	checks = appendSheinPodReadinessChecks(checks, pod, action)
	checks = append(checks, sheinworkspace.BuildSubmitTemplateReadinessChecks(sheinworkspace.SubmitTemplateReadinessInput{
		CategoryReady:        validation.categoryReady,
		CategoryMessage:      validation.categoryMessage,
		CategoryReviewReady:  validation.categoryReviewReady,
		AttributeReady:       validation.attributeReady,
		AttributeMessage:     validation.attributeMessage,
		SaleAttributeReady:   validation.saleAttributeReady,
		SaleAttributeMessage: validation.saleAttributeMessage,
	})...)
	checks = append(checks, sheinworkspace.BuildSubmitPayloadReadinessChecks(pkg, action)...)
	checks = appendSheinBuildValidationChecks(checks, validation)
	return checks
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
	return append(checks, sheinworkspace.BuildSubmitReadinessCheck(
		"pod_platform",
		"POD 平台处理",
		!podBlocked && (action == "save_draft" || (pod.Status != podStatusFailedDegraded && pod.Status != podStatusBypassed)),
		firstNonEmptyString(podMessage, "POD 平台处理状态尚未满足发布要求"),
		[]string{"pod_execution"},
		"处理 POD 平台结果",
		action == "save_draft" || !podBlocked,
	))
}
