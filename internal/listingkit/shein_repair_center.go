package listingkit

import "sort"

type sheinRepairActionCandidate struct {
	action       SheinRepairCenterAction
	sectionKey   string
	sectionLabel string
}

func buildSheinRepairCenter(readiness *SheinSubmitReadiness, checklist *SheinSubmitChecklist) *SheinRepairCenter {
	if readiness == nil {
		return nil
	}

	candidates := collectSheinRepairCandidates(readiness, checklist)
	if len(candidates) == 0 {
		return nil
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return compareSheinRepairActions(candidates[i].action, candidates[j].action)
	})

	center := &SheinRepairCenter{
		Actions: make([]SheinRepairCenterAction, 0, len(candidates)),
		Stats:   &SheinRepairCenterStats{},
	}
	sectionMap := map[string]*SheinRepairCenterSection{}
	sectionOrder := make([]string, 0, 4)

	for _, candidate := range candidates {
		action := candidate.action
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

		section, ok := sectionMap[candidate.sectionKey]
		if !ok {
			section = &SheinRepairCenterSection{
				Key:   candidate.sectionKey,
				Label: candidate.sectionLabel,
			}
			sectionMap[candidate.sectionKey] = section
			sectionOrder = append(sectionOrder, candidate.sectionKey)
		}
		section.ActionCount++
		if action.CanApplyDirectly {
			section.DirectApplyCount++
		}
		section.Highlights = uniqueStrings(append(section.Highlights, action.Label))
	}

	for _, key := range sectionOrder {
		center.Sections = append(center.Sections, *sectionMap[key])
	}

	center.PrimaryPlan = buildSheinRepairPlan(center.Actions)
	center.ApplyQueue = buildSheinRepairApplyQueue(center.Actions)
	center.Session = buildSheinRepairSession(center.Actions, center.PrimaryPlan)
	center.Status = buildSheinRepairCenterStatus(center.Stats)
	center.Summary = buildSheinRepairCenterSummary(center)
	return center
}

func collectSheinRepairCandidates(readiness *SheinSubmitReadiness, checklist *SheinSubmitChecklist) []sheinRepairActionCandidate {
	indexed := map[string]*sheinRepairActionCandidate{}

	addFromItem := func(item SheinReadinessItem, status, sourceGroup string) {
		for _, hint := range item.RepairHints {
			id := sheinRepairActionID(item.Key, hint)
			candidate, ok := indexed[id]
			if !ok {
				action := SheinRepairCenterAction{
					ID:               id,
					Key:              item.Key,
					Label:            item.Label,
					Status:           status,
					Priority:         hint.Priority,
					EditorSection:    hint.EditorSection,
					Target:           hint.Target,
					Description:      hint.Description,
					SuggestedAction:  item.SuggestedAction,
					CanApplyDirectly: hint.Validation != nil && hint.Validation.Valid && hint.Revision != nil,
					FieldPaths:       append([]string(nil), item.FieldPaths...),
					EditorFocus:      append([]string(nil), hint.EditorFocus...),
					RevisionPath:     hint.RevisionPath,
					Reason:           cloneSheinReadinessReason(item.Reason),
					Patch:            cloneSheinRepairPatchPayload(hint.Patch),
					Skeleton:         cloneSheinEditorRevisionSkeleton(hint.Skeleton),
					Revision:         cloneApplyRevisionRequest(hint.Revision),
					Validation:       cloneSheinRepairValidationPreview(hint.Validation),
				}
				candidate = &sheinRepairActionCandidate{
					action:       action,
					sectionKey:   normalizeSheinRepairSection(hint.EditorSection),
					sectionLabel: sheinRepairSectionLabel(hint.EditorSection),
				}
				indexed[id] = candidate
			}
			candidate.action.SourceGroups = uniqueStrings(append(candidate.action.SourceGroups, sourceGroup))
		}
	}

	if readiness != nil {
		for _, item := range readiness.BlockingItems {
			addFromItem(item, "blocking", "blocking")
		}
		for _, item := range readiness.WarningItems {
			addFromItem(item, "warning", "warning")
		}
	}
	if checklist != nil {
		for _, item := range checklist.Required {
			addFromItem(SheinReadinessItem{
				Key:             item.Key,
				Label:           item.Label,
				Message:         item.Message,
				FieldPaths:      item.FieldPaths,
				SuggestedAction: item.SuggestedAction,
				Reason:          item.Reason,
				RepairHints:     item.RepairHints,
			}, item.Status, "required")
		}
		for _, item := range checklist.Recommended {
			addFromItem(SheinReadinessItem{
				Key:             item.Key,
				Label:           item.Label,
				Message:         item.Message,
				FieldPaths:      item.FieldPaths,
				SuggestedAction: item.SuggestedAction,
				Reason:          item.Reason,
				RepairHints:     item.RepairHints,
			}, item.Status, "recommended")
		}
		for _, item := range checklist.Optional {
			addFromItem(SheinReadinessItem{
				Key:             item.Key,
				Label:           item.Label,
				Message:         item.Message,
				FieldPaths:      item.FieldPaths,
				SuggestedAction: item.SuggestedAction,
				Reason:          item.Reason,
				RepairHints:     item.RepairHints,
			}, item.Status, "optional")
		}
	}

	result := make([]sheinRepairActionCandidate, 0, len(indexed))
	for _, candidate := range indexed {
		result = append(result, *candidate)
	}
	return result
}

