package workspace

import "sort"

type RepairHintAccessors[H any, P any, S any, Q any, V any] struct {
	Priority      func(H) string
	Target        func(H) string
	EditorSection func(H) string
	EditorFocus   func(H) []string
	RevisionPath  func(H) string
	Description   func(H) string
	Patch         func(H) *P
	Skeleton      func(H) *S
	Revision      func(H) *Q
	Validation    func(H) *V
}

type RepairCenterFromReadinessOptions[R any, P any, S any, Q any, V any] struct {
	CloneReason     func(*R) *R
	CloneArtifacts  func(*P, *S, *Q, *V) (*P, *S, *Q, *V)
	ValidationValid func(*V) bool
	ChangeCount     func(*V) int
	IsInvalid       func(*V) bool
	ReasonSummary   func(*R) string
	ActionInfo      func(RepairCenterAction[R, P, S, Q, V]) RepairSessionActionInfo
}

type repairActionCandidate[R any, P any, S any, Q any, V any] struct {
	action       RepairCenterAction[R, P, S, Q, V]
	sectionKey   string
	sectionLabel string
}

func BuildRepairCenterFromReadiness[R any, H any, P any, S any, Q any, V any](
	readiness *SubmitReadiness[R, H],
	checklist *SubmitChecklist[R, H],
	accessors RepairHintAccessors[H, P, S, Q, V],
	options RepairCenterFromReadinessOptions[R, P, S, Q, V],
) *RepairCenter[R, P, S, Q, V] {
	if readiness == nil {
		return nil
	}

	candidates := collectRepairCandidates(readiness, checklist, accessors, options)
	if len(candidates) == 0 {
		return nil
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return compareRepairActions(candidates[i].action, candidates[j].action)
	})

	seeds := make([]RepairCenterSeedAction[R, P, S, Q, V], 0, len(candidates))
	for _, candidate := range candidates {
		seeds = append(seeds, RepairCenterSeedAction[R, P, S, Q, V]{
			Action:       candidate.action,
			SectionKey:   candidate.sectionKey,
			SectionLabel: candidate.sectionLabel,
		})
	}

	return BuildRepairCenter(seeds, options.ChangeCount, options.IsInvalid, options.ReasonSummary, options.ActionInfo)
}

func collectRepairCandidates[R any, H any, P any, S any, Q any, V any](
	readiness *SubmitReadiness[R, H],
	checklist *SubmitChecklist[R, H],
	accessors RepairHintAccessors[H, P, S, Q, V],
	options RepairCenterFromReadinessOptions[R, P, S, Q, V],
) []repairActionCandidate[R, P, S, Q, V] {
	indexed := map[string]*repairActionCandidate[R, P, S, Q, V]{}

	addFromItem := func(item ReadinessItem[R, H], status, sourceGroup string) {
		for _, hint := range item.RepairHints {
			id := repairActionID(item.Key, accessors.Target(hint), accessors.RevisionPath(hint))
			candidate, ok := indexed[id]
			if !ok {
				patch, skeleton, revision, validation := cloneRepairArtifacts(hint, accessors, options)
				action := RepairCenterAction[R, P, S, Q, V]{
					ID:               id,
					Key:              item.Key,
					Label:            item.Label,
					Status:           status,
					Priority:         accessors.Priority(hint),
					EditorSection:    accessors.EditorSection(hint),
					Target:           accessors.Target(hint),
					Description:      accessors.Description(hint),
					SuggestedAction:  item.SuggestedAction,
					CanApplyDirectly: validation != nil && options.ValidationValid != nil && options.ValidationValid(validation) && revision != nil,
					FieldPaths:       append([]string(nil), item.FieldPaths...),
					EditorFocus:      append([]string(nil), accessors.EditorFocus(hint)...),
					RevisionPath:     accessors.RevisionPath(hint),
					Reason:           cloneRepairReason(item.Reason, options),
					Patch:            patch,
					Skeleton:         skeleton,
					Revision:         revision,
					Validation:       validation,
				}
				candidate = &repairActionCandidate[R, P, S, Q, V]{
					action:       action,
					sectionKey:   normalizeRepairSection(accessors.EditorSection(hint)),
					sectionLabel: repairSectionLabel(accessors.EditorSection(hint)),
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
			addFromItem(checklistItemAsReadinessItem(item), item.Status, "required")
		}
		for _, item := range checklist.Recommended {
			addFromItem(checklistItemAsReadinessItem(item), item.Status, "recommended")
		}
		for _, item := range checklist.Optional {
			addFromItem(checklistItemAsReadinessItem(item), item.Status, "optional")
		}
	}

	result := make([]repairActionCandidate[R, P, S, Q, V], 0, len(indexed))
	for _, candidate := range indexed {
		result = append(result, *candidate)
	}
	return result
}

func checklistItemAsReadinessItem[R any, H any](item ChecklistGroupItem[R, H]) ReadinessItem[R, H] {
	return ReadinessItem[R, H]{
		Key:             item.Key,
		Label:           item.Label,
		Message:         item.Message,
		FieldPaths:      item.FieldPaths,
		SuggestedAction: item.SuggestedAction,
		Reason:          item.Reason,
		RepairHints:     item.RepairHints,
		Taxonomy:        item.Taxonomy,
	}
}

func cloneRepairArtifacts[R any, H any, P any, S any, Q any, V any](
	hint H,
	accessors RepairHintAccessors[H, P, S, Q, V],
	options RepairCenterFromReadinessOptions[R, P, S, Q, V],
) (*P, *S, *Q, *V) {
	patch := accessors.Patch(hint)
	skeleton := accessors.Skeleton(hint)
	revision := accessors.Revision(hint)
	validation := accessors.Validation(hint)
	if options.CloneArtifacts == nil {
		return patch, skeleton, revision, validation
	}
	return options.CloneArtifacts(patch, skeleton, revision, validation)
}

func cloneRepairReason[R any, P any, S any, Q any, V any](
	reason *R,
	options RepairCenterFromReadinessOptions[R, P, S, Q, V],
) *R {
	if reason == nil || options.CloneReason == nil {
		return reason
	}
	return options.CloneReason(reason)
}

func compareRepairActions[R any, P any, S any, Q any, V any](
	a, b RepairCenterAction[R, P, S, Q, V],
) bool {
	if repairPriorityRank(a.Priority) != repairPriorityRank(b.Priority) {
		return repairPriorityRank(a.Priority) < repairPriorityRank(b.Priority)
	}
	if repairStatusRank(a.Status) != repairStatusRank(b.Status) {
		return repairStatusRank(a.Status) < repairStatusRank(b.Status)
	}
	if a.CanApplyDirectly != b.CanApplyDirectly {
		return a.CanApplyDirectly
	}
	if repairSectionRank(a.EditorSection) != repairSectionRank(b.EditorSection) {
		return repairSectionRank(a.EditorSection) < repairSectionRank(b.EditorSection)
	}
	return a.ID < b.ID
}

func repairActionID(key, target, revisionPath string) string {
	return key + ":" + target + ":" + revisionPath
}

func repairPriorityRank(priority string) int {
	switch priority {
	case "high":
		return 0
	case "medium":
		return 1
	default:
		return 2
	}
}

func repairStatusRank(status string) int {
	switch status {
	case "blocking":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

func repairSectionRank(section string) int {
	switch normalizeRepairSection(section) {
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

func normalizeRepairSection(section string) string {
	switch section {
	case "category", "attributes", "sale_attributes", "basics":
		return section
	default:
		return "system"
	}
}

func repairSectionLabel(section string) string {
	switch normalizeRepairSection(section) {
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
