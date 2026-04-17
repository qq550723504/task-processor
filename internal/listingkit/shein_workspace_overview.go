package listingkit

func buildSheinWorkspaceOverview(status *SheinStatusOverview, readiness *SheinSubmitReadiness, center *SheinRepairCenter) *SheinWorkspaceOverview {
	if status == nil && readiness == nil && center == nil {
		return nil
	}

	overview := &SheinWorkspaceOverview{}
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
		overview.PrimaryView = sheinWorkspacePrimaryView(status, center)
	}
	if readiness != nil {
		overview.SubmitState = &SheinWorkspaceSubmitState{
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
			overview.Headline = buildSheinWorkspaceHeadlineFromReadiness(readiness)
		}
		if overview.Subheadline == "" {
			overview.Subheadline = firstSummaryLine(readiness.Summary)
		}
		overview.NeedsReview = overview.NeedsReview || !readiness.Ready || len(readiness.WarningItems) > 0
	}
	if center != nil {
		overview.RepairState = &SheinWorkspaceRepairState{
			Status:             center.Status,
			TotalActions:       safeRepairActionCount(center),
			DirectApplyActions: safeRepairDirectApplyCount(center),
			PrimaryPlanStatus:  safeRepairPlanStatus(center),
			SessionStatus:      safeRepairSessionStatus(center),
		}
		if center.Session != nil {
			overview.ActiveSession = &SheinWorkspaceSessionEntry{
				Status:        center.Session.Status,
				CurrentStepID: center.Session.CurrentStepID,
				NextStepID:    center.Session.NextStepID,
				RefreshBlocks: append([]string(nil), center.Session.RefreshBlocks...),
			}
			if center.Session.ResumeState != nil {
				overview.ActiveSession.ResumeMode = center.Session.ResumeState.ResumeMode
				if overview.ActiveSession.CurrentStepID == "" {
					overview.ActiveSession.CurrentStepID = center.Session.ResumeState.ResumeStepID
				}
				if len(overview.ActiveSession.RefreshBlocks) == 0 {
					overview.ActiveSession.RefreshBlocks = append([]string(nil), center.Session.ResumeState.RefreshBlocks...)
				}
			}
		}
		if overview.PrimaryAction == "" && center.PrimaryAction != nil {
			overview.PrimaryAction = center.PrimaryAction.SuggestedAction
			overview.PrimaryActionKey = center.PrimaryAction.Key
		}
		if overview.PrimaryView == "" {
			overview.PrimaryView = sheinWorkspacePrimaryView(status, center)
		}
		overview.Highlights = uniqueStrings(append(overview.Highlights, center.Summary...))
	}
	overview.NextActions = uniqueStrings(overview.NextActions)
	overview.Highlights = uniqueStrings(overview.Highlights)
	return overview
}

func sheinWorkspacePrimaryView(status *SheinStatusOverview, center *SheinRepairCenter) string {
	if center != nil && center.Session != nil {
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
	if center != nil {
		return "repair_center"
	}
	return "inspection"
}

func buildSheinWorkspaceHeadlineFromReadiness(readiness *SheinSubmitReadiness) string {
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

func safeRepairActionCount(center *SheinRepairCenter) int {
	if center == nil || center.Stats == nil {
		return 0
	}
	return center.Stats.TotalActions
}

func safeRepairDirectApplyCount(center *SheinRepairCenter) int {
	if center == nil || center.Stats == nil {
		return 0
	}
	return center.Stats.DirectApplyActions
}

func safeRepairPlanStatus(center *SheinRepairCenter) string {
	if center == nil || center.PrimaryPlan == nil {
		return ""
	}
	return center.PrimaryPlan.Status
}

func safeRepairSessionStatus(center *SheinRepairCenter) string {
	if center == nil || center.Session == nil {
		return ""
	}
	return center.Session.Status
}
