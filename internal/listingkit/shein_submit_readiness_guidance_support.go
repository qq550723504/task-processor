package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

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

func buildSheinReadinessGuidance(pkg *SheinPackage, key string, fieldPaths []string, suggestedAction string, warningOnly bool) sheinworkspace.Guidance[SheinReadinessReason, SheinRepairHint] {
	spec := sheinworkspace.BuildReadinessGuidanceSpec(key, warningOnly)
	if spec == nil || spec.Reason == nil {
		return sheinworkspace.Guidance[SheinReadinessReason, SheinRepairHint]{}
	}

	guidance := sheinworkspace.Guidance[SheinReadinessReason, SheinRepairHint]{
		Reason: sheinworkspace.BuildReadinessReason(spec.Reason),
	}
	patch := sheinworkspace.BuildReadinessPatchPayload(pkg, key)
	for _, hint := range spec.Hints {
		guidance.RepairHints = append(guidance.RepairHints, buildSheinReadinessRepairHint(
			pkg,
			suggestedAction,
			fieldPaths,
			hint,
			patch,
		))
	}
	return guidance
}
