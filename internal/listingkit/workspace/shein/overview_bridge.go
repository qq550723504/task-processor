package shein

import sheinworkspace "task-processor/internal/workspace/shein"

type SubmitStateInput = sheinworkspace.SubmitStateInput
type RepairStateInput = sheinworkspace.RepairStateInput
type StatusOverview = sheinworkspace.StatusOverview
type WorkspaceOverview = sheinworkspace.WorkspaceOverview
type WorkspaceSessionEntry = sheinworkspace.WorkspaceSessionEntry
type WorkspaceSubmitState = sheinworkspace.WorkspaceSubmitState
type WorkspaceRepairState = sheinworkspace.WorkspaceRepairState

func BuildSubmitStateInput[R any, H any](readiness *SubmitReadiness[R, H]) *SubmitStateInput {
	return sheinworkspace.BuildSubmitStateInput(readiness)
}

func BuildRepairStateInput[R any, P any, S any, Q any, V any](center *RepairCenter[R, P, S, Q, V]) *RepairStateInput {
	return sheinworkspace.BuildRepairStateInput(center)
}

func BuildStatusOverview(inspection *Inspection, readiness *SubmitStateInput) *StatusOverview {
	return sheinworkspace.BuildStatusOverview(inspection, readiness)
}

func BuildWorkspaceOverview(status *StatusOverview, readiness *SubmitStateInput, repair *RepairStateInput) *WorkspaceOverview {
	return sheinworkspace.BuildWorkspaceOverview(status, readiness, repair)
}
