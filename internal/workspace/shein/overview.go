package shein

import sheinpub "task-processor/internal/publishing/shein"

type ActionItem struct {
	Key             string
	SuggestedAction string
}

type SubmitStateInput struct {
	Status        string
	Ready         bool
	Summary       []string
	BlockingItems []ActionItem
	WarningItems  []ActionItem
}

type SessionInput struct {
	Status        string
	CurrentStepID string
	NextStepID    string
	ResumeMode    string
	RefreshBlocks []string
}

type RepairStateInput struct {
	Status             string
	TotalActions       int
	DirectApplyActions int
	PrimaryPlanStatus  string
	SessionStatus      string
	Summary            []string
	PrimaryAction      string
	PrimaryActionKey   string
	Session            *SessionInput
}

type StatusOverview struct {
	Status           string   `json:"status,omitempty"`
	Headline         string   `json:"headline,omitempty"`
	Subheadline      string   `json:"subheadline,omitempty"`
	NeedsReview      bool     `json:"needs_review"`
	BlockingCount    int      `json:"blocking_count,omitempty"`
	WarningCount     int      `json:"warning_count,omitempty"`
	Highlights       []string `json:"highlights,omitempty"`
	PrimaryAction    string   `json:"primary_action,omitempty"`
	PrimaryActionKey string   `json:"primary_action_key,omitempty"`
	NextActions      []string `json:"next_actions,omitempty"`
}

type WorkspaceOverview struct {
	Status           string                 `json:"status,omitempty"`
	Headline         string                 `json:"headline,omitempty"`
	Subheadline      string                 `json:"subheadline,omitempty"`
	PrimaryAction    string                 `json:"primary_action,omitempty"`
	PrimaryActionKey string                 `json:"primary_action_key,omitempty"`
	PrimaryView      string                 `json:"primary_view,omitempty"`
	NeedsReview      bool                   `json:"needs_review"`
	BlockingCount    int                    `json:"blocking_count,omitempty"`
	WarningCount     int                    `json:"warning_count,omitempty"`
	Highlights       []string               `json:"highlights,omitempty"`
	NextActions      []string               `json:"next_actions,omitempty"`
	ActiveSession    *WorkspaceSessionEntry `json:"active_session,omitempty"`
	SubmitState      *WorkspaceSubmitState  `json:"submit_state,omitempty"`
	RepairState      *WorkspaceRepairState  `json:"repair_state,omitempty"`
}

type WorkspaceSessionEntry struct {
	Status        string   `json:"status,omitempty"`
	CurrentStepID string   `json:"current_step_id,omitempty"`
	NextStepID    string   `json:"next_step_id,omitempty"`
	ResumeMode    string   `json:"resume_mode,omitempty"`
	RefreshBlocks []string `json:"refresh_blocks,omitempty"`
}

type WorkspaceSubmitState struct {
	Status        string   `json:"status,omitempty"`
	Ready         bool     `json:"ready"`
	BlockingCount int      `json:"blocking_count,omitempty"`
	WarningCount  int      `json:"warning_count,omitempty"`
	Summary       []string `json:"summary,omitempty"`
}

type WorkspaceRepairState struct {
	Status             string `json:"status,omitempty"`
	TotalActions       int    `json:"total_actions,omitempty"`
	DirectApplyActions int    `json:"direct_apply_actions,omitempty"`
	PrimaryPlanStatus  string `json:"primary_plan_status,omitempty"`
	SessionStatus      string `json:"session_status,omitempty"`
}

func BuildStatusOverview(inspection *sheinpub.Inspection, readiness *SubmitStateInput) *StatusOverview {
	overview := &StatusOverview{}
	if readiness != nil {
		overview.BlockingCount = len(readiness.BlockingItems)
		overview.WarningCount = len(readiness.WarningItems)
	}

	switch {
	case readiness != nil && readiness.Status == "blocked":
		overview.Status = "blocked"
		overview.Headline = "SHEIN 资料包暂不能直接提交"
		overview.Subheadline = firstNonEmpty(firstSummaryLine(readiness.Summary), "当前仍有关键字段未完成")
		applyPrimaryActionFromItems(overview, readiness.BlockingItems)
	case readiness != nil && readiness.Status == "ready_with_warnings":
		overview.Status = "ready_with_warnings"
		overview.Headline = "SHEIN 资料包已基本可提交"
		overview.Subheadline = firstNonEmpty(firstSummaryLine(readiness.Summary), "关键骨架已齐，建议先处理人工备注")
		applyPrimaryActionFromItems(overview, readiness.WarningItems)
	default:
		overview.Status = "ready"
		overview.Headline = "SHEIN 资料包已可进入提交流程"
		overview.Subheadline = firstNonEmpty(firstSummaryLine(readinessSummary(readiness)), "关键字段已满足提交前要求")
	}

	if inspection != nil {
		overview.NeedsReview = inspection.NeedsReview
		overview.Highlights = append(overview.Highlights, buildStatusHighlights(inspection)...)
		overview.NextActions = append(overview.NextActions, buildNextActions(inspection)...)
	}
	if readiness != nil {
		overview.NeedsReview = overview.NeedsReview || !readiness.Ready || len(readiness.WarningItems) > 0
		if len(overview.NextActions) == 0 {
			overview.NextActions = append(overview.NextActions, buildNextActionsFromReadiness(readiness)...)
		}
	}
	overview.Highlights = uniqueStrings(overview.Highlights)
	overview.NextActions = uniqueStrings(overview.NextActions)
	return overview
}

