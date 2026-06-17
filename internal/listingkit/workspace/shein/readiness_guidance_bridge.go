package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type Guidance[R any, H any] = sheinmarketplace.Guidance[R, H]
type ReadinessCheckSpec = sheinmarketplace.ReadinessCheckSpec
type ReadinessTaxonomy = sheinmarketplace.ReadinessTaxonomy
type ReadinessReasonSpec = sheinmarketplace.ReadinessReasonSpec
type ReadinessHintSpec = sheinmarketplace.ReadinessHintSpec
type ReadinessGuidanceSpec = sheinmarketplace.ReadinessGuidanceSpec

func BuildReadinessGuidanceSpec(key string, warningOnly bool) *ReadinessGuidanceSpec {
	return sheinmarketplace.BuildReadinessGuidanceSpec(key, warningOnly)
}

func BuildSubmitReadiness[R any, H any](
	checks []ReadinessCheckSpec,
	guidanceResolver func(ReadinessCheckSpec) Guidance[R, H],
	blockedSummary string,
	warningSummary string,
	readySummary string,
) *SubmitReadiness[R, H] {
	return sheinmarketplace.BuildSubmitReadiness(checks, guidanceResolver, blockedSummary, warningSummary, readySummary)
}

func JoinReadinessLabels[R any, H any](items []ReadinessItem[R, H], sep string) string {
	return sheinmarketplace.JoinReadinessLabels(items, sep)
}

func FindKeys[R any, H any](items []ReadinessItem[R, H]) []string {
	return sheinmarketplace.FindKeys(items)
}

func CloneReadinessItems[R any, H any](items []ReadinessItem[R, H]) []ReadinessItem[R, H] {
	return sheinmarketplace.CloneReadinessItems(items)
}
