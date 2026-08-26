package imageagent

import "strings"

type Action string

const (
	ActionEditPlan       Action = "edit_plan"
	ActionRetrySlot      Action = "retry_slot"
	ActionApproveResults Action = "approve_results"
	ActionCancel         Action = "cancel"
	ActionSwitchManual   Action = "switch_manual"
)

func AllowedActions(run Run) []Action {
	if run.Mode != RunModeManual {
		return nil
	}
	switch run.Status {
	case RunStatusBlocked:
		if run.Block == nil || strings.TrimSpace(run.Block.Code) == "" || strings.TrimSpace(run.Block.SlotID) == "" {
			return []Action{ActionCancel}
		}
		return []Action{ActionEditPlan, ActionRetrySlot, ActionCancel}
	case RunStatusAwaitingFinalApproval:
		return []Action{ActionApproveResults, ActionCancel}
	case RunStatusCompleted, RunStatusFailed, RunStatusCancelled:
		return nil
	default:
		return nil
	}
}
