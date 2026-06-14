package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type RepairCenter[R any, P any, S any, Q any, V any] = sheinmarketplace.RepairCenter[R, P, S, Q, V]
type RepairCenterSeedAction[R any, P any, S any, Q any, V any] = sheinmarketplace.RepairCenterSeedAction[R, P, S, Q, V]
type RepairCenterStats = sheinmarketplace.RepairCenterStats
type RepairCenterSection = sheinmarketplace.RepairCenterSection
type RepairCenterAction[R any, P any, S any, Q any, V any] = sheinmarketplace.RepairCenterAction[R, P, S, Q, V]
type RepairPlan = sheinmarketplace.RepairPlan
type RepairPlanStep = sheinmarketplace.RepairPlanStep
type RepairApplyQueue[Q any, V any] = sheinmarketplace.RepairApplyQueue[Q, V]
type RepairApplyQueueItem[Q any, V any] = sheinmarketplace.RepairApplyQueueItem[Q, V]
type RepairSession = sheinmarketplace.RepairSession
type RepairResumeState = sheinmarketplace.RepairResumeState
type RepairCompletionSnapshot = sheinmarketplace.RepairCompletionSnapshot
type RepairRunbookStep = sheinmarketplace.RepairRunbookStep
type RepairSessionActionInfo = sheinmarketplace.RepairSessionActionInfo

func RepairCenterActionCount[R any, P any, S any, Q any, V any](center *RepairCenter[R, P, S, Q, V]) int {
	return sheinmarketplace.RepairCenterActionCount(center)
}

func RepairCenterDirectApplyCount[R any, P any, S any, Q any, V any](center *RepairCenter[R, P, S, Q, V]) int {
	return sheinmarketplace.RepairCenterDirectApplyCount(center)
}

func RepairCenterPrimaryPlanStatus[R any, P any, S any, Q any, V any](center *RepairCenter[R, P, S, Q, V]) string {
	return sheinmarketplace.RepairCenterPrimaryPlanStatus(center)
}

func RepairCenterSessionStatus[R any, P any, S any, Q any, V any](center *RepairCenter[R, P, S, Q, V]) string {
	return sheinmarketplace.RepairCenterSessionStatus(center)
}

func BuildRepairPlan[R any, P any, S any, Q any, V any](
	actions []RepairCenterAction[R, P, S, Q, V],
	changeCount func(*V) int,
	isInvalid func(*V) bool,
	reasonSummary func(*R) string,
) *RepairPlan {
	return sheinmarketplace.BuildRepairPlan(actions, changeCount, isInvalid, reasonSummary)
}

func BuildRepairApplyQueue[R any, P any, S any, Q any, V any](
	actions []RepairCenterAction[R, P, S, Q, V],
) *RepairApplyQueue[Q, V] {
	return sheinmarketplace.BuildRepairApplyQueue(actions)
}

func BuildRepairSession(
	plan *RepairPlan,
	actionInfo []RepairSessionActionInfo,
) *RepairSession {
	return sheinmarketplace.BuildRepairSession(plan, actionInfo)
}

func BuildRepairCenterStatus(stats *RepairCenterStats) string {
	return sheinmarketplace.BuildRepairCenterStatus(stats)
}

func BuildRepairCenterSummary[R any, P any, S any, Q any, V any](center *RepairCenter[R, P, S, Q, V]) []string {
	return sheinmarketplace.BuildRepairCenterSummary(center)
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
