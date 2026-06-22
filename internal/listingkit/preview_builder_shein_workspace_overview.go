package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

func buildSheinPreviewWorkspaceOverview(statusOverview *sheinworkspace.StatusOverview, submitState *sheinworkspace.SubmitStateInput, repairCenter *SheinRepairCenter) *sheinworkspace.WorkspaceOverview {
	repairState := sheinworkspace.BuildRepairStateInput(repairCenter)
	return sheinworkspace.BuildWorkspaceOverview(statusOverview, submitState, repairState)
}
