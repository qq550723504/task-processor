package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
)

type Guidance[R any, H any] = sheinmarketplace.Guidance[R, H]
type ReadinessCheckSpec = sheinmarketplace.ReadinessCheckSpec
type SubmitReadiness[R any, H any] = sheinmarketplace.SubmitReadiness[R, H]
type ReadinessItem[R any, H any] = sheinmarketplace.ReadinessItem[R, H]
type ReadinessCheck[R any, H any] = sheinmarketplace.ReadinessCheck[R, H]
type SubmitChecklist[R any, H any] = sheinmarketplace.SubmitChecklist[R, H]
type ChecklistGroupItem[R any, H any] = sheinmarketplace.ChecklistGroupItem[R, H]

func BuildSubmitReadiness[R any, H any](
	checks []ReadinessCheckSpec,
	guidanceResolver func(ReadinessCheckSpec) Guidance[R, H],
	blockedSummary string,
	warningSummary string,
	readySummary string,
) *SubmitReadiness[R, H] {
	return sheinmarketplace.BuildSubmitReadiness(checks, guidanceResolver, blockedSummary, warningSummary, readySummary)
}

func BuildSubmitChecklist[R any, H any](readiness *SubmitReadiness[R, H], groupForCheck func(string) string) *SubmitChecklist[R, H] {
	return sheinmarketplace.BuildSubmitChecklist(readiness, groupForCheck)
}

func ChecklistItemCount[R any, H any](checklist *SubmitChecklist[R, H]) int {
	return sheinmarketplace.ChecklistItemCount(checklist)
}

func SubmitChecklistGroupForKey(key string) string {
	return sheinmarketplace.SubmitChecklistGroupForKey(key)
}

func FindLabels[R any, H any](items []ReadinessItem[R, H]) []string {
	return sheinmarketplace.FindLabels(items)
}

func FindKeys[R any, H any](items []ReadinessItem[R, H]) []string {
	return sheinmarketplace.FindKeys(items)
}

func CloneReadinessItems[R any, H any](items []ReadinessItem[R, H]) []ReadinessItem[R, H] {
	return sheinmarketplace.CloneReadinessItems(items)
}

func ToActionItems[R any, H any](items []ReadinessItem[R, H]) []ActionItem {
	return sheinmarketplace.ToActionItems(items)
}

func JoinReadinessLabels[R any, H any](items []ReadinessItem[R, H], sep string) string {
	return sheinmarketplace.JoinReadinessLabels(items, sep)
}
