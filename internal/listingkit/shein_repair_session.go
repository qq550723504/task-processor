package listingkit

type SheinRepairSession struct {
	Status             string                         `json:"status,omitempty"`
	Headline           string                         `json:"headline,omitempty"`
	Subheadline        string                         `json:"subheadline,omitempty"`
	CurrentStepID      string                         `json:"current_step_id,omitempty"`
	NextStepID         string                         `json:"next_step_id,omitempty"`
	RefreshBlocks      []string                       `json:"refresh_blocks,omitempty"`
	ResumeState        *SheinRepairResumeState        `json:"resume_state,omitempty"`
	CompletionSnapshot *SheinRepairCompletionSnapshot `json:"completion_snapshot,omitempty"`
	SkippedSteps       []string                       `json:"skipped_steps,omitempty"`
	Runbook            []SheinRepairRunbookStep       `json:"runbook,omitempty"`
}

type SheinRepairResumeState struct {
	ResumeStepID   string   `json:"resume_step_id,omitempty"`
	ResumeActionID string   `json:"resume_action_id,omitempty"`
	ResumeMode     string   `json:"resume_mode,omitempty"`
	RemainingSteps int      `json:"remaining_steps,omitempty"`
	CompletedSteps int      `json:"completed_steps,omitempty"`
	SkippedSteps   int      `json:"skipped_steps,omitempty"`
	RefreshBlocks  []string `json:"refresh_blocks,omitempty"`
}

type SheinRepairCompletionSnapshot struct {
	TotalSteps       int `json:"total_steps,omitempty"`
	CompletedSteps   int `json:"completed_steps,omitempty"`
	SkippedSteps     int `json:"skipped_steps,omitempty"`
	DirectReadySteps int `json:"direct_ready_steps,omitempty"`
	ManualSteps      int `json:"manual_steps,omitempty"`
}

type SheinRepairRunbookStep struct {
	ID                string   `json:"id,omitempty"`
	ActionID          string   `json:"action_id,omitempty"`
	Order             int      `json:"order,omitempty"`
	Label             string   `json:"label,omitempty"`
	EditorSection     string   `json:"editor_section,omitempty"`
	ExecutionMode     string   `json:"execution_mode,omitempty"`
	CanSkipIfResolved bool     `json:"can_skip_if_resolved"`
	AutoAdvance       bool     `json:"auto_advance"`
	CompletionRule    string   `json:"completion_rule,omitempty"`
	NextStepID        string   `json:"next_step_id,omitempty"`
	RefreshBlocks     []string `json:"refresh_blocks,omitempty"`
}

func buildSheinRepairSession(actions []SheinRepairCenterAction, plan *SheinRepairPlan) *SheinRepairSession {
	if len(actions) == 0 || plan == nil || len(plan.Steps) == 0 {
		return nil
	}
	session := &SheinRepairSession{
		Runbook: make([]SheinRepairRunbookStep, 0, len(plan.Steps)),
	}
	for i, step := range plan.Steps {
		action := findSheinRepairCenterAction(actions, step.ActionID)
		runbookStep := SheinRepairRunbookStep{
			ID:                step.ID,
			ActionID:          step.ActionID,
			Order:             step.Order,
			Label:             step.Label,
			EditorSection:     step.EditorSection,
			ExecutionMode:     step.ExecutionMode,
			CanSkipIfResolved: true,
			AutoAdvance:       action != nil && action.CanApplyDirectly,
			CompletionRule:    buildSheinRepairCompletionRule(step, action),
			RefreshBlocks:     buildSheinRepairRefreshBlocks(action),
		}
		if i+1 < len(plan.Steps) {
			runbookStep.NextStepID = plan.Steps[i+1].ID
		}
		session.Runbook = append(session.Runbook, runbookStep)
	}
	session.CurrentStepID = session.Runbook[0].ID
	session.NextStepID = session.Runbook[0].NextStepID
	session.RefreshBlocks = append([]string(nil), session.Runbook[0].RefreshBlocks...)
	session.Status = buildSheinRepairSessionStatus(plan)
	session.Headline, session.Subheadline = buildSheinRepairSessionCopy(plan)
	session.SkippedSteps = buildSheinRepairSkippedSteps(session.Runbook)
	session.CompletionSnapshot = buildSheinRepairCompletionSnapshot(session.Runbook)
	session.ResumeState = buildSheinRepairResumeState(session, actions)
	return session
}

