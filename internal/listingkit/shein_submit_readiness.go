package listingkit

import (
	"strings"

	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

func buildSheinSubmitReadiness(pkg *SheinPackage) *SheinSubmitReadiness {
	return buildSheinSubmitReadinessWithPodForAction(pkg, nil, "publish")
}

func buildSheinSubmitReadinessForAction(pkg *SheinPackage, action string) *SheinSubmitReadiness {
	return buildSheinSubmitReadinessWithPodForAction(pkg, nil, action)
}

func buildSheinSubmitReadinessWithPod(pkg *SheinPackage, pod *PodExecutionSummary) *SheinSubmitReadiness {
	return buildSheinSubmitReadinessWithPodForAction(pkg, pod, "publish")
}

func buildSheinSubmitReadinessWithPodForAction(pkg *SheinPackage, pod *PodExecutionSummary, action string) *SheinSubmitReadiness {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return nil
	}
	pod = normalizePodExecutionSummary(clonePodExecutionSummary(pod))
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		action = "publish"
	}

	validation := ValidateSheinPackageAgainstTemplates(pkg)
	checks := buildSheinSubmitReadinessChecks(pkg, pod, action, validation)

	readiness := sheinworkspace.BuildSubmitReadiness(
		checks,
		buildSheinSubmitReadinessGuidanceResolver(pkg),
		"当前仍有关键字段未完成，SHEIN 资料包还不能直接进入提交态",
		"SHEIN 资料包已经基本可提交，但仍建议先处理人工备注",
		"SHEIN 资料包已具备提交前所需的关键骨架",
	)
	if readiness == nil {
		return nil
	}
	return shapeSheinSubmitReadinessSummary(readiness, sheinSubmitReadinessSummaryShape{
		blockingLabel: "待补关键项：",
		warningLabel:  "待确认项：",
	})
}

func buildSheinSubmitReadinessGuidanceResolver(
	pkg *SheinPackage,
) func(spec sheinworkspace.ReadinessCheckSpec) sheinworkspace.Guidance[SheinReadinessReason, SheinRepairHint] {
	return func(spec sheinworkspace.ReadinessCheckSpec) sheinworkspace.Guidance[SheinReadinessReason, SheinRepairHint] {
		return buildSheinReadinessGuidance(pkg, spec.Key, spec.FieldPaths, spec.SuggestedAction, spec.WarningOnly)
	}
}

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

func shapeSheinSubmitReadinessSummary(
	readiness *SheinSubmitReadiness,
	shape sheinSubmitReadinessSummaryShape,
) *SheinSubmitReadiness {
	if readiness == nil {
		return nil
	}
	if len(readiness.BlockingItems) > 0 {
		if shape.prependFirstBlocker {
			if message := strings.TrimSpace(readiness.BlockingItems[0].Message); message != "" {
				readiness.Summary = append([]string{message}, readiness.Summary...)
			}
		}
		if label := strings.TrimSpace(shape.blockingLabel); label != "" {
			readiness.Summary = append(readiness.Summary, label+sheinworkspace.JoinReadinessLabels(readiness.BlockingItems, "、"))
		}
	}
	if len(readiness.WarningItems) > 0 {
		if label := strings.TrimSpace(shape.warningLabel); label != "" {
			readiness.Summary = append(readiness.Summary, label+sheinworkspace.JoinReadinessLabels(readiness.WarningItems, "、"))
		}
	}
	readiness.Summary = uniqueStrings(readiness.Summary)
	return readiness
}
