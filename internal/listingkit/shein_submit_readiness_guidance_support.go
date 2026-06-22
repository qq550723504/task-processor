package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

func buildSheinReadinessReason(spec *sheinworkspace.ReadinessReasonSpec) *SheinReadinessReason {
	if spec == nil {
		return nil
	}
	return &SheinReadinessReason{
		Code:     spec.Code,
		Category: spec.Category,
		Summary:  spec.Summary,
	}
}

func buildSheinReadinessRepairHint(pkg *SheinPackage, action string, fieldPaths []string, hint sheinworkspace.ReadinessHintSpec, patch *SheinRepairPatchPayload) SheinRepairHint {
	artifacts := buildSheinRepairArtifacts(pkg, action, hint.EditorSection, patch)
	return SheinRepairHint{
		Action:        action,
		Priority:      hint.Priority,
		Target:        hint.Target,
		EditorSection: hint.EditorSection,
		EditorFocus:   append([]string(nil), hint.EditorFocus...),
		RevisionPath:  hint.RevisionPath,
		Description:   hint.Description,
		FieldPaths:    append([]string(nil), fieldPaths...),
		Patch:         artifacts.patch,
		Skeleton:      artifacts.skeleton,
		Revision:      artifacts.request,
		Validation:    artifacts.validation,
	}
}

func buildSheinReadinessGuidance(pkg *SheinPackage, key string, fieldPaths []string, suggestedAction string, warningOnly bool) sheinReadinessGuidance {
	spec := sheinworkspace.BuildReadinessGuidanceSpec(key, warningOnly)
	if spec == nil || spec.Reason == nil {
		return sheinReadinessGuidance{}
	}

	guidance := sheinReadinessGuidance{
		reason: buildSheinReadinessReason(spec.Reason),
	}
	patch := sheinworkspace.BuildReadinessPatchPayload(pkg, key)
	for _, hint := range spec.Hints {
		guidance.repairHints = append(guidance.repairHints, buildSheinReadinessRepairHint(
			pkg,
			suggestedAction,
			fieldPaths,
			hint,
			patch,
		))
	}
	return guidance
}

func cloneSheinReadinessReason(reason *SheinReadinessReason) *SheinReadinessReason {
	if reason == nil {
		return nil
	}
	cloned := *reason
	return &cloned
}

func cloneSheinRepairHints(items []SheinRepairHint) []SheinRepairHint {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]SheinRepairHint, 0, len(items))
	for _, item := range items {
		artifacts := cloneSheinRepairArtifacts(item.Patch, item.Skeleton, item.Revision, item.Validation)
		cloned = append(cloned, SheinRepairHint{
			Action:        item.Action,
			Priority:      item.Priority,
			Target:        item.Target,
			EditorSection: item.EditorSection,
			EditorFocus:   append([]string(nil), item.EditorFocus...),
			RevisionPath:  item.RevisionPath,
			Description:   item.Description,
			FieldPaths:    append([]string(nil), item.FieldPaths...),
			Patch:         artifacts.patch,
			Skeleton:      artifacts.skeleton,
			Revision:      artifacts.request,
			Validation:    artifacts.validation,
		})
	}
	return cloned
}
