package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

func buildSheinReadinessReason(spec *sheinworkspace.ReadinessReasonSpec) *SheinReadinessReason {
	return sheinworkspace.BuildReadinessReason(spec)
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
		Patch:         artifacts.Patch,
		Skeleton:      artifacts.Skeleton,
		Revision:      artifacts.Request,
		Validation:    artifacts.Validation,
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
	return sheinworkspace.CloneReadinessReason(reason)
}

func cloneSheinRepairHints(items []SheinRepairHint) []SheinRepairHint {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]SheinRepairHint, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, SheinRepairHint{
			Action:        item.Action,
			Priority:      item.Priority,
			Target:        item.Target,
			EditorSection: item.EditorSection,
			EditorFocus:   append([]string(nil), item.EditorFocus...),
			RevisionPath:  item.RevisionPath,
			Description:   item.Description,
			FieldPaths:    append([]string(nil), item.FieldPaths...),
			Patch:         sheinworkspace.CloneRepairPatchPayload(item.Patch),
			Skeleton:      sheinworkspace.CloneEditorRevisionSkeleton(item.Skeleton),
			Revision:      cloneApplyRevisionRequest(item.Revision),
			Validation:    sheinworkspace.CloneRepairValidationPreview(item.Validation),
		})
	}
	return cloned
}
