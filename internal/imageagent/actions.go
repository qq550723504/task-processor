package imageagent

import "strings"

type Action string

const (
	ActionEditPlan       Action = "edit_plan"
	ActionRetrySlot      Action = "retry_slot"
	ActionApproveResults Action = "approve_results"
	ActionCancel         Action = "cancel"
	ActionRestart        Action = "restart"
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
		if policy, ok := SlotEffectV3BlockedPolicyForCode(run.Block.Code); ok {
			return append([]Action(nil), policy.PermittedActions...)
		}
		return []Action{ActionEditPlan, ActionRetrySlot, ActionCancel}
	case RunStatusAwaitingFinalApproval:
		return []Action{ActionApproveResults, ActionCancel}
	case RunStatusFailed:
		return []Action{ActionRestart}
	case RunStatusCompleted, RunStatusCancelled:
		return nil
	default:
		return nil
	}
}

func BlockAllowsAction(block *Block, action Action) bool {
	if block == nil {
		return false
	}
	for _, allowed := range AllowedActions(Run{Mode: RunModeManual, Status: RunStatusBlocked, Block: block}) {
		if allowed == action {
			return true
		}
	}
	return false
}
