package shein

type RepairCenter[R any, P any, S any, Q any, V any] struct {
	Status        string                              `json:"status,omitempty"`
	Summary       []string                            `json:"summary,omitempty"`
	Stats         *RepairCenterStats                  `json:"stats,omitempty"`
	PrimaryAction *RepairCenterAction[R, P, S, Q, V]  `json:"primary_action,omitempty"`
	PrimaryPlan   *RepairPlan                         `json:"primary_plan,omitempty"`
	ApplyQueue    *RepairApplyQueue[Q, V]             `json:"apply_queue,omitempty"`
	Session       *RepairSession                      `json:"session,omitempty"`
	Sections      []RepairCenterSection               `json:"sections,omitempty"`
	Actions       []RepairCenterAction[R, P, S, Q, V] `json:"actions,omitempty"`
}

type RepairCenterSeedAction[R any, P any, S any, Q any, V any] struct {
	Action       RepairCenterAction[R, P, S, Q, V]
	SectionKey   string
	SectionLabel string
}

type RepairCenterStats struct {
	TotalActions       int `json:"total_actions,omitempty"`
	BlockingActions    int `json:"blocking_actions,omitempty"`
	WarningActions     int `json:"warning_actions,omitempty"`
	DirectApplyActions int `json:"direct_apply_actions,omitempty"`
}

type RepairCenterSection struct {
	Key              string   `json:"key,omitempty"`
	Label            string   `json:"label,omitempty"`
	ActionCount      int      `json:"action_count,omitempty"`
	DirectApplyCount int      `json:"direct_apply_count,omitempty"`
	Highlights       []string `json:"highlights,omitempty"`
}

type RepairCenterAction[R any, P any, S any, Q any, V any] struct {
	ID               string   `json:"id,omitempty"`
	Key              string   `json:"key,omitempty"`
	Label            string   `json:"label,omitempty"`
	Status           string   `json:"status,omitempty"`
	Priority         string   `json:"priority,omitempty"`
	EditorSection    string   `json:"editor_section,omitempty"`
	Target           string   `json:"target,omitempty"`
	Description      string   `json:"description,omitempty"`
	SuggestedAction  string   `json:"suggested_action,omitempty"`
	CanApplyDirectly bool     `json:"can_apply_directly"`
	SourceGroups     []string `json:"source_groups,omitempty"`
	FieldPaths       []string `json:"field_paths,omitempty"`
	EditorFocus      []string `json:"editor_focus,omitempty"`
	RevisionPath     string   `json:"revision_path,omitempty"`
	Reason           *R       `json:"reason,omitempty"`
	Patch            *P       `json:"patch,omitempty"`
	Skeleton         *S       `json:"skeleton,omitempty"`
	Revision         *Q       `json:"revision,omitempty"`
	Validation       *V       `json:"validation,omitempty"`
}

type RepairPlan struct {
	Status           string           `json:"status,omitempty"`
	Headline         string           `json:"headline,omitempty"`
	Subheadline      string           `json:"subheadline,omitempty"`
	PrimaryStepID    string           `json:"primary_step_id,omitempty"`
	TotalSteps       int              `json:"total_steps,omitempty"`
	DirectApplySteps int              `json:"direct_apply_steps,omitempty"`
	ManualSteps      int              `json:"manual_steps,omitempty"`
	Steps            []RepairPlanStep `json:"steps,omitempty"`
}

