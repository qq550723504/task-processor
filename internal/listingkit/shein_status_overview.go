package listingkit

type SheinStatusOverview struct {
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

func buildSheinStatusOverview(pkg *SheinPackage, readiness *SheinSubmitReadiness) *SheinStatusOverview {
	if pkg == nil {
		return nil
	}

	overview := &SheinStatusOverview{}
	if readiness != nil {
		overview.BlockingCount = len(readiness.BlockingItems)
		overview.WarningCount = len(readiness.WarningItems)
	}

	switch {
	case readiness != nil && readiness.Status == "blocked":
		overview.Status = "blocked"
		overview.Headline = "SHEIN 资料包暂不能直接提交"
		overview.Subheadline = firstNonEmpty(firstSummaryLine(readiness.Summary), "当前仍有关键字段未完成")
		applyPrimaryActionFromReadiness(overview, readiness.BlockingItems)
	case readiness != nil && readiness.Status == "ready_with_warnings":
		overview.Status = "ready_with_warnings"
		overview.Headline = "SHEIN 资料包已基本可提交"
		overview.Subheadline = firstNonEmpty(firstSummaryLine(readiness.Summary), "关键骨架已齐，建议先处理人工备注")
		applyPrimaryActionFromReadiness(overview, readiness.WarningItems)
	default:
		overview.Status = "ready"
		overview.Headline = "SHEIN 资料包已可进入提交流程"
		overview.Subheadline = firstNonEmpty(firstSummaryLine(readinessSummary(readiness)), "关键字段已满足提交前要求")
	}

	if pkg.Inspection != nil {
		overview.NeedsReview = pkg.Inspection.NeedsReview
		overview.Highlights = append(overview.Highlights, buildSheinStatusHighlights(pkg.Inspection)...)
		overview.NextActions = append(overview.NextActions, buildSheinNextActions(pkg.Inspection)...)
	}
	if readiness != nil {
		overview.NeedsReview = overview.NeedsReview || !readiness.Ready || len(readiness.WarningItems) > 0
		if len(overview.NextActions) == 0 {
			overview.NextActions = append(overview.NextActions, buildSheinNextActionsFromReadiness(readiness)...)
		}
	}
	overview.Highlights = uniqueStrings(overview.Highlights)
	overview.NextActions = uniqueStrings(overview.NextActions)
	return overview
}

func applyPrimaryActionFromReadiness(overview *SheinStatusOverview, items []SheinReadinessItem) {
	if overview == nil || len(items) == 0 {
		return
	}
	overview.PrimaryAction = items[0].SuggestedAction
	overview.PrimaryActionKey = items[0].Key
}

func buildSheinStatusHighlights(inspection *SheinInspection) []string {
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

func buildSheinNextActions(inspection *SheinInspection) []string {
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

func buildSheinNextActionsFromReadiness(readiness *SheinSubmitReadiness) []string {
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

func readinessSummary(readiness *SheinSubmitReadiness) []string {
	if readiness == nil {
		return nil
	}
	return readiness.Summary
}
