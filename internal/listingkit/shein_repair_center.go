package listingkit

import (
	"sort"

	sheinworkspace "task-processor/internal/workspace/shein"
)

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

	seeds := make([]sheinworkspace.RepairCenterSeedAction[SheinReadinessReason, SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview], 0, len(candidates))
	for _, candidate := range candidates {
		seeds = append(seeds, sheinworkspace.RepairCenterSeedAction[SheinReadinessReason, SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview]{
			Action:       candidate.action,
			SectionKey:   candidate.sectionKey,
			SectionLabel: candidate.sectionLabel,
		})
	}

	return sheinworkspace.BuildRepairCenter(
		seeds,
		func(validation *SheinRepairValidationPreview) int {
			if validation == nil || validation.RevisionDiffPreview == nil {
				return 0
			}
			return validation.RevisionDiffPreview.ChangeCount
		},
		func(validation *SheinRepairValidationPreview) bool {
			return validation != nil && !validation.Valid
		},
		func(reason *SheinReadinessReason) string {
			if reason == nil {
				return ""
			}
			return reason.Summary
		},
		func(action SheinRepairCenterAction) sheinworkspace.RepairSessionActionInfo {
			info := sheinworkspace.RepairSessionActionInfo{
				ID:               action.ID,
				CanApplyDirectly: action.CanApplyDirectly,
			}
			if action.Validation != nil {
				info.ValidationValid = action.Validation.Valid
				info.AffectedSections = append([]string(nil), action.Validation.AffectedSections...)
			}
			return info
		},
	)
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
