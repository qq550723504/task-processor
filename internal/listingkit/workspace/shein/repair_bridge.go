package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type RepairCenterSeedAction[R any, P any, S any, Q any, V any] = sheinmarketplace.RepairCenterSeedAction[R, P, S, Q, V]
type RepairHintAccessors[H any, P any, S any, Q any, V any] = sheinmarketplace.RepairHintAccessors[H, P, S, Q, V]
type RepairCenterFromReadinessOptions[R any, P any, S any, Q any, V any] = sheinmarketplace.RepairCenterFromReadinessOptions[R, P, S, Q, V]
type RepairValidationPreview[FieldError any] = sheinmarketplace.RepairValidationPreview[FieldError]
type RepairSessionActionInfo = sheinmarketplace.RepairSessionActionInfo

func BuildRepairValidationPreview[FieldError any](
	pkg *Package,
	editorSection string,
	skeleton *EditorRevisionSkeleton,
	valid bool,
	fieldErrors []FieldError,
) *RepairValidationPreview[FieldError] {
	return sheinmarketplace.BuildRepairValidationPreview(pkg, editorSection, skeleton, valid, fieldErrors)
}

func BuildRepairCenter[R any, P any, S any, Q any, V any](
	seeds []RepairCenterSeedAction[R, P, S, Q, V],
	changeCount func(*V) int,
	isInvalid func(*V) bool,
	reasonSummary func(*R) string,
	actionInfo func(RepairCenterAction[R, P, S, Q, V]) RepairSessionActionInfo,
) *RepairCenter[R, P, S, Q, V] {
	return sheinmarketplace.BuildRepairCenter(seeds, changeCount, isInvalid, reasonSummary, actionInfo)
}

func BuildRepairCenterFromReadiness[R any, H any, P any, S any, Q any, V any](
	readiness *SubmitReadiness[R, H],
	checklist *SubmitChecklist[R, H],
	accessors RepairHintAccessors[H, P, S, Q, V],
	options RepairCenterFromReadinessOptions[R, P, S, Q, V],
) *RepairCenter[R, P, S, Q, V] {
	return sheinmarketplace.BuildRepairCenterFromReadiness(readiness, checklist, accessors, options)
}