func BuildWorkspaceOverview(status *StatusOverview, readiness *SubmitStateInput, repair *RepairStateInput) *WorkspaceOverview {
	if status == nil && readiness == nil && repair == nil {
		return nil
	}

	overview := &WorkspaceOverview{}
	if status != nil {
		overview.Status = status.Status
		overview.Headline = status.Headline
		overview.Subheadline = status.Subheadline
		overview.PrimaryAction = status.PrimaryAction
		overview.PrimaryActionKey = status.PrimaryActionKey
		overview.NeedsReview = status.NeedsReview
		overview.BlockingCount = status.BlockingCount
		overview.WarningCount = status.WarningCount
		overview.Highlights = append([]string(nil), status.Highlights...)
		overview.NextActions = append([]string(nil), status.NextActions...)
		overview.PrimaryView = workspacePrimaryView(status, repair)
	}
	if readiness != nil {
		overview.SubmitState = &WorkspaceSubmitState{
			Status:        readiness.Status,
			Ready:         readiness.Ready,
			BlockingCount: len(readiness.BlockingItems),
			WarningCount:  len(readiness.WarningItems),
			Summary:       append([]string(nil), readiness.Summary...),
		}
		if overview.Status == "" {
			overview.Status = readiness.Status
		}
		if overview.Headline == "" {
			overview.Headline = workspaceHeadlineFromReadiness(readiness)
		}
		if overview.Subheadline == "" {
			overview.Subheadline = firstSummaryLine(readiness.Summary)
		}
		overview.NeedsReview = overview.NeedsReview || !readiness.Ready || len(readiness.WarningItems) > 0
	}
	if repair != nil {
		overview.RepairState = &WorkspaceRepairState{
			Status:             repair.Status,
			TotalActions:       repair.TotalActions,
			DirectApplyActions: repair.DirectApplyActions,
			PrimaryPlanStatus:  repair.PrimaryPlanStatus,
			SessionStatus:      repair.SessionStatus,
		}
		if repair.Session != nil {
			overview.ActiveSession = &WorkspaceSessionEntry{
				Status:        repair.Session.Status,
				CurrentStepID: repair.Session.CurrentStepID,
				NextStepID:    repair.Session.NextStepID,
				ResumeMode:    repair.Session.ResumeMode,
				RefreshBlocks: append([]string(nil), repair.Session.RefreshBlocks...),
			}
		}
		if overview.PrimaryAction == "" {
			overview.PrimaryAction = repair.PrimaryAction
			overview.PrimaryActionKey = repair.PrimaryActionKey
		}
		if overview.PrimaryView == "" {
			overview.PrimaryView = workspacePrimaryView(status, repair)
		}
		overview.Highlights = uniqueStrings(append(overview.Highlights, repair.Summary...))
	}
	overview.NextActions = uniqueStrings(overview.NextActions)
	overview.Highlights = uniqueStrings(overview.Highlights)
	return overview
}

func applyPrimaryActionFromItems(overview *StatusOverview, items []ActionItem) {
	if overview == nil || len(items) == 0 {
		return
	}
	overview.PrimaryAction = items[0].SuggestedAction
	overview.PrimaryActionKey = items[0].Key
}

func buildStatusHighlights(inspection *sheinpub.Inspection) []string {
	if inspection == nil {
		return nil
	}
	highlights := make([]string, 0, len(inspection.Sections))
	for _, section := range inspection.Sections {
		switch section.Status {
		case "resolved":
			highlights = append(highlights, section.Title+"已完成")
		case "partial":
			highlights = append(highlights, section.Title+"部分完成")
		case "missing", "unresolved":
			highlights = append(highlights, section.Title+"待处理")
		}
	}
	return highlights
}

func buildNextActions(inspection *sheinpub.Inspection) []string {
	if inspection == nil {
		return nil
	}
	actions := make([]string, 0, len(inspection.Sections))
	for _, section := range inspection.Sections {
		if len(section.Actions) > 0 && section.Actions[0].Label != "" {
			actions = append(actions, section.Actions[0].Label)
			continue
		}
		if len(section.ActionItems) > 0 {
			actions = append(actions, section.ActionItems[0])
		}
	}
	return actions
}

func buildNextActionsFromReadiness(readiness *SubmitStateInput) []string {
	if readiness == nil {
		return nil
	}
	actions := make([]string, 0, len(readiness.BlockingItems)+len(readiness.WarningItems))
	for _, item := range readiness.BlockingItems {
		if item.SuggestedAction != "" {
			actions = append(actions, item.SuggestedAction)
		}
	}
	for _, item := range readiness.WarningItems {
		if item.SuggestedAction != "" {
			actions = append(actions, item.SuggestedAction)
		}
	}
	return actions
}

func firstSummaryLine(summary []string) string {
	if len(summary) == 0 {
		return ""
	}
	return summary[0]
}

func readinessSummary(readiness *SubmitStateInput) []string {
	if readiness == nil {
		return nil
	}
	return readiness.Summary
}

func workspacePrimaryView(status *StatusOverview, repair *RepairStateInput) string {
	if repair != nil && repair.Session != nil {
		return "repair_center"
	}
	if status != nil {
		switch status.Status {
		case "blocked", "ready_with_warnings":
			return "inspection"
		default:
			return "submit"
		}
	}
	if repair != nil {
		return "repair_center"
	}
	return "inspection"
}

func workspaceHeadlineFromReadiness(readiness *SubmitStateInput) string {
	if readiness == nil {
		return ""
	}
	switch readiness.Status {
	case "blocked":
		return "SHEIN 工作台待修复"
	case "ready_with_warnings":
		return "SHEIN 工作台待确认"
	default:
		return "SHEIN 工作台已就绪"
	}
}

func uniqueStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
