package listingkit

type SheinRepairPlan struct {
	Status           string                `json:"status,omitempty"`
	Headline         string                `json:"headline,omitempty"`
	Subheadline      string                `json:"subheadline,omitempty"`
	PrimaryStepID    string                `json:"primary_step_id,omitempty"`
	TotalSteps       int                   `json:"total_steps,omitempty"`
	DirectApplySteps int                   `json:"direct_apply_steps,omitempty"`
	ManualSteps      int                   `json:"manual_steps,omitempty"`
	Steps            []SheinRepairPlanStep `json:"steps,omitempty"`
}

type SheinRepairPlanStep struct {
	ID               string   `json:"id,omitempty"`
	ActionID         string   `json:"action_id,omitempty"`
	Order            int      `json:"order,omitempty"`
	Label            string   `json:"label,omitempty"`
	Status           string   `json:"status,omitempty"`
	Priority         string   `json:"priority,omitempty"`
	EditorSection    string   `json:"editor_section,omitempty"`
	ExecutionMode    string   `json:"execution_mode,omitempty"`
	CanApplyDirectly bool     `json:"can_apply_directly"`
	Reason           string   `json:"reason,omitempty"`
	Highlights       []string `json:"highlights,omitempty"`
}

type SheinRepairApplyQueue struct {
	Status          string                      `json:"status,omitempty"`
	DirectApplyOnly bool                        `json:"direct_apply_only"`
	TotalActions    int                         `json:"total_actions,omitempty"`
	ReadyActions    int                         `json:"ready_actions,omitempty"`
	Items           []SheinRepairApplyQueueItem `json:"items,omitempty"`
}

type SheinRepairApplyQueueItem struct {
	Order         int                           `json:"order,omitempty"`
	ActionID      string                        `json:"action_id,omitempty"`
	Label         string                        `json:"label,omitempty"`
	EditorSection string                        `json:"editor_section,omitempty"`
	Revision      *ApplyRevisionRequest         `json:"revision,omitempty"`
	Validation    *SheinRepairValidationPreview `json:"validation,omitempty"`
}

func buildSheinRepairPlan(actions []SheinRepairCenterAction) *SheinRepairPlan {
	if len(actions) == 0 {
		return nil
	}
	plan := &SheinRepairPlan{
		Steps: make([]SheinRepairPlanStep, 0, len(actions)),
	}
	for i, action := range actions {
		step := SheinRepairPlanStep{
			ID:               "step-" + repairCenterIntString(i+1),
			ActionID:         action.ID,
			Order:            i + 1,
			Label:            action.Label,
			Status:           action.Status,
			Priority:         action.Priority,
			EditorSection:    action.EditorSection,
			ExecutionMode:    buildSheinRepairExecutionMode(action),
			CanApplyDirectly: action.CanApplyDirectly,
			Reason:           buildSheinRepairStepReason(action),
			Highlights:       buildSheinRepairStepHighlights(action),
		}
		plan.Steps = append(plan.Steps, step)
		if action.CanApplyDirectly {
			plan.DirectApplySteps++
		} else {
			plan.ManualSteps++
		}
	}
	plan.TotalSteps = len(plan.Steps)
	if len(plan.Steps) > 0 {
		plan.PrimaryStepID = plan.Steps[0].ID
	}
	plan.Status = buildSheinRepairPlanStatus(plan)
	plan.Headline, plan.Subheadline = buildSheinRepairPlanCopy(plan)
	return plan
}

func buildSheinRepairApplyQueue(actions []SheinRepairCenterAction) *SheinRepairApplyQueue {
	if len(actions) == 0 {
		return nil
	}
	queue := &SheinRepairApplyQueue{
		DirectApplyOnly: true,
		Items:           make([]SheinRepairApplyQueueItem, 0, len(actions)),
	}
	for _, action := range actions {
		queue.TotalActions++
		if !action.CanApplyDirectly || action.Revision == nil {
			queue.DirectApplyOnly = false
			continue
		}
		queue.ReadyActions++
		queue.Items = append(queue.Items, SheinRepairApplyQueueItem{
			Order:         len(queue.Items) + 1,
			ActionID:      action.ID,
			Label:         action.Label,
			EditorSection: action.EditorSection,
			Revision:      cloneApplyRevisionRequest(action.Revision),
			Validation:    cloneSheinRepairValidationPreview(action.Validation),
		})
	}
	switch {
	case queue.ReadyActions == 0:
		queue.Status = "manual_only"
	case queue.ReadyActions < queue.TotalActions:
		queue.Status = "partial_ready"
	default:
		queue.Status = "ready"
	}
	return queue
}

func buildSheinRepairExecutionMode(action SheinRepairCenterAction) string {
	if action.CanApplyDirectly {
		return "direct_apply"
	}
	return "editor_required"
}

func buildSheinRepairStepReason(action SheinRepairCenterAction) string {
	if action.Validation != nil && !action.Validation.Valid {
		return "当前建议仍需先处理字段校验问题"
	}
	if action.CanApplyDirectly {
		return "当前动作已经具备最小可提交请求"
	}
	return "当前动作需要先在编辑器里确认字段"
}

func buildSheinRepairStepHighlights(action SheinRepairCenterAction) []string {
	highlights := make([]string, 0, 3)
	if action.Reason != nil && action.Reason.Summary != "" {
		highlights = append(highlights, action.Reason.Summary)
	}
	if action.Validation != nil && action.Validation.RevisionDiffPreview != nil && action.Validation.RevisionDiffPreview.ChangeCount > 0 {
		highlights = append(highlights, "预计变更 "+repairCenterIntString(action.Validation.RevisionDiffPreview.ChangeCount)+" 个字段")
	}
	if action.CanApplyDirectly {
		highlights = append(highlights, "可直接进入最小修复请求")
	}
	return uniqueStrings(highlights)
}

func buildSheinRepairPlanStatus(plan *SheinRepairPlan) string {
	if plan == nil || plan.TotalSteps == 0 {
		return "empty"
	}
	if plan.ManualSteps == 0 {
		return "ready"
	}
	if plan.DirectApplySteps == 0 {
		return "manual_first"
	}
	return "mixed"
}

func buildSheinRepairPlanCopy(plan *SheinRepairPlan) (string, string) {
	if plan == nil {
		return "", ""
	}
	switch plan.Status {
	case "ready":
		return "可以直接开始修复", "当前推荐动作都已经具备最小可提交请求"
	case "manual_first":
		return "建议先逐项人工确认", "当前修复流仍以编辑器确认步骤为主"
	default:
		return "建议按顺序推进修复", "先处理最高优先级动作，再进入可直接提交的修复项"
	}
}
