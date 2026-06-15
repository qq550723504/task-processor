package listingkit

import (
	"strings"

	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
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
		guidance := buildSheinReadinessGuidance(pkg, spec.Key, spec.FieldPaths, spec.SuggestedAction, spec.WarningOnly)
		return sheinworkspace.Guidance[SheinReadinessReason, SheinRepairHint]{
			Reason:      cloneSheinReadinessReason(guidance.reason),
			RepairHints: cloneSheinRepairHints(guidance.repairHints),
		}
	}
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
