package listingkit

import sheinworkspace "task-processor/internal/listingkit/workspace/shein"

func buildSheinPreviewWorkspaceOverview(statusOverview *sheinworkspace.StatusOverview, submitState *sheinworkspace.SubmitStateInput, repairCenter *SheinRepairCenter) *sheinworkspace.WorkspaceOverview {
	repairState := sheinworkspace.BuildRepairStateInput(repairCenter)
	return sheinworkspace.BuildWorkspaceOverview(statusOverview, submitState, repairState)
}