type RepairPlanStep struct {
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

type RepairApplyQueue[Q any, V any] struct {
	Status          string                       `json:"status,omitempty"`
	DirectApplyOnly bool                         `json:"direct_apply_only"`
	TotalActions    int                          `json:"total_actions,omitempty"`
	ReadyActions    int                          `json:"ready_actions,omitempty"`
	Items           []RepairApplyQueueItem[Q, V] `json:"items,omitempty"`
}

type RepairApplyQueueItem[Q any, V any] struct {
	Order         int    `json:"order,omitempty"`
	ActionID      string `json:"action_id,omitempty"`
	Label         string `json:"label,omitempty"`
	EditorSection string `json:"editor_section,omitempty"`
	Revision      *Q     `json:"revision,omitempty"`
	Validation    *V     `json:"validation,omitempty"`
}

type RepairSession struct {
	Status             string                    `json:"status,omitempty"`
	Headline           string                    `json:"headline,omitempty"`
	Subheadline        string                    `json:"subheadline,omitempty"`
	CurrentStepID      string                    `json:"current_step_id,omitempty"`
	NextStepID         string                    `json:"next_step_id,omitempty"`
	RefreshBlocks      []string                  `json:"refresh_blocks,omitempty"`
	ResumeState        *RepairResumeState        `json:"resume_state,omitempty"`
	CompletionSnapshot *RepairCompletionSnapshot `json:"completion_snapshot,omitempty"`
	SkippedSteps       []string                  `json:"skipped_steps,omitempty"`
	Runbook            []RepairRunbookStep       `json:"runbook,omitempty"`
}

type RepairResumeState struct {
	ResumeStepID   string   `json:"resume_step_id,omitempty"`
	ResumeActionID string   `json:"resume_action_id,omitempty"`
	ResumeMode     string   `json:"resume_mode,omitempty"`
	RemainingSteps int      `json:"remaining_steps,omitempty"`
	CompletedSteps int      `json:"completed_steps,omitempty"`
	SkippedSteps   int      `json:"skipped_steps,omitempty"`
	RefreshBlocks  []string `json:"refresh_blocks,omitempty"`
}

type RepairCompletionSnapshot struct {
	TotalSteps       int `json:"total_steps,omitempty"`
	CompletedSteps   int `json:"completed_steps,omitempty"`
	SkippedSteps     int `json:"skipped_steps,omitempty"`
	DirectReadySteps int `json:"direct_ready_steps,omitempty"`
	ManualSteps      int `json:"manual_steps,omitempty"`
}

type RepairRunbookStep struct {
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

type RepairSessionActionInfo struct {
	ID               string
	CanApplyDirectly bool
	ValidationValid  bool
	AffectedSections []string
}

func RepairCenterActionCount[R any, P any, S any, Q any, V any](center *RepairCenter[R, P, S, Q, V]) int {
	if center == nil || center.Stats == nil {
		return 0
	}
	return center.Stats.TotalActions
}

func RepairCenterDirectApplyCount[R any, P any, S any, Q any, V any](center *RepairCenter[R, P, S, Q, V]) int {
	if center == nil || center.Stats == nil {
		return 0
	}
	return center.Stats.DirectApplyActions
}

func RepairCenterPrimaryPlanStatus[R any, P any, S any, Q any, V any](center *RepairCenter[R, P, S, Q, V]) string {
	if center == nil || center.PrimaryPlan == nil {
		return ""
	}
	return center.PrimaryPlan.Status
}

func RepairCenterSessionStatus[R any, P any, S any, Q any, V any](center *RepairCenter[R, P, S, Q, V]) string {
	if center == nil || center.Session == nil {
		return ""
	}
	return center.Session.Status
}

func BuildRepairPlan[R any, P any, S any, Q any, V any](
	actions []RepairCenterAction[R, P, S, Q, V],
	changeCount func(*V) int,
	isInvalid func(*V) bool,
	reasonSummary func(*R) string,
) *RepairPlan {
	if len(actions) == 0 {
		return nil
	}
	plan := &RepairPlan{
		Steps: make([]RepairPlanStep, 0, len(actions)),
	}
	for i, action := range actions {
		step := RepairPlanStep{
			ID:               "step-" + intString(i+1),
			ActionID:         action.ID,
			Order:            i + 1,
			Label:            action.Label,
			Status:           action.Status,
			Priority:         action.Priority,
			EditorSection:    action.EditorSection,
			ExecutionMode:    repairExecutionMode(action.CanApplyDirectly),
			CanApplyDirectly: action.CanApplyDirectly,
			Reason:           buildRepairStepReason(action.CanApplyDirectly, action.Validation != nil && isInvalid != nil && isInvalid(action.Validation)),
			Highlights:       buildRepairStepHighlights(action, changeCount, reasonSummary),
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
	plan.Status = buildRepairPlanStatus(plan)
	plan.Headline, plan.Subheadline = buildRepairPlanCopy(plan)
	return plan
}

func BuildRepairApplyQueue[R any, P any, S any, Q any, V any](
	actions []RepairCenterAction[R, P, S, Q, V],
) *RepairApplyQueue[Q, V] {
	if len(actions) == 0 {
		return nil
	}
	queue := &RepairApplyQueue[Q, V]{
		DirectApplyOnly: true,
		Items:           make([]RepairApplyQueueItem[Q, V], 0, len(actions)),
	}
	for _, action := range actions {
		queue.TotalActions++
		if !action.CanApplyDirectly || action.Revision == nil {
			queue.DirectApplyOnly = false
			continue
		}
		queue.ReadyActions++
		queue.Items = append(queue.Items, RepairApplyQueueItem[Q, V]{
			Order:         len(queue.Items) + 1,
			ActionID:      action.ID,
			Label:         action.Label,
			EditorSection: action.EditorSection,
			Revision:      action.Revision,
			Validation:    action.Validation,
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

func BuildRepairSession(
	plan *RepairPlan,
	actionInfo []RepairSessionActionInfo,
) *RepairSession {
	if plan == nil || len(plan.Steps) == 0 {
		return nil
	}
	lookup := map[string]RepairSessionActionInfo{}
	for _, info := range actionInfo {
		lookup[info.ID] = info
	}
	session := &RepairSession{
		Runbook: make([]RepairRunbookStep, 0, len(plan.Steps)),
	}
	for i, step := range plan.Steps {
		info := lookup[step.ActionID]
		runbookStep := RepairRunbookStep{
			ID:                step.ID,
			ActionID:          step.ActionID,
			Order:             step.Order,
			Label:             step.Label,
			EditorSection:     step.EditorSection,
			ExecutionMode:     step.ExecutionMode,
			CanSkipIfResolved: true,
			AutoAdvance:       info.CanApplyDirectly,
			CompletionRule:    buildRepairCompletionRule(step.EditorSection, info.CanApplyDirectly),
			RefreshBlocks:     uniqueStrings(append([]string(nil), info.AffectedSections...)),
		}
		if len(runbookStep.RefreshBlocks) == 0 {
			runbookStep.RefreshBlocks = []string{"inspection", "submit_readiness", "submit_checklist"}
		}
		if i+1 < len(plan.Steps) {
			runbookStep.NextStepID = plan.Steps[i+1].ID
		}
		session.Runbook = append(session.Runbook, runbookStep)
	}
	session.CurrentStepID = session.Runbook[0].ID
	session.NextStepID = session.Runbook[0].NextStepID
	session.RefreshBlocks = append([]string(nil), session.Runbook[0].RefreshBlocks...)
	session.Status = buildRepairSessionStatus(plan)
	session.Headline, session.Subheadline = buildRepairSessionCopy(plan)
	session.SkippedSteps = buildRepairSkippedSteps(session.Runbook)
	session.CompletionSnapshot = buildRepairCompletionSnapshot(session.Runbook)
	session.ResumeState = buildRepairResumeState(session, lookup)
	return session
}

func BuildRepairCenterStatus(stats *RepairCenterStats) string {
	if stats == nil || stats.TotalActions == 0 {
		return "empty"
	}
	if stats.BlockingActions > 0 {
		return "needs_repair"
	}
	if stats.WarningActions > 0 {
		return "review_recommended"
	}
	return "ready"
}

func BuildRepairCenterSummary[R any, P any, S any, Q any, V any](center *RepairCenter[R, P, S, Q, V]) []string {
	if center == nil || center.Stats == nil {
		return nil
	}
	summary := make([]string, 0, 3)
	if center.Stats.TotalActions > 0 {
		summary = append(summary, "已整理 "+intString(center.Stats.TotalActions)+" 个修复动作")
	}
	if center.Stats.BlockingActions > 0 {
		summary = append(summary, "其中 "+intString(center.Stats.BlockingActions)+" 个会直接影响提交")
	}
	if center.Stats.DirectApplyActions > 0 {
		summary = append(summary, "有 "+intString(center.Stats.DirectApplyActions)+" 个动作可直接生成最小修复请求")
	}
	return summary
}

func BuildRepairCenter[R any, P any, S any, Q any, V any](
	seeds []RepairCenterSeedAction[R, P, S, Q, V],
	changeCount func(*V) int,
	isInvalid func(*V) bool,
	reasonSummary func(*R) string,
	actionInfo func(RepairCenterAction[R, P, S, Q, V]) RepairSessionActionInfo,
) *RepairCenter[R, P, S, Q, V] {
	if len(seeds) == 0 {
		return nil
	}

	center := &RepairCenter[R, P, S, Q, V]{
		Actions: make([]RepairCenterAction[R, P, S, Q, V], 0, len(seeds)),
		Stats:   &RepairCenterStats{},
	}
	sectionMap := map[string]*RepairCenterSection{}
	sectionOrder := make([]string, 0, len(seeds))
	sessionInfo := make([]RepairSessionActionInfo, 0, len(seeds))

	for _, seed := range seeds {
		action := seed.Action
		center.Actions = append(center.Actions, action)
		center.Stats.TotalActions++
		if action.Status == "blocking" {
			center.Stats.BlockingActions++
		}
		if action.Status == "warning" {
			center.Stats.WarningActions++
		}
		if action.CanApplyDirectly {
			center.Stats.DirectApplyActions++
		}
		if center.PrimaryAction == nil {
			primary := action
			center.PrimaryAction = &primary
		}

		section, ok := sectionMap[seed.SectionKey]
		if !ok {
			section = &RepairCenterSection{
				Key:   seed.SectionKey,
				Label: seed.SectionLabel,
			}
			sectionMap[seed.SectionKey] = section
			sectionOrder = append(sectionOrder, seed.SectionKey)
		}
		section.ActionCount++
		if action.CanApplyDirectly {
			section.DirectApplyCount++
		}
		section.Highlights = uniqueStrings(append(section.Highlights, action.Label))

		if actionInfo != nil {
			sessionInfo = append(sessionInfo, actionInfo(action))
		}
	}

	for _, key := range sectionOrder {
		center.Sections = append(center.Sections, *sectionMap[key])
	}

	center.PrimaryPlan = BuildRepairPlan(center.Actions, changeCount, isInvalid, reasonSummary)
	center.ApplyQueue = BuildRepairApplyQueue(center.Actions)
	center.Session = BuildRepairSession(center.PrimaryPlan, sessionInfo)
	center.Status = BuildRepairCenterStatus(center.Stats)
	center.Summary = BuildRepairCenterSummary(center)
	return center
}

func repairExecutionMode(canApplyDirectly bool) string {
	if canApplyDirectly {
		return "direct_apply"
	}
	return "editor_required"
}

func buildRepairStepReason(canApplyDirectly bool, invalid bool) string {
	if invalid {
		return "当前建议仍需先处理字段校验问题"
	}
	if canApplyDirectly {
		return "当前动作已经具备最小可提交请求"
	}
	return "当前动作需要先在编辑器里确认字段"
}

func buildRepairStepHighlights[R any, P any, S any, Q any, V any](
	action RepairCenterAction[R, P, S, Q, V],
	changeCount func(*V) int,
	reasonSummary func(*R) string,
) []string {
	highlights := make([]string, 0, 3)
	if action.Reason != nil {
		if summary := reasonSummary(action.Reason); summary != "" {
			highlights = append(highlights, summary)
		}
	}
	if action.Validation != nil && changeCount != nil {
		if count := changeCount(action.Validation); count > 0 {
			highlights = append(highlights, "预计变更 "+intString(count)+" 个字段")
		}
	}
	if action.CanApplyDirectly {
		highlights = append(highlights, "可直接进入最小修复请求")
	}
	return uniqueStrings(highlights)
}

func buildRepairPlanStatus(plan *RepairPlan) string {
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

func buildRepairPlanCopy(plan *RepairPlan) (string, string) {
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

func buildRepairCompletionRule(section string, canApplyDirectly bool) string {
	if canApplyDirectly {
		return "提交最小修复请求并刷新对应区块"
	}
	switch section {
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

func buildRepairSessionStatus(plan *RepairPlan) string {
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

func buildRepairSessionCopy(plan *RepairPlan) (string, string) {
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

func buildRepairSkippedSteps(runbook []RepairRunbookStep) []string {
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

func buildRepairCompletionSnapshot(runbook []RepairRunbookStep) *RepairCompletionSnapshot {
	if len(runbook) == 0 {
		return nil
	}
	snapshot := &RepairCompletionSnapshot{TotalSteps: len(runbook)}
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

func buildRepairResumeState(session *RepairSession, lookup map[string]RepairSessionActionInfo) *RepairResumeState {
	if session == nil || len(session.Runbook) == 0 {
		return nil
	}
	resume := &RepairResumeState{}
	for _, step := range session.Runbook {
		info := lookup[step.ActionID]
		if info.ID == "" {
			continue
		}
		if info.CanApplyDirectly {
			continue
		}
		resume.ResumeStepID = step.ID
		resume.ResumeActionID = step.ActionID
		resume.ResumeMode = step.ExecutionMode
		resume.RefreshBlocks = append([]string(nil), step.RefreshBlocks...)
		break
	}
	if resume.ResumeStepID == "" {
		first := session.Runbook[0]
		resume.ResumeStepID = first.ID
		resume.ResumeActionID = first.ActionID
		resume.ResumeMode = first.ExecutionMode
		resume.RefreshBlocks = append([]string(nil), first.RefreshBlocks...)
	}
	if session.CompletionSnapshot != nil {
		resume.CompletedSteps = session.CompletionSnapshot.CompletedSteps
		resume.SkippedSteps = session.CompletionSnapshot.SkippedSteps
		resume.RemainingSteps = session.CompletionSnapshot.TotalSteps - session.CompletionSnapshot.CompletedSteps
	}
	return resume
}

func intString(v int) string {
	if v == 0 {
		return "0"
	}
	return formatInt(v)
}

func formatInt(v int) string {
	if v == 0 {
		return "0"
	}
	digits := []byte{}
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}
