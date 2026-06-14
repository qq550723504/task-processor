package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type SubmitStateInput = sheinmarketplace.SubmitStateInput
type RepairStateInput = sheinmarketplace.RepairStateInput
type StatusOverview = sheinmarketplace.StatusOverview
type WorkspaceOverview = sheinmarketplace.WorkspaceOverview
type WorkspaceSessionEntry = sheinmarketplace.WorkspaceSessionEntry
type WorkspaceSubmitState = sheinmarketplace.WorkspaceSubmitState
type WorkspaceRepairState = sheinmarketplace.WorkspaceRepairState

func BuildSubmitStateInput[R any, H any](readiness *SubmitReadiness[R, H]) *SubmitStateInput {
	return sheinmarketplace.BuildSubmitStateInput(readiness)
}

func BuildRepairStateInput[R any, P any, S any, Q any, V any](center *RepairCenter[R, P, S, Q, V]) *RepairStateInput {
	return sheinmarketplace.BuildRepairStateInput(center)
}

func BuildStatusOverview(inspection *Inspection, readiness *SubmitStateInput) *StatusOverview {
	return sheinmarketplace.BuildStatusOverview(inspection, readiness)
}

func BuildWorkspaceOverview(status *StatusOverview, readiness *SubmitStateInput, repair *RepairStateInput) *WorkspaceOverview {
	return sheinmarketplace.BuildWorkspaceOverview(status, readiness, repair)
}
