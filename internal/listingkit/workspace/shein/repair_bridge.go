package shein

import sheinworkspace "task-processor/internal/workspace/shein"

type RepairCenterSeedAction[R any, P any, S any, Q any, V any] = sheinworkspace.RepairCenterSeedAction[R, P, S, Q, V]
type RepairValidationPreview[FieldError any] = sheinworkspace.RepairValidationPreview[FieldError]
type RepairSessionActionInfo = sheinworkspace.RepairSessionActionInfo

func BuildRepairValidationPreview[FieldError any](
	pkg *Package,
	editorSection string,
	skeleton *EditorRevisionSkeleton,
	valid bool,
	fieldErrors []FieldError,
) *RepairValidationPreview[FieldError] {
	return sheinworkspace.BuildRepairValidationPreview(pkg, editorSection, skeleton, valid, fieldErrors)
}

func BuildRepairCenter[R any, P any, S any, Q any, V any](
	seeds []RepairCenterSeedAction[R, P, S, Q, V],
	changeCount func(*V) int,
	isInvalid func(*V) bool,
	reasonSummary func(*R) string,
	actionInfo func(RepairCenterAction[R, P, S, Q, V]) RepairSessionActionInfo,
) *RepairCenter[R, P, S, Q, V] {
	return sheinworkspace.BuildRepairCenter(seeds, changeCount, isInvalid, reasonSummary, actionInfo)
}
