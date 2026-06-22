package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

func buildSheinRepairCenter(readiness *SheinSubmitReadiness, checklist *SheinSubmitChecklist) *SheinRepairCenter {
	return sheinworkspace.BuildRepairCenterFromReadiness(
		readiness,
		checklist,
		sheinRepairHintAccessors(),
		sheinworkspace.RepairCenterFromReadinessOptions[SheinReadinessReason, SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview]{
			CloneReason:    cloneSheinReadinessReason,
			CloneArtifacts: cloneSheinRepairArtifactsForWorkspace,
			ValidationValid: func(validation *SheinRepairValidationPreview) bool {
				return validation != nil && validation.Valid
			},
			ChangeCount: func(validation *SheinRepairValidationPreview) int {
				if validation == nil || validation.RevisionDiffPreview == nil {
					return 0
				}
				return validation.RevisionDiffPreview.ChangeCount
			},
			IsInvalid: func(validation *SheinRepairValidationPreview) bool {
				return validation != nil && !validation.Valid
			},
			ReasonSummary: func(reason *SheinReadinessReason) string {
				if reason == nil {
					return ""
				}
				return reason.Summary
			},
			ActionInfo: func(action SheinRepairCenterAction) sheinworkspace.RepairSessionActionInfo {
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
		},
	)
}

func sheinRepairHintAccessors() sheinworkspace.RepairHintAccessors[SheinRepairHint, SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview] {
	return sheinworkspace.RepairHintAccessors[SheinRepairHint, SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview]{
		Priority: func(hint SheinRepairHint) string {
			return hint.Priority
		},
		Target: func(hint SheinRepairHint) string {
			return hint.Target
		},
		EditorSection: func(hint SheinRepairHint) string {
			return hint.EditorSection
		},
		EditorFocus: func(hint SheinRepairHint) []string {
			return hint.EditorFocus
		},
		RevisionPath: func(hint SheinRepairHint) string {
			return hint.RevisionPath
		},
		Description: func(hint SheinRepairHint) string {
			return hint.Description
		},
		Patch: func(hint SheinRepairHint) *SheinRepairPatchPayload {
			return hint.Patch
		},
		Skeleton: func(hint SheinRepairHint) *SheinEditorRevisionSkeleton {
			return hint.Skeleton
		},
		Revision: func(hint SheinRepairHint) *ApplyRevisionRequest {
			return hint.Revision
		},
		Validation: func(hint SheinRepairHint) *SheinRepairValidationPreview {
			return hint.Validation
		},
	}
}

func cloneSheinRepairArtifactsForWorkspace(
	patch *SheinRepairPatchPayload,
	skeleton *SheinEditorRevisionSkeleton,
	request *ApplyRevisionRequest,
	validation *SheinRepairValidationPreview,
) (*SheinRepairPatchPayload, *SheinEditorRevisionSkeleton, *ApplyRevisionRequest, *SheinRepairValidationPreview) {
	artifacts := cloneSheinRepairArtifacts(patch, skeleton, request, validation)
	return artifacts.patch, artifacts.skeleton, artifacts.request, artifacts.validation
}
