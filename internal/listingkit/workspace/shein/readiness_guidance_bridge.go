package shein

import sheinworkspace "task-processor/internal/workspace/shein"

type Guidance[R any, H any] = sheinworkspace.Guidance[R, H]
type ReadinessCheckSpec = sheinworkspace.ReadinessCheckSpec
type ReadinessReasonSpec = sheinworkspace.ReadinessReasonSpec
type ReadinessHintSpec = sheinworkspace.ReadinessHintSpec
type ReadinessGuidanceSpec = sheinworkspace.ReadinessGuidanceSpec

func BuildReadinessGuidanceSpec(key string, warningOnly bool) *ReadinessGuidanceSpec {
	return sheinworkspace.BuildReadinessGuidanceSpec(key, warningOnly)
}

func BuildSubmitReadiness[R any, H any](
	checks []ReadinessCheckSpec,
	guidanceResolver func(ReadinessCheckSpec) Guidance[R, H],
	blockedSummary string,
	warningSummary string,
	readySummary string,
) *SubmitReadiness[R, H] {
	return sheinworkspace.BuildSubmitReadiness(checks, guidanceResolver, blockedSummary, warningSummary, readySummary)
}

func JoinReadinessLabels[R any, H any](items []ReadinessItem[R, H], sep string) string {
	return sheinworkspace.JoinReadinessLabels(items, sep)
}

func FindKeys[R any, H any](items []ReadinessItem[R, H]) []string {
	return sheinworkspace.FindKeys(items)
}

func CloneReadinessItems[R any, H any](items []ReadinessItem[R, H]) []ReadinessItem[R, H] {
	return sheinworkspace.CloneReadinessItems(items)
}