func compareSheinRepairActions(a, b SheinRepairCenterAction) bool {
	if sheinRepairPriorityRank(a.Priority) != sheinRepairPriorityRank(b.Priority) {
		return sheinRepairPriorityRank(a.Priority) < sheinRepairPriorityRank(b.Priority)
	}
	if sheinRepairStatusRank(a.Status) != sheinRepairStatusRank(b.Status) {
		return sheinRepairStatusRank(a.Status) < sheinRepairStatusRank(b.Status)
	}
	if a.CanApplyDirectly != b.CanApplyDirectly {
		return a.CanApplyDirectly
	}
	if sheinRepairSectionRank(a.EditorSection) != sheinRepairSectionRank(b.EditorSection) {
		return sheinRepairSectionRank(a.EditorSection) < sheinRepairSectionRank(b.EditorSection)
	}
	return a.ID < b.ID
}

func sheinRepairActionID(key string, hint SheinRepairHint) string {
	return key + ":" + hint.Target + ":" + hint.RevisionPath
}

func sheinRepairPriorityRank(priority string) int {
	switch priority {
	case "high":
		return 0
	case "medium":
		return 1
	default:
		return 2
	}
}

func sheinRepairStatusRank(status string) int {
	switch status {
	case "blocking":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

func sheinRepairSectionRank(section string) int {
	switch normalizeSheinRepairSection(section) {
	case "category":
		return 0
	case "attributes":
		return 1
	case "sale_attributes":
		return 2
	case "basics":
		return 3
	default:
		return 4
	}
}

func normalizeSheinRepairSection(section string) string {
	switch section {
	case "category", "attributes", "sale_attributes", "basics":
		return section
	default:
		return "system"
	}
}

func sheinRepairSectionLabel(section string) string {
	switch normalizeSheinRepairSection(section) {
	case "category":
		return "类目修复"
	case "attributes":
		return "属性修复"
	case "sale_attributes":
		return "规格修复"
	case "basics":
		return "基础资料修复"
	default:
		return "系统动作"
	}
}

func buildSheinRepairCenterStatus(stats *SheinRepairCenterStats) string {
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

func buildSheinRepairCenterSummary(center *SheinRepairCenter) []string {
	if center == nil || center.Stats == nil {
		return nil
	}
	summary := make([]string, 0, 3)
	if center.Stats.TotalActions > 0 {
		summary = append(summary, "已整理 "+repairCenterIntString(center.Stats.TotalActions)+" 个修复动作")
	}
	if center.Stats.BlockingActions > 0 {
		summary = append(summary, "其中 "+repairCenterIntString(center.Stats.BlockingActions)+" 个会直接影响提交")
	}
	if center.Stats.DirectApplyActions > 0 {
		summary = append(summary, "有 "+repairCenterIntString(center.Stats.DirectApplyActions)+" 个动作可直接生成最小修复请求")
	}
	return summary
}

func repairCenterIntString(v int) string {
	if v == 0 {
		return "0"
	}
	return formatInt(v)
}