func findSheinRepairCenterAction(actions []SheinRepairCenterAction, actionID string) *SheinRepairCenterAction {
	for i := range actions {
		if actions[i].ID == actionID {
			return &actions[i]
		}
	}
	return nil
}

func buildSheinRepairCompletionRule(step SheinRepairPlanStep, action *SheinRepairCenterAction) string {
	if action != nil && action.CanApplyDirectly {
		return "提交最小修复请求并刷新对应区块"
	}
	switch step.EditorSection {
	case "category":
		return "确认类目解析后刷新 inspection 和 submit_readiness"
	case "attributes":
		return "确认属性映射后刷新 inspection 和 submit_checklist"
	case "sale_attributes":
		return "确认规格后刷新 preview_product 和提交校验"
	case "basics":
		return "更新基础资料后刷新 inspection 和 preview_product"
	default:
		return "完成当前修复后刷新相关预览区块"
	}
}

func buildSheinRepairRefreshBlocks(action *SheinRepairCenterAction) []string {
	if action == nil || action.Validation == nil {
		return []string{"inspection", "submit_readiness", "submit_checklist"}
	}
	blocks := append([]string(nil), action.Validation.AffectedSections...)
	return uniqueStrings(blocks)
}

func buildSheinRepairSessionStatus(plan *SheinRepairPlan) string {
	if plan == nil {
		return "empty"
	}
	switch plan.Status {
	case "ready":
		return "direct_repair"
	case "manual_first":
		return "guided_editor"
	default:
		return "guided_mixed"
	}
}

func buildSheinRepairSessionCopy(plan *SheinRepairPlan) (string, string) {
	if plan == nil {
		return "", ""
	}
	switch plan.Status {
	case "ready":
		return "可以直接开始修复会话", "优先执行可直接提交的修复动作，并在每步后刷新关键区块"
	case "manual_first":
		return "建议按编辑器引导完成修复", "当前会话以人工确认步骤为主，完成后再进入提交态"
	default:
		return "建议按引导顺序推进修复", "会话会先带你完成人工确认，再切换到可直接提交的动作"
	}
}

func buildSheinRepairSkippedSteps(runbook []SheinRepairRunbookStep) []string {
	if len(runbook) == 0 {
		return nil
	}
	skipped := make([]string, 0, len(runbook))
	for _, step := range runbook {
		if step.CanSkipIfResolved && step.ExecutionMode == "editor_required" {
			skipped = append(skipped, step.ID)
		}
	}
	return skipped
}

func buildSheinRepairCompletionSnapshot(runbook []SheinRepairRunbookStep) *SheinRepairCompletionSnapshot {
	if len(runbook) == 0 {
		return nil
	}
	snapshot := &SheinRepairCompletionSnapshot{
		TotalSteps: len(runbook),
	}
	for _, step := range runbook {
		if step.ExecutionMode == "direct_apply" {
			snapshot.DirectReadySteps++
		} else {
			snapshot.ManualSteps++
		}
		if step.CanSkipIfResolved && step.ExecutionMode == "editor_required" {
			snapshot.SkippedSteps++
		}
	}
	snapshot.CompletedSteps = snapshot.DirectReadySteps
	return snapshot
}

func buildSheinRepairResumeState(session *SheinRepairSession, actions []SheinRepairCenterAction) *SheinRepairResumeState {
	if session == nil || len(session.Runbook) == 0 {
		return nil
	}
	resume := &SheinRepairResumeState{}
	for _, step := range session.Runbook {
		action := findSheinRepairCenterAction(actions, step.ActionID)
		if action == nil {
			continue
		}
		if action.CanApplyDirectly {
			continue
		}
		resume.ResumeStepID = step.ID
		resume.ResumeActionID = step.ActionID
		resume.ResumeMode = step.ExecutionMode
		resume.RefreshBlocks = append([]string(nil), step.RefreshBlocks...)
		break
	}
	if resume.ResumeStepID == "" {
		last := session.Runbook[0]
		resume.ResumeStepID = last.ID
		resume.ResumeActionID = last.ActionID
		resume.ResumeMode = last.ExecutionMode
		resume.RefreshBlocks = append([]string(nil), last.RefreshBlocks...)
	}
	if session.CompletionSnapshot != nil {
		resume.CompletedSteps = session.CompletionSnapshot.CompletedSteps
		resume.SkippedSteps = session.CompletionSnapshot.SkippedSteps
		resume.RemainingSteps = session.CompletionSnapshot.TotalSteps - session.CompletionSnapshot.CompletedSteps
	}
	return resume
}
