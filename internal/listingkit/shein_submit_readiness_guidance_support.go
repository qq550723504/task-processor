package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

func buildSheinRepairArtifacts(pkg *SheinPackage, action string, editorSection string, patch *SheinRepairPatchPayload) sheinworkspace.RepairArtifacts[SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview] {
	seed := sheinworkspace.BuildRepairRevisionSeed(action, patch)
	var request *ApplyRevisionRequest
	if seed.Input != nil && seed.Skeleton != nil {
		request = &ApplyRevisionRequest{
			Platform: seed.Skeleton.Platform,
			Actor:    seed.Skeleton.Actor,
			Reason:   seed.Skeleton.Reason,
			Shein:    sheinworkspace.CloneRevisionInput(seed.Skeleton.Shein),
		}
	}
	var validation *SheinRepairValidationPreview
	if request != nil && seed.Skeleton != nil && seed.Skeleton.Shein != nil {
		valid := true
		var fieldErrors []RevisionFieldError
		if validationErr, ok := validateApplyRevisionRequest(request).(*RevisionValidationError); ok {
			valid = false
			fieldErrors = append([]RevisionFieldError(nil), validationErr.Fields...)
		}
		validation = sheinworkspace.BuildRepairValidationPreview(pkg, editorSection, seed.Skeleton, valid, fieldErrors)
	}
	return sheinworkspace.RepairArtifacts[SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview]{
		Patch:      sheinworkspace.CloneRepairPatchPayload(patch),
		Skeleton:   seed.Skeleton,
		Request:    request,
		Validation: validation,
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
